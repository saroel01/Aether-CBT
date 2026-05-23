<script lang="ts">
  import { toast } from '$lib/stores/toast';
  import { flip } from 'svelte/animate';
  import { fade, fly } from 'svelte/transition';

  let typeClasses = {
    success: 'bg-emerald-500 text-white shadow-emerald-200/50',
    warning: 'bg-amber-500 text-white shadow-amber-200/50',
    error: 'bg-red-500 text-white shadow-red-200/50',
    info: 'bg-indigo-500 text-white shadow-indigo-200/50'
  };

  let iconMap = {
    success: 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
    warning: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
    error: 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z',
    info: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
  };
</script>

<div class="fixed top-6 right-6 z-50 flex flex-col gap-3 w-full max-w-sm pointer-events-none">
  {#each $toast as t (t.id)}
    <div
      animate:flip={{ duration: 200 }}
      in:fly={{ x: 100, duration: 200 }}
      out:fade={{ duration: 150 }}
      class="pointer-events-auto flex items-center justify-between p-4 rounded-2xl shadow-lg border border-white/10 {typeClasses[t.type]} transition-all duration-300"
    >
      <div class="flex items-center gap-3">
        <svg class="h-6 w-6 stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d={iconMap[t.type]} />
        </svg>
        <span class="text-sm font-medium pr-4">{t.message}</span>
      </div>

      <button
        type="button"
        class="text-white/80 hover:text-white transition p-1 hover:bg-white/10 rounded-lg"
        on:click={() => toast.remove(t.id)}
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  {/each}
</div>
