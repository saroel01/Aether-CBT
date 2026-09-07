<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, apiUrl } from '$lib/api';
  import Button from '$lib/components/ui/Button.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
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

  // --- Network Resilience & Offline Detection (Phase 2 R1) ---
  let isOnline = true;
  let isPinging = false;
  let wasOffline = false;
  let showReconnectedBanner = false;
  let reconnectedTimer: ReturnType<typeof setTimeout> | null = null;
  let pingHandle: ReturnType<typeof setInterval> | null = null;
  const PING_INTERVAL_ONLINE_MS = 10_000;
  const PING_INTERVAL_OFFLINE_MS = 3_000;

  // Submission retry state (Zero-Data-Loss Auto-Retry Engine)
  let submissionPending = false;
  let pendingPayload: string | null = null;
  let retryCount = 0;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let isSubmitting = false;
  const PENDING_SUBMISSION_KEY = 'aether_pending_submission';

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

    // Zero-Data-Loss payload interceptor: hook into the iframe's fetch, XHR, sendBeacon, and form submit
    try {
      const f = document.getElementById('exam-iframe') as HTMLIFrameElement | null;
      const cw = f?.contentWindow as any;
      if (cw) {
        // Hook fetch
        if (cw.fetch) {
          const origFetch = cw.fetch;
          cw.fetch = function (input: any, init: any) {
            const url = typeof input === 'string' ? input : (input?.url || '');
            const isWebhook = typeof url === 'string' && (url.includes('/ispring/webhook') || url.includes('/webhook'));
            const hasDr = init?.body && typeof init.body === 'string' && (init.body.includes('dr=') || init.body.includes('sp='));
            if (isWebhook || hasDr) {
              saveSubmissionPayload(init?.body);
            }
            const p = origFetch.apply(this, arguments);
            if (p && typeof p.catch === 'function') {
              p.catch(() => {
                if (isWebhook || hasDr) {
                  isOnline = false;
                  submissionPending = true;
                  retrySubmission();
                }
              });
            }
            return p;
          };
        }
        // Hook XMLHttpRequest
        if (cw.XMLHttpRequest && cw.XMLHttpRequest.prototype) {
          const origSend = cw.XMLHttpRequest.prototype.send;
          cw.XMLHttpRequest.prototype.send = function (body: any) {
            if (body && ((typeof body === 'string' && (body.includes('dr=') || body.includes('sp='))) || (typeof FormData !== 'undefined' && body instanceof FormData && (body.has('dr') || body.has('sp'))))) {
              saveSubmissionPayload(body);
            }
            return origSend.apply(this, arguments);
          };
        }
        // Hook navigator.sendBeacon
        if (cw.navigator && cw.navigator.sendBeacon) {
          const origBeacon = cw.navigator.sendBeacon.bind(cw.navigator);
          cw.navigator.sendBeacon = function (url: string, data: any) {
            if (data && ((typeof data === 'string' && (data.includes('dr=') || data.includes('sp='))) || (typeof FormData !== 'undefined' && data instanceof FormData && data.has('dr')))) {
              saveSubmissionPayload(data);
            }
            return origBeacon(url, data);
          };
        }
        // Hook HTMLFormElement.submit
        if (cw.HTMLFormElement && cw.HTMLFormElement.prototype) {
          const origFormSubmit = cw.HTMLFormElement.prototype.submit;
          cw.HTMLFormElement.prototype.submit = function () {
            try {
              const formData = new FormData(this);
              saveSubmissionPayload(formData);
            } catch (e) { /* ignore */ }
            return origFormSubmit.apply(this, arguments);
          };
        }
      }
    } catch (e) {
      console.warn('Same-origin iframe hook notice:', e);
    }

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

  // --- Zero-Data-Loss Submission Intercept & Auto-Retry Engine ---
  function saveSubmissionPayload(body: any) {
    let bodyStr = '';
    if (typeof body === 'string') {
      bodyStr = body;
    } else if (typeof FormData !== 'undefined' && body instanceof FormData) {
      const params = new URLSearchParams();
      body.forEach((val, key) => {
        params.append(key, String(val));
      });
      bodyStr = params.toString();
    } else if (body && typeof body === 'object') {
      if (typeof URLSearchParams !== 'undefined' && body instanceof URLSearchParams) {
        bodyStr = body.toString();
      } else {
        try {
          bodyStr = JSON.stringify(body);
        } catch {
          bodyStr = String(body);
        }
      }
    }
    if (!bodyStr) return;

    // Ensure attempt_token, sid, and session_id are populated if known
    try {
      const params = new URLSearchParams(bodyStr);
      if (attemptToken && !params.get('attempt_token')) {
        params.set('attempt_token', attemptToken);
      }
      if (pesertaNoId && !params.get('sid')) {
        params.set('sid', pesertaNoId);
      }
      if (sessionId && !params.get('session_id')) {
        params.set('session_id', sessionId);
      }
      bodyStr = params.toString();
    } catch { /* keep existing bodyStr */ }

    pendingPayload = bodyStr;
    try {
      localStorage.setItem(PENDING_SUBMISSION_KEY, JSON.stringify({
        body: bodyStr,
        peserta_id: pesertaId,
        peserta_no_id: pesertaNoId,
        session_id: sessionId,
        attempt_token: attemptToken,
        saved_at: Date.now()
      }));
    } catch (e) {
      console.warn('Failed to save pending submission to localStorage:', e);
    }
  }

  function checkPendingSubmission() {
    try {
      const raw = localStorage.getItem(PENDING_SUBMISSION_KEY);
      if (raw) {
        const parsed = JSON.parse(raw);
        if (parsed?.body && (!attemptToken || parsed?.attempt_token === attemptToken)) {
          pendingPayload = parsed.body;
          submissionPending = true;
          submitted = true;
          retrySubmission();
        }
      }
    } catch (e) { /* ignore */ }
  }

  async function retrySubmission() {
    if (isSubmitting) return;
    if (!pendingPayload) {
      if (submissionPending) {
        if (retryTimer) clearTimeout(retryTimer);
        retryTimer = setTimeout(retrySubmission, 1000);
      }
      return;
    }

    isSubmitting = true;
    retryCount++;

    try {
      const res = await fetch(apiUrl('/ispring/webhook'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: pendingPayload
      });

      if (res.ok) {
        localStorage.removeItem(PENDING_SUBMISSION_KEY);
        pendingPayload = null;
        submissionPending = false;
        submitted = true;
        showResultModal = true;
        toast.success('Hasil ujian berhasil disinkronkan ke server!');
        return;
      }
    } catch (e) {
      // Network failure; auto-retry will trigger again
    } finally {
      isSubmitting = false;
    }

    if (submissionPending) {
      if (retryTimer) clearTimeout(retryTimer);
      retryTimer = setTimeout(retrySubmission, 3500);
    }
  }

  // --- Network Resilience & Connectivity Probing ---
  async function pingBackend(): Promise<boolean> {
    if (typeof window === 'undefined') return true;
    isPinging = true;
    try {
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 3500);
      const res = await fetch(apiUrl('/health'), {
        method: 'GET',
        cache: 'no-store',
        signal: controller.signal
      });
      clearTimeout(timeout);

      const ok = res.ok && res.status === 200;
      handlePingResult(ok);
      return ok;
    } catch {
      handlePingResult(false);
      return false;
    } finally {
      isPinging = false;
    }
  }

  function handlePingResult(ok: boolean) {
    if (ok) {
      if (!isOnline || wasOffline) {
        isOnline = true;
        showReconnectedBanner = true;
        if (reconnectedTimer) clearTimeout(reconnectedTimer);
        reconnectedTimer = setTimeout(() => {
          showReconnectedBanner = false;
          wasOffline = false;
        }, 4000);
        toast.success('Koneksi jaringan pulih.');
        onNetworkRestored();
      }
      isOnline = true;
      adjustPingInterval(PING_INTERVAL_ONLINE_MS);
    } else {
      if (isOnline) {
        wasOffline = true;
        showReconnectedBanner = false;
        toast.warning('Jaringan terputus. Jawaban tersimpan aman di peramban.');
      }
      isOnline = false;
      adjustPingInterval(PING_INTERVAL_OFFLINE_MS);
    }
  }

  function adjustPingInterval(intervalMs: number) {
    if (pingHandle) clearInterval(pingHandle);
    pingHandle = setInterval(pingBackend, intervalMs);
  }

  function handleWindowOnline() {
    pingBackend();
  }

  function handleWindowOffline() {
    handlePingResult(false);
  }

  function onNetworkRestored() {
    resyncFromServer();
    flushProgress();
    if (submissionPending) {
      retrySubmission();
    }
  }

  function handleBeforeUnload(e: BeforeUnloadEvent) {
    if ((!submitted && !locked) || submissionPending) {
      e.preventDefault();
      e.returnValue = '';
      return '';
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

    // Network resilience initialization (Phase 2 R1)
    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      isOnline = false;
      wasOffline = true;
    }
    window.addEventListener('online', handleWindowOnline);
    window.addEventListener('offline', handleWindowOffline);
    window.addEventListener('beforeunload', handleBeforeUnload);

    // Check for uncompleted pending submission from previous session/crash
    checkPendingSubmission();

    // Start periodic connectivity ping to /api/health
    adjustPingInterval(isOnline ? PING_INTERVAL_ONLINE_MS : PING_INTERVAL_OFFLINE_MS);
    pingBackend();

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
    window.removeEventListener('online', handleWindowOnline);
    window.removeEventListener('offline', handleWindowOffline);
    window.removeEventListener('beforeunload', handleBeforeUnload);
    if (pingHandle) clearInterval(pingHandle);
    if (reconnectedTimer) clearTimeout(reconnectedTimer);
    if (retryTimer) clearTimeout(retryTimer);
    if (progressTimer) clearInterval(progressTimer);
    if (tickHandle) clearInterval(tickHandle);
    if (resyncHandle) clearInterval(resyncHandle);
    if (lockPollHandle) clearInterval(lockPollHandle);
    if (iframeLoadTimer) clearTimeout(iframeLoadTimer);
  });

  async function refreshRemainingTime() {
    if (!isOnline) return;
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
      // Result was sent through the shim to the webhook; surface success or enter pending retry
      if (!isOnline || submissionPending) {
        submissionPending = true;
        retrySubmission();
      } else {
        submitted = true;
        showResultModal = true;
      }
    }
  }

  async function flushProgress() {
    if (!progressDirty || submitted || locked || !isOnline) return;
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
    if (!isOnline) return;
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
    if (!isOnline) {
      submissionPending = true;
      retrySubmission();
    }
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
    if (!isOnline) {
      submissionPending = true;
      retrySubmission();
    } else {
      submitted = true;
      showResultModal = true;
    }
  }

  function finishExam() {
    localStorage.clear();
    window.location.href = '/student/login';
  }
