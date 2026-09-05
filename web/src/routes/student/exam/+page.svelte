<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, apiUrl } from '$lib/api';
  import Button from '$lib/components/ui/Button.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import { toast } from '$lib/stores/toast';
  import { makeCountdown, deadlineFromServerRemaining, formatHMS } from '$lib/timer';

  // Task 14: replace the hardcoded question simulator with the real iSpring package served
  // same-origin via the content-session cookie. The iSpring player (inside the iframe) sends
  // its result POST through the injected shim to /api/ispring/webhook with attempt_token;
  // this page no longer synthesizes fake questions or result XML.

  let pesertaId = '';
  let pesertaNoId = '';
  let sessionId = '';
  let attemptToken = '';
  let examLabel = 'Ujian';

  // Server-issued exam duration / remaining seconds (Requirement 7.5).
  let remainingSeconds = 0;
  let duration = 0;

  // Anti-cheat + lock state (Requirement 10). The authoritative lock is server-side; we mirror
  // it client-side only to show an immediate overlay.
  //
  // Detection uses `document.visibilitychange` (not `window.blur`): `blur` fires on any focus
  // loss, including legitimate interactions with the iSpring iframe (clicking buttons,
  // opening dialogs, requesting fullscreen), which caused false-positive infractions. Only a
  // real tab/window switch flips `document.visibilityState` to 'hidden'. A debounce coalesces
  // the blur+visibilitychange double-fire of a single switch (review, Task 46), and a long-away
  // guard (>30s) treats a prolonged departure as serious regardless of debounce.
  let tabSwitchCount = 0;
  let showCheatModal = false;
  let locked = false;
  let submitted = false;
  let showConfirmExit = false;
  let showResultModal = false;
  let lastInfractionAt = 0;
  let lastHiddenAt: number | null = null;
  const INFRACTION_DEBOUNCE_MS = 1500;
  const SERIOUS_AWAY_MS = 30_000;

  // Time-warning thresholds (P3). The server is authoritative for remaining time (wall-clock,
  // resynced every 60s); these are pure UI signals so the student is alerted at the 10/5/1 minute
  // marks. Each threshold fires exactly once per session via a "warned" flag (same pattern as the
  // infraction debounce) so resyncs / countdown ticks never double-fire a warning.
  let warned10 = false;
  let warned5 = false;
  let warned1 = false;

  // P2: force-submit idempotency guard. Set the first time we tell the shim to force-submit
  // (either from a server resync returning force_submit, or from the local countdown expiring),
  // so we never drive player.submitQuiz() twice (which would double-submit or error after a
  // manual submit that already captured the result).
  let forceSubmitSent = false;

  // iSpring QuizMaker desktop player renders at a fixed canvas size (the "984 676" marker in
  // the package's index.html). The player itself is not responsive, so to make it fill the
  // available area (desktop fullscreen, and especially small/rotated mobile screens) we render
  // the iframe at the native size inside a scaler div and apply a CSS transform: scale() so the
  // whole player fits the container while preserving its aspect ratio. The container size is
  // bound reactively, so a browser resize or device rotation recomputes the scale immediately.
  const ISPRING_NATIVE_W = 984;
  const ISPRING_NATIVE_H = 676;
  let contentW = 0;
  let contentH = 0;
  // Reactive scale + centering (Svelte `$:` statements). Computed from the bound container
  // size so a browser resize or device rotation immediately refits the iSpring player. Because
  // we preserve aspect ratio (scale = min), one axis fills the container and the other leaves a
  // remainder; we center the scaled canvas by translating it half that remainder on each side,
  // so the leftover space splits evenly instead of bunching on the right/bottom (letterbox).
  let ispringScale = 1;
  let ispringOffsetX = 0;
  let ispringOffsetY = 0;
  $: {
    const sx = contentW > 0 ? contentW / ISPRING_NATIVE_W : 1;
    const sy = contentH > 0 ? contentH / ISPRING_NATIVE_H : 1;
    const s = Math.min(sx, sy);
    ispringScale = s;
    ispringOffsetX = (contentW - ISPRING_NATIVE_W * s) / 2;
    ispringOffsetY = (contentH - ISPRING_NATIVE_H * s) / 2;
  }

  // Iframe load state (Task 21): detect a failed/blank content load and offer retry.
  // Same-origin content lets us sanity-check the loaded document; cross-origin falls back
  // to assuming success. A timeout guard catches network failures (which never fire 'error').
  let iframeError = false;
  let iframeLoaded = false;
  let iframeLoadTimer: ReturnType<typeof setTimeout> | null = null;
  const IFRAME_LOAD_TIMEOUT_MS = 8000;

  function reloadIframe() {
    iframeError = false;
    iframeLoaded = false;
    const f = document.getElementById('exam-iframe') as HTMLIFrameElement | null;
    if (f) {
      // Force a reload by reassigning src.
      const src = f.src;
      f.src = 'about:blank';
      requestAnimationFrame(() => { f.src = src; });
    }
    armLoadTimeout();
  }

  function armLoadTimeout() {
    if (iframeLoadTimer) clearTimeout(iframeLoadTimer);
    iframeLoadTimer = setTimeout(() => {
      if (!iframeLoaded) iframeError = true;
    }, IFRAME_LOAD_TIMEOUT_MS);
  }

  function onIframeLoad() {
    iframeLoaded = true;
    iframeLoadTimer && clearTimeout(iframeLoadTimer);
    // Same-origin: sanity-check the document isn't a near-empty error page.
    try {
      const f = document.getElementById('exam-iframe') as HTMLIFrameElement | null;
      const doc = f?.contentDocument;
      if (doc && doc.body && doc.body.innerHTML.replace(/\s/g, '').length < 100) {
        iframeError = true;
      } else {
        iframeError = false;
      }
    } catch {
      // cross-origin: assume loaded OK.
      iframeError = false;
    }
  }

  // Debounced progress ticker (Requirement 13.2): coalesce frequent iSpring progress events into
  // periodic light writes instead of writing on every click.
  let pendingAnswered = 0;
  let pendingTotal = 0;
  let progressDirty = false;
  let progressTimer: ReturnType<typeof setInterval> | null = null;
  const PROGRESS_DEBOUNCE_MS = 5000;

  function fmtTime(total: number): string {
    if (total <= 0) return '00:00';
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    const mm = String(m).padStart(2, '0');
    const ss = String(s).padStart(2, '0');
    return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
  }

  onMount(async () => {
    pesertaId = localStorage.getItem('peserta_id') || '';
    pesertaNoId = localStorage.getItem('peserta_no_id') || '';
    sessionId = localStorage.getItem('session_id') || '';
    attemptToken = localStorage.getItem('attempt_token') || '';
    examLabel = localStorage.getItem('selected_mapel_name') || 'Ujian';

    if (!pesertaId || !attemptToken) {
      toast.error('Silakan login dan mulai sesi ujian terlebih dahulu.');
      window.location.href = '/student/login';
      return;
    }

    // Fetch authoritative remaining time once; the local timer counts down from there.
    await refreshRemainingTime();
    anchorDeadlineFromServer();
    startTicker();
    // Resync the deadline every 60s to absorb clock drift and catch server-side expiry.
    resyncHandle = setInterval(resyncFromServer, RESYNC_INTERVAL_MS);

    // Listen for progress/lock messages posted by the shim inside the iframe. The iSpring
    // player does not natively postMessage; the shim (injected server-side on index.html)
    // forwards a lightweight progress summary so we can debounce it to the server.
    window.addEventListener('message', onIframeMessage);

    // Anti-cheat tab/blur monitoring.
    // `visibilitychange` only — see note above (blur false-positives on iframe interaction).
    if (typeof window !== 'undefined') {
      document.addEventListener('visibilitychange', handleVisibilityChange);
    }

    // Debounced progress flush interval.
    progressTimer = setInterval(flushProgress, PROGRESS_DEBOUNCE_MS);

    // Arm the iframe load timeout so a network failure surfaces an error overlay.
    armLoadTimeout();
  });

  let lockPollHandle: ReturnType<typeof setInterval> | null = null;
  $: {
    if (locked && !submitted) {
      if (!lockPollHandle) {
        lockPollHandle = setInterval(refreshRemainingTime, 3000);
      }
    } else {
      if (lockPollHandle) {
        clearInterval(lockPollHandle);
        lockPollHandle = null;
      }
    }
  }

  onDestroy(() => {
    window.removeEventListener('message', onIframeMessage);
    document.removeEventListener('visibilitychange', handleVisibilityChange);
    if (progressTimer) clearInterval(progressTimer);
    if (tickHandle) clearInterval(tickHandle);
    if (resyncHandle) clearInterval(resyncHandle);
    if (lockPollHandle) clearInterval(lockPollHandle);
    if (iframeLoadTimer) clearTimeout(iframeLoadTimer);
  });

  async function refreshRemainingTime() {
    try {
      const res = await api(
        `/student/remaining-time?peserta_id=${encodeURIComponent(pesertaId)}` +
        (sessionId ? `&session_id=${encodeURIComponent(sessionId)}` : '')
      );
      if (res?.data?.remaining_seconds !== undefined) {
        remainingSeconds = res.data.remaining_seconds;
        if (!duration) duration = remainingSeconds;
      }
      if (res?.data?.locked !== undefined) {
        locked = Boolean(res.data.locked);
      }
      // P2: server-authoritative force-submit signal. When the clock hits 0 the server sets
      // force_submit=true; the parent page forwards this to the iSpring shim (inside the iframe)
      // via postMessage so the shim can call player.submitQuiz() and capture answers even though
      // the student never clicked Submit. Sent once (forceSubmitSent guard) to avoid re-driving
      // the player API after a manual submit that already succeeded.
      if (res?.data?.force_submit && !submitted && !forceSubmitSent) {
        forceSubmitSent = true;
        sendForceSubmitToShim();
      }
    } catch { /* server is authoritative; timer keeps ticking locally */ }
  }

  // Ask the iSpring shim (inside the exam iframe) to force-submit. Same-origin (dev proxy /
  // single-port production), so postMessage reaches it; the shim guards with its own flag and
  // falls back to form.submit() if the player API is unavailable.
  function sendForceSubmitToShim() {
    try {
      const f = document.getElementById('exam-iframe') as HTMLIFrameElement | null;
      const target = f?.contentWindow;
      if (target) {
        target.postMessage({ type: 'aether_force_submit', aether: true }, '*');
      }
    } catch { /* best-effort */ }
  }

  function onIframeMessage(e: MessageEvent) {
    // Trust messages stamped with our 'aether' marker (the shim sets it). A same-origin origin
    // check is brittle here because the iSpring content can legitimately originate from a host
    // other than the page (e.g. served by the backend on a different port); the marker is the
    // authority and the payload is constrained to known types, so no untrusted data is acted on.
    const d = e.data;
    if (!d || typeof d !== 'object' || d.aether !== true) return;
    // The shim posts { type: 'aether_progress', answered, total } and { type: 'aether_result' }.
    if (d.type === 'aether_progress' && typeof d.answered === 'number' && typeof d.total === 'number') {
      pendingAnswered = d.answered;
      pendingTotal = d.total;
      progressDirty = true;
    } else if (d.type === 'aether_result') {
      // Result was sent through the shim to the webhook; surface success to the student.
      submitted = true;
      showResultModal = true;
    }
  }

  async function flushProgress() {
    if (!progressDirty || submitted || locked) return;
    progressDirty = false;
    try {
      await api('/student/progress', {
        method: 'POST',
        body: JSON.stringify({
          peserta_id: parseInt(pesertaId),
          session_id: sessionId ? parseInt(sessionId) : undefined,
          answered_count: pendingAnswered,
          total_questions: pendingTotal
        })
      });
    } catch { /* best-effort; server absorbs write load */ }
  }

  async function recordInfraction(force = false) {
    if (submitted || locked || showConfirmExit) return;
    const now = Date.now();
    // Debounce: a single tab switch can fire visibilitychange multiple times in quick
    // succession; coalesce them into one infraction unless `force` (a long absence).
    if (!force && now - lastInfractionAt < INFRACTION_DEBOUNCE_MS) return;
    lastInfractionAt = now;
    tabSwitchCount++;
    showCheatModal = true;
    toast.error(`⚠️ Dilarang meninggalkan halaman ujian! (${tabSwitchCount}x)`);
    try {
      const res = await api('/student/infraction', {
        method: 'POST',
        body: JSON.stringify({
          peserta_id: parseInt(pesertaId),
          session_id: sessionId ? parseInt(sessionId) : undefined
        })
      });
      // Server reports the authoritative lock (Requirement 10.2, Property 11).
      if (res?.data?.locked) {
        locked = true;
      }
    } catch { /* network errors don't reset the local counter */ }
  }

  function handleVisibilityChange() {
    if (document.hidden) {
      // Tab/window hidden: record the moment and flag an infraction (debounced).
      if (!submitted && !locked && !showConfirmExit) {
        lastHiddenAt = Date.now();
        recordInfraction(false);
      }
    } else {
      // Tab refocused: a long absence (>30s) is treated as serious — record immediately,
      // bypassing the debounce. Short refocuses only resync the timer.
      if (lastHiddenAt && !submitted && !locked && !showConfirmExit) {
        const awayMs = Date.now() - lastHiddenAt;
        if (awayMs > SERIOUS_AWAY_MS) {
          recordInfraction(true);
        }
      }
      lastHiddenAt = null;
      remainingSeconds = countdown ? countdown.remaining() : 0;
      resyncFromServer();
    }
  }

  // Wall-clock-anchored countdown (Task 20): remaining() recomputes from Date.now() vs an
  // absolute deadline, so background-tab throttling cannot make it undercount. Resync every
  // 60s and on tab refocus so the deadline tracks the server's authoritative clock.
  let countdown: ReturnType<typeof makeCountdown> | null = null;
  let tickHandle: ReturnType<typeof setInterval> | null = null;
  let resyncHandle: ReturnType<typeof setInterval> | null = null;
  const RESYNC_INTERVAL_MS = 60000;

  function anchorDeadlineFromServer() {
    countdown = makeCountdown(deadlineFromServerRemaining(remainingSeconds));
    remainingSeconds = countdown.remaining();
  }

  function startTicker() {
    if (tickHandle) clearInterval(tickHandle);
    tickHandle = setInterval(() => {
      remainingSeconds = countdown ? countdown.remaining() : 0;
      if (countdown && countdown.expired() && !submitted && !locked) {
        remainingSeconds = 0;
        handleTimeExpired();
      }
    }, 1000);
  }

  async function resyncFromServer() {
    await refreshRemainingTime();
    anchorDeadlineFromServer();
  }

  // P3: time-warning thresholds. Reactive on remainingSeconds (updated each tick). Each warning
  // fires once per session; the guard bands prevent a warning from re-triggering if the countdown
  // briefly crosses back over the boundary due to a server resync. Warnings are suppressed once
  // the exam is submitted/locked.
  $: if (!submitted && !locked && countdown) {
    if (!warned10 && remainingSeconds <= 600 && remainingSeconds > 300) {
      warned10 = true;
      toast.warning('⏳ Sisa waktu ujian 10 menit.');
    }
    if (!warned5 && remainingSeconds <= 300 && remainingSeconds > 60) {
      warned5 = true;
      toast.warning('⏳ Sisa waktu ujian tinggal 5 menit!');
    }
    if (!warned1 && remainingSeconds <= 60 && remainingSeconds > 0) {
      warned1 = true;
      toast.error('🚨 Waktu ujian hampir habis — segera kirim jawaban!');
    }
  }

  function handleTimeExpired() {
    toast.warning('Waktu ujian telah habis. Silakan tunggu hasil dari server.');
    // P2: tell the iSpring shim to force-submit now (capture answers). The local countdown
    // detects expiry within 1s, much faster than waiting for the 60s server resync to report
    // force_submit. forceSubmitSent guards against a double drive if both paths fire.
    if (!forceSubmitSent) {
      forceSubmitSent = true;
      sendForceSubmitToShim();
    }
    submitted = true;
  }

  function confirmExit() {
    showConfirmExit = true;
  }

  function endExamEarly() {
    // The student ends early; tell the iSpring shim to force-submit now to capture answers.
    showConfirmExit = false;
    if (!forceSubmitSent) {
      forceSubmitSent = true;
      sendForceSubmitToShim();
    }
    submitted = true;
    showResultModal = true;
  }

  function finishExam() {
    localStorage.clear();
    window.location.href = '/student/login';
  }
