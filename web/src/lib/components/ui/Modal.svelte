<script lang="ts">
  import { fade, scale } from 'svelte/transition';
  
  export let show = false;
  export let title = '';
  export let size: 'sm' | 'md' | 'lg' | 'xl' = 'md';

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
      class="fixed inset-0 bg-slate-900/40 backdrop-blur-[4px]" 
      on:click={close}
      transition:fade={{ duration: 150 }}
    ></div>

    <!-- Modal Content -->
    <div 
      class="relative w-full bg-white rounded-3xl shadow-xl border border-slate-100 flex flex-col my-8 overflow-hidden {sizeClasses[size]}"
      transition:scale={{ start: 0.95, duration: 150 }}
    >
      <!-- Header -->
      {#if title || $$slots.header}
        <div class="px-6 py-5 border-b border-slate-100 flex items-center justify-between">
          <slot name="header">
            <h3 class="text-lg font-semibold text-slate-800">{title}</h3>
          </slot>
          <button 
            type="button" 
            class="text-slate-400 hover:text-slate-600 transition p-1 hover:bg-slate-50 rounded-lg" 
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
        <div class="px-6 py-4 bg-slate-50/50 border-t border-slate-100 flex items-center justify-end gap-3">
          <slot name="footer" />
        </div>
      {/if}
    </div>
  </div>
{/if}
