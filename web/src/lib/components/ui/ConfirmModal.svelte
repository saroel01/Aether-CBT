<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { fade, scale } from 'svelte/transition';
  import { browser } from '$app/environment';
  import Button from './Button.svelte';

  export let isOpen = false;
  export let show: boolean | undefined = undefined;
  export let title = 'Konfirmasi Tindakan';
  export let message = 'Apakah Anda yakin ingin melanjutkan tindakan ini?';
  export let confirmLabel = 'Konfirmasi';
  export let cancelLabel = 'Batal';
  export let variant: 'danger' | 'warning' | 'primary' = 'danger';
  export let loading = false;
  export let theme: 'light' | 'dark' = 'light';

  const dispatch = createEventDispatcher();

  $: visible = show !== undefined ? show : isOpen;
  $: activeTheme = theme === 'dark' ? 'dark' : 'light';

  $: if (browser) {
    if (visible) {
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

  function handleCancel() {
    if (loading) return;
    isOpen = false;
    if (show !== undefined) show = false;
    dispatch('cancel');
  }

  function handleConfirm() {
    dispatch('confirm');
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && visible && !loading) {
      event.preventDefault();
      handleCancel();
    }
  }

  const iconColors = {
    danger: 'text-ruby-600 dark:text-ruby-400 bg-ruby-50 dark:bg-ruby-950/50 border-ruby-200 dark:border-ruby-800/60',
    warning: 'text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/50 border-amber-200 dark:border-amber-800/60',
    primary: 'text-cobalt-600 dark:text-cobalt-400 bg-cobalt-50 dark:bg-cobalt-950/50 border-cobalt-200 dark:border-cobalt-800/60'
  };

  $: activeIconColor = iconColors[variant] || iconColors.danger;
</script>

<svelte:window on:keydown={handleKeydown} />

{#if visible}
  <div 
    class="fixed inset-0 z-50 flex items-center justify-center p-4 overflow-y-auto" 
    role="alertdialog" 
    aria-modal="true"
    aria-labelledby="confirm-modal-title"
    aria-describedby="confirm-modal-desc"
  >
    <!-- Clean backdrop -->
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div 
      class="fixed inset-0 bg-slate-950/70 transition-opacity" 
      on:click={handleCancel}
      transition:fade={{ duration: 150 }}
    ></div>

    <!-- Dialog container -->
    <div 
      class="relative w-full max-w-md rounded-2xl shadow-xl border overflow-hidden z-10 
      {activeTheme === 'dark' ? 'bg-slate-900 border-slate-800 text-slate-100' : 'bg-white border-slate-200 text-slate-900'}"
      transition:scale={{ start: 0.96, duration: 150 }}
    >
      <div class="p-6 flex flex-col gap-4">
        <div class="flex items-start gap-4">
          <div class="p-2.5 rounded-xl border shrink-0 {activeIconColor}">
            {#if variant === 'danger'}
              <svg class="h-6 w-6 stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            {:else if variant === 'warning'}
              <svg class="h-6 w-6 stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            {:else}
              <svg class="h-6 w-6 stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            {/if}
          </div>

          <div class="flex-1 min-w-0">
            <h3 id="confirm-modal-title" class="text-base font-bold font-display {theme === 'dark' ? 'text-slate-100' : 'text-slate-900'}">
              {title}
            </h3>
            <p id="confirm-modal-desc" class="text-sm mt-1.5 leading-relaxed {theme === 'dark' ? 'text-slate-300' : 'text-slate-600'}">
              {message}
            </p>
            <slot />
          </div>
        </div>

        <div class="flex items-center justify-end gap-3 pt-2">
          <Button 
            variant="secondary" 
            size="sm" 
            {theme} 
            disabled={loading} 
            on:click={handleCancel}
          >
            {cancelLabel}
          </Button>

          <Button 
            variant={variant} 
            size="sm" 
            {theme} 
            {loading} 
            on:click={handleConfirm}
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  </div>
{/if}
