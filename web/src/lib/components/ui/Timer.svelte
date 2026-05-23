<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';

  export let durationSeconds = 0;
  export let active = true;

  const dispatch = createEventDispatcher();
  let intervalId: any;
  let remaining = durationSeconds;

  $: {
    if (durationSeconds !== remaining && durationSeconds > 0) {
      remaining = durationSeconds;
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
    intervalId = setInterval(() => {
      if (remaining > 0) {
        remaining -= 1;
        dispatch('tick', remaining);
        
        if (remaining === 600) { // 10 minutes left
          dispatch('warning', '10 minutes left');
        } else if (remaining === 180) { // 3 minutes left
          dispatch('warning', '3 minutes left');
        }
      } else {
        stop();
        dispatch('expired');
      }
    }, 1000);
  }

  function stop() {
    if (intervalId) {
      clearInterval(intervalId);
      intervalId = null;
    }
  }

  // Visual cues: Green (>10m), Amber (3-10m), Red (<3m)
  $: isLow = remaining < 600 && remaining >= 180;
  $: isCritical = remaining < 180;
</script>

<div class="inline-flex items-center gap-2 px-4 py-2 rounded-2xl border transition-all duration-300 font-mono text-lg font-semibold tracking-wider
  {isCritical ? 'bg-red-50 text-red-600 border-red-200 animate-pulse' : 
   isLow ? 'bg-amber-50 text-amber-600 border-amber-200' : 
   'bg-emerald-50 text-emerald-600 border-emerald-200'}"
>
  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
    <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
  </svg>
  <span>{formatTime(remaining)}</span>
</div>
