<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, apiUrl } from '$lib/api';
  import Button from '$lib/components/ui/Button.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import { toast } from '$lib/stores/toast';

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
  let tabSwitchCount = 0;
  let showCheatModal = false;
  let locked = false;
  let submitted = false;
  let showConfirmExit = false;
  let showResultModal = false;

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

    // Listen for progress/lock messages posted by the shim inside the iframe. The iSpring
    // player does not natively postMessage; the shim (injected server-side on index.html)
    // forwards a lightweight progress summary so we can debounce it to the server.
    window.addEventListener('message', onIframeMessage);

    // Anti-cheat tab/blur monitoring.
    if (typeof window !== 'undefined') {
      window.addEventListener('blur', recordTabSwitch);
      document.addEventListener('visibilitychange', handleVisibilityChange);
    }

    // Debounced progress flush interval.
    progressTimer = setInterval(flushProgress, PROGRESS_DEBOUNCE_MS);
  });

  onDestroy(() => {
    window.removeEventListener('message', onIframeMessage);
    window.removeEventListener('blur', recordTabSwitch);
    document.removeEventListener('visibilitychange', handleVisibilityChange);
    if (progressTimer) clearInterval(progressTimer);
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
    } catch { /* server is authoritative; timer keeps ticking locally */ }
  }

  function onIframeMessage(e: MessageEvent) {
    // Only accept same-origin messages (the content iframe is same-origin).
    if (!e.origin || e.origin !== window.location.origin) return;
    const d = e.data;
    if (!d || typeof d !== 'object') return;
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

  async function recordTabSwitch() {
    if (submitted || locked || showConfirmExit) return;
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
    if (document.hidden) recordTabSwitch();
  }

  // Local countdown; the server remains authoritative (Requirement 7.5) — this is a UX timer.
  let tickHandle: ReturnType<typeof setInterval> | null = null;
  $: if (remainingSeconds > 0 && !submitted && !locked) {
    if (tickHandle) clearInterval(tickHandle);
    tickHandle = setInterval(() => {
      remainingSeconds -= 1;
      if (remainingSeconds <= 0) {
        remainingSeconds = 0;
        handleTimeExpired();
      }
    }, 1000);
  }

  function handleTimeExpired() {
    toast.warning('Waktu ujian telah habis. Silakan tunggu hasil dari server.');
    submitted = true;
  }

  function confirmExit() {
    showConfirmExit = true;
  }

  function endExamEarly() {
    // The student ends early; the iframe result (if already submitted via shim) stands. We
    // simply navigate away. We do NOT synthesize a result — only the iSpring player may.
    showConfirmExit = false;
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
       attempt_token/tenant_id/sid. No hardcoded URL/token here (Req 12.5). -->
  <div class="flex-1 relative z-10">
    <iframe
      title="Lembar Ujian iSpring"
      src={apiUrl('/exam/content/index.html')}
      class="w-full h-full border-0"
      style="min-height: calc(100vh - 73px);"
      allow="fullscreen; autoplay; clipboard-write"
      sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals"
    ></iframe>
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
