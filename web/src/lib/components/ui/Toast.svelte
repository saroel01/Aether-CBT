<script lang="ts">
  import { toast } from '$lib/stores/toast';
  import { flip } from 'svelte/animate';
  import { fade, fly } from 'svelte/transition';

  let typeStyles = {
    success: {
      border: 'border-l-emerald-500',
      iconText: 'text-emerald-500',
      iconBg: 'bg-emerald-500/10'
    },
    warning: {
      border: 'border-l-amber-500',
      iconText: 'text-amber-500',
      iconBg: 'bg-amber-500/10'
    },
    error: {
      border: 'border-l-ruby-500',
      iconText: 'text-ruby-500',
      iconBg: 'bg-ruby-500/10'
    },
    info: {
      border: 'border-l-cobalt-500',
      iconText: 'text-cobalt-500',
      iconBg: 'bg-cobalt-500/10'
    }
  };

  let iconPaths = {
    success: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
    warning: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
    error: 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z',
    info: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
  };
</script>

<div class="fixed top-4 left-4 right-4 sm:left-auto sm:right-6 sm:top-6 z-[100] flex flex-col gap-2.5 w-auto sm:w-full max-w-sm pointer-events-none" aria-live="assertive">
  {#each $toast as t (t.id)}
    {@const style = typeStyles[t.type] || typeStyles.info}
    <div
      animate:flip={{ duration: 150 }}
      in:fly={{ x: 80, duration: 150 }}
      out:fade={{ duration: 100 }}
      class="pointer-events-auto flex items-center justify-between p-3.5 rounded-xl shadow-xl border border-slate-200 dark:border-slate-800 border-l-4 {style.border} bg-white dark:bg-slate-900 text-slate-900 dark:text-slate-100 transition-colors duration-150"
      role="alert"
    >
      <div class="flex items-center gap-3 min-w-0 pr-2">
        <div class="p-1.5 rounded-lg shrink-0 {style.iconBg} {style.iconText}">
          <svg class="h-5 w-5 stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d={iconPaths[t.type] || iconPaths.info} />
          </svg>
        </div>
        <span class="text-sm font-medium leading-snug break-words">{t.message}</span>
      </div>

      <button
        type="button"
        aria-label="Tutup notifikasi"
        class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors p-1 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-500 shrink-0"
        on:click={() => toast.remove(t.id)}
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  {/each}
</div>
