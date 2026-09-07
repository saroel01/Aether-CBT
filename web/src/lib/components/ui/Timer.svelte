<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';

  export let durationSeconds = 0;
  export let active = true;
  export let theme: 'light' | 'dark' = 'dark';

  const dispatch = createEventDispatcher();
  let intervalId: any = null;
  let remaining = durationSeconds;
  let endTime = 0;
  let lastDuration = -1;

  function initTimer() {
    if (durationSeconds > 0) {
      remaining = durationSeconds;
      endTime = Date.now() + durationSeconds * 1000;
    }
  }

  $: {
    if (durationSeconds > 0 && durationSeconds !== lastDuration) {
      lastDuration = durationSeconds;
      initTimer();
    }
  }

  function formatTime(secs: number) {
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    const s = secs % 60;
    
    const pad = (n: number) => n.toString().padStart(2, '0');
    
    if (h > 0) {
      return `${pad(h)}:${pad(m)}:${pad(s)}`;
    }
    return `${pad(m)}:${pad(s)}`;
  }

  onMount(() => {
    if (lastDuration === -1 && durationSeconds > 0) {
      lastDuration = durationSeconds;
      initTimer();
    }
    if (active) {
      start();
    }
  });

  onDestroy(() => {
    stop();
  });

  $: {
    if (active) {
      start();
    } else {
      stop();
    }
  }

  function start() {
    if (intervalId) return;
    if (endTime === 0 && remaining > 0) {
      endTime = Date.now() + remaining * 1000;
    }

    intervalId = setInterval(() => {
      if (endTime > 0) {
        const now = Date.now();
        const diffSecs = Math.max(0, Math.ceil((endTime - now) / 1000));
        remaining = diffSecs;
        dispatch('tick', remaining);

        if (remaining === 300) {
          dispatch('warning', '5 minutes left');
        } else if (remaining === 60) {
          dispatch('warning', '1 minute left');
        }

        if (remaining <= 0) {
          stop();
          dispatch('expired');
        }
      }
    }, 1000);
  }

  function stop() {
    if (intervalId) {
      clearInterval(intervalId);
      intervalId = null;
    }
  }

  // Visual cues: Normal (>5m), Amber Warning (<=5m and >1m), Ruby Critical (<=1m)
  $: isWarning = remaining <= 300 && remaining > 60;
  $: isCritical = remaining <= 60 && remaining > 0;
  $: isExpired = remaining <= 0;

  const classes = {
    dark: {
      normal: 'bg-slate-900 text-slate-200 border-slate-800',
      warning: 'bg-amber-950/50 text-amber-300 border-amber-800/60',
      critical: 'bg-ruby-950/50 text-ruby-300 border-ruby-800/60',
      expired: 'bg-ruby-950/60 text-ruby-300 border-ruby-800/70'
    },
    light: {
      normal: 'bg-slate-50 text-slate-800 border-slate-200',
      warning: 'bg-amber-50 text-amber-900 border-amber-200',
      critical: 'bg-ruby-50 text-ruby-800 border-ruby-200',
      expired: 'bg-ruby-50 text-ruby-800 border-ruby-200'
    }
  };

  $: activeTheme = classes[theme] || classes.dark;
  $: currentClass = isExpired ? activeTheme.expired :
                   isCritical ? activeTheme.critical :
                   isWarning ? activeTheme.warning :
                   activeTheme.normal;
</script>

<div 
  class="inline-flex items-center justify-center gap-2 px-3.5 py-1.5 rounded-xl border transition-colors duration-150 font-mono tabular-nums text-base font-semibold min-w-[7.5rem] shadow-sm select-none {currentClass} {$$props.class || ''}"
  role="timer"
  aria-live={isCritical || isExpired ? 'assertive' : 'off'}
  {...$$restProps}
>
  <svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
    <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
  <span class="tabular-nums">{formatTime(remaining)}</span>
</div>
