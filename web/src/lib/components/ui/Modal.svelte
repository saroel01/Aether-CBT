<script lang="ts">
  import { fade, scale } from 'svelte/transition';
  
  export let show = false;
  export let title = '';
  export let size: 'sm' | 'md' | 'lg' | 'xl' = 'md';
  export let theme: 'light' | 'dark' = 'light';

  let sizeClasses = {
    sm: 'max-w-md',
    md: 'max-w-lg',
    lg: 'max-w-2xl',
    xl: 'max-w-4xl'
  };

  function close() {
    show = false;
  }
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 overflow-y-auto" role="dialog" aria-modal="true">
    <!-- Backdrop with blur -->
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div 
      class="fixed inset-0 bg-slate-950/60 backdrop-blur-[6px]" 
      on:click={close}
      transition:fade={{ duration: 150 }}
    ></div>

    <!-- Modal Content -->
    <div 
      class="relative w-full rounded-3xl shadow-2xl border flex flex-col my-8 overflow-hidden {sizeClasses[size]}
      {theme === 'dark' ? 'bg-[oklch(0.16_0.014_250)] border-[oklch(0.22_0.016_250)] text-slate-100' : 'bg-white border-slate-100 text-slate-800'}"
      transition:scale={{ start: 0.95, duration: 150 }}
    >
      <!-- Header -->
      {#if title || $$slots.header}
        <div class="px-6 py-5 border-b flex items-center justify-between {theme === 'dark' ? 'border-[oklch(0.22_0.016_250)]' : 'border-slate-100'}">
          <slot name="header">
            <h3 class="text-lg font-bold font-display {theme === 'dark' ? 'text-slate-100' : 'text-slate-900'}">{title}</h3>
          </slot>
          <button 
            type="button" 
            aria-label="Tutup dialog"
            class="transition p-1.5 rounded-xl {theme === 'dark' ? 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/50' : 'text-slate-400 hover:text-slate-600 hover:bg-slate-50'}" 
            on:click={close}
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      {/if}

      <!-- Body -->
      <div class="p-6 flex-1 overflow-y-auto">
        <slot />
      </div>

      <!-- Footer -->
      {#if $$slots.footer}
        <div class="px-6 py-4 border-t flex items-center justify-end gap-3 
          {theme === 'dark' ? 'bg-slate-950/40 border-[oklch(0.22_0.016_250)]' : 'bg-slate-50/50 border-slate-100'}"
        >
          <slot name="footer" />
        </div>
      {/if}
    </div>
  </div>
{/if}