</script>

<svelte:head>
  <title>Lembar Ujian: {examLabel} - Aether CBT</title>
</svelte:head>

<div class="min-h-screen bg-slate-950 bg-grid-sovereign text-slate-100 flex flex-col select-none relative overflow-hidden">
  <!-- Top focus-mode bar -->
  <header class="border-b border-slate-900 bg-slate-950/80 backdrop-blur-md px-6 py-4 flex justify-between items-center z-20 sticky top-0">
    <div class="flex items-center gap-4">
      <div>
        <span class="text-[10px] uppercase tracking-widest text-indigo-500 font-bold font-mono">Ujian Sedang Berlangsung</span>
        <h1 class="text-lg font-bold text-slate-200 font-display">{examLabel}</h1>
      </div>
    </div>

    <div class="flex items-center gap-6">
      <!-- Local UX countdown; server is authoritative (Requirement 7.5) -->
      <div class="flex items-center gap-2 px-4 py-2 rounded-2xl border border-slate-800 bg-slate-900/50 font-mono">
        <svg class="h-4 w-4 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span class="text-sm font-bold tabular-nums {remainingSeconds < 300 ? 'text-red-400' : 'text-slate-200'}">
          {fmtTime(remainingSeconds)}
        </span>
      </div>
      <Button variant="danger" size="sm" class="font-semibold" on:click={confirmExit} disabled={submitted}>
        Hentikan Ujian
      </Button>
    </div>
  </header>

  <!-- Real iSpring content (Task 14). Same-origin iframe: the content-session cookie (set by
       POST /student/start) is sent automatically, and the server injects the shim on the
       served index.html which redirects result POSTs to /api/ispring/webhook with the
       attempt_token/tenant_id/sid. No hardcoded URL/token here (Req 12.5).

       The iSpring desktop player renders at a fixed 984x676 canvas, so the iframe is wrapped
       in a scaler div: the iframe keeps its native size while a CSS transform: scale() fit to
       the container makes the whole player responsive (desktop fullscreen, small mobile). -->
  <div class="flex-1 relative z-10 bg-slate-950 overflow-hidden" bind:clientWidth={contentW} bind:clientHeight={contentH}>
    {#if iframeError}
      <div class="absolute inset-0 bg-red-950/40 backdrop-blur-sm flex flex-col items-center justify-center text-center p-6 z-30">
        <div class="bg-slate-900 border border-red-900/40 rounded-3xl p-8 max-w-md shadow-2xl">
          <div class="h-16 w-16 bg-red-950/40 text-red-400 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-red-900/30">
            <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01M5.07 19h13.86c1.54 0 2.5-1.67 1.73-3L13.73 4c-.77-1.33-2.7-1.33-3.46 0L3.34 16c-.77 1.33.19 3 1.73 3z" />
            </svg>
          </div>
          <h3 class="text-lg font-bold text-slate-100 mb-2 font-display">Gagal memuat soal ujian</h3>
          <p class="text-sm text-slate-400 mb-5 leading-relaxed">
            Periksa koneksi internet, lalu coba lagi. Jika masih gagal, hubungi pengawas.
          </p>
          <div class="flex flex-col gap-2">
            <Button variant="primary" size="md" on:click={reloadIframe}>Coba Muat Ulang</Button>
            <Button variant="danger" size="sm" on:click={confirmExit}>Hentikan Ujian</Button>
          </div>
        </div>
      </div>
    {/if}
    <div
      class="absolute top-0 left-0"
      style="width:{ISPRING_NATIVE_W}px; height:{ISPRING_NATIVE_H}px; transform: translate({ispringOffsetX}px, {ispringOffsetY}px) scale({ispringScale}); transform-origin: top left;"
    >
      <iframe
        id="exam-iframe"
        title="Lembar Ujian iSpring"
        src={apiUrl('/exam/content/index.html')}
        allow="fullscreen; autoplay; clipboard-write"
        sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals"
        style="width:{ISPRING_NATIVE_W}px; height:{ISPRING_NATIVE_H}px; border:0; display:block;"
        on:load={onIframeLoad}
      ></iframe>
    </div>
  </div>

  <!-- Confirm exit modal -->
  <Modal theme="dark" bind:show={showConfirmExit} title="Akhiri Sesi Ujian" size="sm">
    <div class="text-slate-300 p-2">
      <p class="text-lg font-bold text-slate-100 mb-2 font-display">Akhiri ujian sekarang?</p>
      <p class="text-sm text-slate-400 leading-relaxed">
        Bila Anda sudah mengirim jawaban melalui pemain iSpring, hasil Anda telah tercatat di server. Menekan tombol di bawah hanya menutup halaman ini — pastikan Anda sudah menekan tombol kirim di dalam lembar soal.
      </p>
    </div>
    <div slot="footer" class="flex gap-3 justify-end">
      <Button variant="secondary" size="sm" on:click={() => (showConfirmExit = false)}>Batal</Button>
      <Button variant="primary" size="sm" on:click={endExamEarly}>Ya, Akhiri</Button>
    </div>
  </Modal>

  <!-- Result acknowledgement modal -->
  <Modal theme="dark" bind:show={showResultModal} title="Sesi Ujian Selesai" size="sm">
    <div class="text-center py-6 text-slate-300 px-4">
      <div class="h-16 w-16 bg-emerald-950/20 text-emerald-400 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-emerald-900/30 shadow-sm">
        <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
        </svg>
      </div>
      <h3 class="text-2xl font-bold text-slate-100 mb-2 font-display">Sesi Selesai</h3>
      <p class="text-sm text-slate-400 leading-relaxed mb-6">
        Terima kasih telah berpartisipasi. Hasil pengerjaan Anda telah dilaporkan dan tercatat dengan aman.
      </p>
      <div class="bg-slate-950/40 border border-[oklch(0.22_0.016_250)] p-4 rounded-2xl max-w-xs mx-auto mb-6 flex justify-between text-left text-sm">
        <span class="text-slate-400 font-semibold">Ujian:</span>
        <span class="font-bold text-slate-200">{examLabel}</span>
      </div>
      <Button variant="primary" size="md" class="w-full" on:click={finishExam}>Selesai & Keluar Sesi</Button>
    </div>
  </Modal>

  <!-- Anti-cheat infraction warning -->
  <Modal theme="dark" bind:show={showCheatModal} title="Peringatan Keamanan Ujian" size="sm">
    <div class="text-center py-4 text-slate-300 px-4">
      <div class="h-16 w-16 bg-red-950/20 text-red-400 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-red-900/30 animate-pulse">
        <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
      </div>
      <h3 class="text-xl font-bold text-red-500 mb-2 font-display">Peringatan Keamanan!</h3>
      <p class="text-sm text-slate-400 leading-relaxed mb-5">
        Sistem mencatat Anda meninggalkan halaman ujian. Aktivitas ini telah dilaporkan kepada proktor.
      </p>
      <div class="bg-red-950/30 border border-red-900/20 p-3 rounded-2xl max-w-xs mx-auto mb-6 text-sm text-red-400 font-bold">
        Jumlah Pelanggaran: {tabSwitchCount}x
      </div>
      <Button variant="primary" size="md" class="w-full" on:click={() => (showCheatModal = false)}>Kembali Mengerjakan Ujian</Button>
    </div>
  </Modal>

  <!-- Authoritative lock overlay (server-locked via RecordInfraction, Requirement 10/Property 11) -->
  {#if locked}
    <div class="fixed inset-0 bg-slate-950/95 backdrop-blur-md flex items-center justify-center z-50 p-4 select-none">
      <div class="bg-slate-900 border border-red-900/30 p-8 rounded-3xl max-w-md w-full text-center shadow-2xl space-y-6 relative overflow-hidden">
        <div class="absolute top-0 left-0 w-full h-[2px] bg-red-600"></div>
        <div class="h-16 w-16 bg-red-950/40 text-red-400 rounded-2xl flex items-center justify-center mx-auto border border-red-900/30 animate-pulse">
          <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
        </div>
        <div class="space-y-2">
          <h3 class="text-2xl font-extrabold text-red-500 font-display">UJIAN ANDA DIKUNCI!</h3>
          <p class="text-slate-400 text-sm leading-relaxed">
            Sesi ujian Anda ditangguhkan oleh server karena terdeteksi pelanggaran keamanan. Konten ujian kini ditolak (HTTP 403).
          </p>
        </div>
        <div class="bg-red-950/10 border border-red-900/20 p-4 rounded-2xl text-xs text-red-400 leading-normal font-semibold">
          Silakan hubungi pengawas ruangan Anda untuk memverifikasi dan membuka kembali sesi ujian.
        </div>
      </div>
    </div>
  {/if}
</div>