</script>

<svelte:head>
  <title>Lembar Ujian: {examLabel} - Aether CBT</title>
</svelte:head>

<div class="min-h-dvh bg-slate-950 bg-grid-sovereign text-slate-100 flex flex-col select-none relative overflow-hidden">
  <!-- Top focus-mode bar -->
  <header class="border-b border-slate-800 bg-slate-950/80 backdrop-blur-md px-6 py-3.5 flex justify-between items-center z-20 sticky top-0">
    <div class="flex items-center gap-3">
      <div class="flex flex-col">
        <div class="flex items-center gap-2">
          <h1 class="text-base font-bold text-slate-100 font-display leading-tight">{examLabel}</h1>
          <Badge variant="cobalt" theme="dark">Sedang Berlangsung</Badge>
        </div>
        <div class="text-xs text-slate-400 font-medium mt-0.5">
          No. Peserta: <span class="font-mono tabular-nums text-slate-300">{pesertaNoId}</span>
        </div>
      </div>
    </div>

    <div class="flex items-center gap-4">
      <!-- Fixed Tabular Countdown Timer with min-w-[7.5rem] -->
      <div 
        class="flex items-center justify-center gap-2 px-3.5 py-1.5 rounded-xl min-w-[7.5rem] font-mono transition-colors duration-150 {remainingSeconds < 60 ? 'border border-ruby-800/60 bg-ruby-950/40' : remainingSeconds < 300 ? 'border border-amber-800/60 bg-amber-950/40' : 'border border-slate-800 bg-slate-900/60'}"
      >
        <svg class="h-4 w-4 shrink-0 {remainingSeconds < 60 ? 'text-ruby-400' : remainingSeconds < 300 ? 'text-amber-400' : 'text-cobalt-400'}" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span class="text-sm font-bold tabular-nums {remainingSeconds < 60 ? 'text-ruby-300' : remainingSeconds < 300 ? 'text-amber-300' : 'text-slate-100'}">
          {fmtTime(remainingSeconds)}
        </span>
      </div>

      <Button variant="danger" size="sm" class="font-semibold" on:click={confirmExit} disabled={submitted}>
        Hentikan Ujian
      </Button>
    </div>
  </header>

  <!-- Calming Network Connectivity Banner (Phase 2 R1) -->
  {#if !isOnline}
    <div class="bg-amber-950/90 border-b border-amber-800/60 px-6 py-2.5 flex items-center justify-between z-20 shadow-sm transition-all duration-200">
      <div class="flex items-center gap-3">
        <div class="h-8 w-8 rounded-lg bg-amber-900/60 border border-amber-700/60 flex items-center justify-center text-amber-400 shrink-0">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M18.364 5.636a9 9 0 010 12.728m0 0l-2.829-2.829m2.829 2.829L3 3m15.364 2.636A9 9 0 005.636 5.636m12.728 0l-2.829 2.829M9.88 9.88a3 3 0 104.24 4.24m-4.24-4.24L3 3" />
          </svg>
        </div>
        <div class="text-xs">
          <div class="font-bold text-amber-200 flex items-center gap-2">
            <span>Koneksi Jaringan Terputus Sementara</span>
            <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-900/70 text-amber-300 border border-amber-700/50">
              <span class="h-1.5 w-1.5 rounded-full bg-amber-400 animate-pulse"></span>
              Mencoba menghubungkan kembali...
            </span>
          </div>
          <p class="text-amber-300/80 mt-0.5 leading-relaxed">
            Tetap tenang dan lanjutkan pengerjaan. <strong>Jangan tutup atau muat ulang (refresh) halaman ini</strong>. Jawaban Anda tersimpan aman di peramban dan akan otomatis disinkronkan ke server.
          </p>
        </div>
      </div>
      <div class="flex items-center gap-2 shrink-0 ml-4">
        <Button variant="secondary" size="sm" class="text-xs py-1 px-2.5 border-amber-700/60 hover:bg-amber-900/40 text-amber-200" on:click={pingBackend} disabled={isPinging} loading={isPinging}>
          {isPinging ? 'Memeriksa...' : 'Periksa Sekarang'}
        </Button>
      </div>
    </div>
  {:else if showReconnectedBanner}
    <div class="bg-emerald-950/90 border-b border-emerald-800/60 px-6 py-2.5 flex items-center justify-between z-20 shadow-sm transition-all duration-200">
      <div class="flex items-center gap-3">
        <div class="h-8 w-8 rounded-lg bg-emerald-900/60 border border-emerald-700/60 flex items-center justify-center text-emerald-400 shrink-0">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
        </div>
        <div class="text-xs">
          <span class="font-bold text-emerald-200">Koneksi Jaringan Telah Pulih</span>
          <span class="text-emerald-300/80 ml-2">Tersambung kembali ke server ujian. Seluruh data dan progres pengerjaan Anda telah tersinkronisasi ke server.</span>
        </div>
      </div>
    </div>
  {/if}

  <!-- Real iSpring content (Task 14). Same-origin iframe: the content-session cookie (set by
       POST /student/start) is sent automatically, and the server injects the shim on the
       served index.html which redirects result POSTs to /api/ispring/webhook with the
       attempt_token/tenant_id/sid. No hardcoded URL/token here (Req 12.5).

       The iSpring desktop player renders at a fixed 984x676 canvas, so the iframe is wrapped
       in a scaler div: the iframe keeps its native size while a CSS transform: scale() fit to
       the container makes the whole player responsive (desktop fullscreen, small mobile). -->
  <div 
    class="flex-1 relative z-10 bg-slate-950 overflow-hidden transition-opacity duration-200 {contentW > 0 && contentH > 0 ? 'opacity-100' : 'opacity-0'}" 
    bind:clientWidth={contentW} 
    bind:clientHeight={contentH}
  >
    {#if iframeError}
      <div class="absolute inset-0 bg-slate-950/90 backdrop-blur-sm flex flex-col items-center justify-center text-center p-6 z-30">
        <div class="bg-slate-900 border border-ruby-800/60 rounded-2xl p-8 max-w-md shadow-xl">
          <div class="h-14 w-14 bg-ruby-950/40 text-ruby-400 rounded-xl flex items-center justify-center mx-auto mb-4 border border-ruby-800/50">
            <svg class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01M5.07 19h13.86c1.54 0 2.5-1.67 1.73-3L13.73 4c-.77-1.33-2.7-1.33-3.46 0L3.34 16c-.77 1.33.19 3 1.73 3z" />
            </svg>
          </div>
          <h3 class="text-lg font-bold text-slate-100 mb-2 font-display">Gagal memuat soal ujian</h3>
          <p class="text-sm text-slate-400 mb-5 leading-relaxed text-pretty">
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
  <Modal theme="dark" bind:show={showConfirmExit} title="Konfirmasi Penghentian Ujian" size="sm">
    <div class="text-slate-300 p-2">
      <div class="flex items-start gap-3 mb-3">
        <div class="p-2 rounded-xl border border-ruby-800/60 bg-ruby-950/50 text-ruby-400 shrink-0">
          <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        </div>
        <div>
          <h3 class="text-base font-bold text-slate-100 font-display">Akhiri ujian sekarang?</h3>
          <p class="text-xs text-slate-400 mt-1 leading-relaxed text-pretty">
            Bila Anda sudah mengirim jawaban melalui lembar iSpring, hasil Anda telah tercatat di server. Menekan tombol di bawah akan menutup sesi ujian secara permanen.
          </p>
        </div>
      </div>
    </div>
    <div slot="footer" class="flex gap-3 justify-end">
      <Button variant="secondary" size="sm" on:click={() => (showConfirmExit = false)}>Kembali Mengerjakan</Button>
      <Button variant="danger" size="sm" on:click={endExamEarly}>Ya, Hentikan Ujian</Button>
    </div>
  </Modal>

  <!-- Result acknowledgement modal -->
  <Modal theme="dark" bind:show={showResultModal} title="Sesi Ujian Selesai" size="sm">
    <div class="text-center py-6 text-slate-300 px-4">
      <div class="h-14 w-14 bg-emerald-950/40 text-emerald-400 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-emerald-800/50 shadow-sm">
        <svg class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
        </svg>
      </div>
      <h3 class="text-2xl font-bold text-slate-100 mb-2 font-display">Sesi Selesai</h3>
      <p class="text-sm text-slate-400 leading-relaxed mb-6 text-pretty">
        Terima kasih telah berpartisipasi. Hasil pengerjaan Anda telah dilaporkan dan tercatat dengan aman di server.
      </p>
      <div class="bg-slate-950/60 border border-slate-800 p-4 rounded-xl max-w-xs mx-auto mb-6 flex justify-between text-left text-sm">
        <span class="text-slate-400 font-semibold">Ujian:</span>
        <span class="font-bold text-slate-200">{examLabel}</span>
      </div>
      <Button variant="primary" size="md" class="w-full" on:click={finishExam}>Selesai & Keluar Sesi</Button>
    </div>
  </Modal>

  <!-- Anti-cheat infraction warning -->
  <Modal theme="dark" bind:show={showCheatModal} title="Peringatan Keamanan Ujian" size="sm">
    <div class="text-center py-4 text-slate-300 px-4">
      <div class="h-14 w-14 bg-ruby-950/40 text-ruby-400 rounded-2xl flex items-center justify-center mx-auto mb-4 border border-ruby-800/50 shadow-sm">
        <svg class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
      </div>
      <h3 class="text-xl font-bold text-ruby-400 mb-2 font-display">Peringatan Keamanan!</h3>
      <p class="text-sm text-slate-400 leading-relaxed mb-5 text-pretty">
        Sistem mencatat Anda meninggalkan halaman ujian. Aktivitas ini telah dilaporkan kepada proktor ruangan.
      </p>
      <div class="bg-ruby-950/40 border border-ruby-800/50 p-3 rounded-xl max-w-xs mx-auto mb-6 text-sm text-ruby-300 font-bold font-mono tabular-nums">
        Jumlah Pelanggaran: {tabSwitchCount}x
      </div>
      <Button variant="primary" size="md" class="w-full" on:click={() => (showCheatModal = false)}>Kembali Mengerjakan Ujian</Button>
    </div>
  </Modal>

  <!-- Authoritative lock overlay (server-locked via RecordInfraction, Requirement 10/Property 11) -->
  {#if locked}
    <div class="fixed inset-0 bg-slate-950/95 backdrop-blur-md flex items-center justify-center z-50 p-4 select-none">
      <div class="bg-slate-900 border border-ruby-800/60 p-8 rounded-2xl max-w-md w-full text-center shadow-xl space-y-6">
        <div class="h-16 w-16 bg-ruby-950/50 text-ruby-400 rounded-2xl flex items-center justify-center mx-auto border border-ruby-800/60 shadow-sm">
          <svg class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
        </div>
        <div class="space-y-2">
          <h3 class="text-xl font-extrabold text-ruby-400 font-display uppercase tracking-wide">Sesi Ujian Dikunci</h3>
          <p class="text-slate-300 text-sm leading-relaxed text-pretty">
            Sesi ujian Anda ditangguhkan oleh server karena sistem mendeteksi indikasi pelanggaran keamanan berulang. Akses konten soal dihentikan sementara.
          </p>
        </div>
        <div class="bg-ruby-950/30 border border-ruby-800/40 p-4 rounded-xl text-xs text-ruby-300 leading-relaxed font-semibold">
          Silakan tetap di tempat dan hubungi pengawas ruangan Anda untuk melakukan verifikasi dan pembukaan kunci sesi ujian.
        </div>
      </div>
    </div>
  {/if}

  <!-- Reassuring Submission Pending Overlay (Phase 2 R1) -->
  {#if submissionPending}
    <div class="fixed inset-0 bg-slate-950/95 backdrop-blur-md flex items-center justify-center z-50 p-4 select-none">
      <div class="bg-slate-900 border border-amber-700/60 p-8 rounded-2xl max-w-md w-full text-center shadow-2xl space-y-6">
        <div class="h-16 w-16 bg-amber-950/50 text-amber-400 rounded-2xl flex items-center justify-center mx-auto border border-amber-700/60 shadow-sm">
          <svg class="h-8 w-8 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
          </svg>
        </div>
        <div class="space-y-2">
          <h3 class="text-xl font-bold text-amber-200 font-display">Menyimpan & Mengirim Jawaban</h3>
          <p class="text-slate-300 text-sm leading-relaxed text-pretty">
            Waktu ujian telah berakhir dan seluruh jawaban Anda telah <strong>tersimpan aman di peramban ini</strong>.
          </p>
          <div class="text-xs text-amber-300/90 leading-relaxed bg-amber-950/40 p-3 rounded-xl border border-amber-800/40 text-left space-y-1">
            <div class="flex justify-between items-center font-mono">
              <span>Status Pengiriman:</span>
              <span class="font-bold text-amber-200">Menunggu Jaringan ({retryCount}x)</span>
            </div>
            <div>Jangan matikan komputer atau menutup peramban ini. Sistem akan mengirim jawaban secara otomatis saat jaringan server terhubung kembali.</div>
          </div>
        </div>
        <div class="flex flex-col gap-2">
          <Button variant="primary" size="md" class="w-full" on:click={retrySubmission} disabled={isSubmitting} loading={isSubmitting}>
            {isSubmitting ? 'Mengirim Ulang...' : 'Kirim Ulang Sekarang'}
          </Button>
        </div>
      </div>
    </div>
  {/if}
</div>

