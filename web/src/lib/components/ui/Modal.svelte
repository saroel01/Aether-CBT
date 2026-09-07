<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import { browser } from '$app/environment';
  
  export let show = false;
  export let title = '';
  export let size: 'sm' | 'md' | 'lg' | 'xl' = 'md';
  export let theme: 'light' | 'dark' = 'light';
  export let closeOnBackdrop = true;

  const dispatch = createEventDispatcher();

  const sizeClasses = {
    sm: 'max-w-md',
    md: 'max-w-lg',
    lg: 'max-w-2xl',
    xl: 'max-w-4xl'
  };

  $: activeTheme = theme === 'dark' ? 'dark' : 'light';

  $: if (browser) {
    if (show) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
  }

  onDestroy(() => {
    if (browser) {
      document.body.style.overflow = '';
    }
  });

  function close() {
    show = false;
    dispatch('close');
  }

  function handleBackdropClick() {
    if (closeOnBackdrop) {
      close();
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && show) {
      event.preventDefault();
      close();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if show}
  <div 
    class="fixed inset-0 z-50 flex items-center justify-center p-4 overflow-y-auto" 
    role="dialog" 
    aria-modal="true"
    aria-labelledby={title ? 'modal-title' : undefined}
  >
    <!-- Clean semi-transparent backdrop without slop blur -->
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div 
      class="fixed inset-0 bg-slate-950/70 transition-opacity" 
      on:click={handleBackdropClick}
      transition:fade={{ duration: 150 }}
    ></div>

    <!-- Modal Content -->
    <div 
      class="relative w-full max-h-[calc(100vh-3rem)] rounded-2xl shadow-xl border flex flex-col my-8 overflow-hidden z-10 {sizeClasses[size]}
      {activeTheme === 'dark' ? 'bg-slate-900 border-slate-800 text-slate-100' : 'bg-white border-slate-200 text-slate-900'}"
      transition:scale={{ start: 0.96, duration: 150 }}
    >
      <!-- Header -->
      {#if title || $$slots.header}
        <div class="px-6 py-4 border-b shrink-0 flex items-center justify-between {activeTheme === 'dark' ? 'border-slate-800 bg-slate-900' : 'border-slate-200 bg-white'}">
          <slot name="header">
            <h3 id="modal-title" class="text-base font-bold font-display {activeTheme === 'dark' ? 'text-slate-100' : 'text-slate-900'}">{title}</h3>
          </slot>
          <button 
            type="button" 
            aria-label="Tutup dialog"
            class="transition-colors duration-150 p-1.5 rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-500 {activeTheme === 'dark' ? 'text-slate-400 hover:text-slate-200 hover:bg-slate-800' : 'text-slate-400 hover:text-slate-600 hover:bg-slate-100'}" 
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
        <div class="px-6 py-4 border-t shrink-0 flex items-center justify-end gap-3 
          {activeTheme === 'dark' ? 'bg-slate-950/40 border-slate-800' : 'bg-slate-50 border-slate-200'}"
        >
          <slot name="footer" />
        </div>
      {/if}
    </div>
  </div>
{/if}
