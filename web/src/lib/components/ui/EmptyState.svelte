<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import Button from './Button.svelte';

  export let title = 'Tidak Ada Data';
  export let description = 'Belum ada data yang tersedia untuk ditampilkan.';
  export let icon: 'search' | 'users' | 'room' | 'file' | 'inbox' | 'alert' | 'none' = 'inbox';
  export let actionText = '';
  export let actionVariant: 'primary' | 'secondary' | 'ghost' | 'danger' | 'warning' = 'primary';
  export let actionHref = '';
  export let theme: 'light' | 'dark' = 'light';
  export let compact = false;

  const dispatch = createEventDispatcher<{ action: void }>();

  function handleActionClick() {
    dispatch('action');
  }

  $: isDark = theme === 'dark';
</script>

<div 
  class="flex flex-col items-center justify-center text-center select-none {compact ? 'py-6 px-4' : 'py-12 px-6 sm:px-12'} {$$props.class || ''}"
  role="status"
  aria-live="polite"
>
  {#if icon !== 'none'}
    <div 
      class="{compact ? 'w-10 h-10 rounded-xl mb-3' : 'w-14 h-14 rounded-2xl mb-4'} flex items-center justify-center border transition-colors
      {isDark ? 'bg-slate-900/90 border-slate-800 text-slate-400' : 'bg-slate-100/80 border-slate-200/80 text-slate-400 shadow-sm'}"
    >
      <slot name="icon">
        {#if icon === 'search'}
          <svg class="{compact ? 'h-5 w-5' : 'h-6 w-6'} stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        {:else if icon === 'users'}
          <svg class="{compact ? 'h-5 w-5' : 'h-6 w-6'} stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
          </svg>
        {:else if icon === 'room'}
          <svg class="{compact ? 'h-5 w-5' : 'h-6 w-6'} stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
          </svg>
        {:else if icon === 'file'}
          <svg class="{compact ? 'h-5 w-5' : 'h-6 w-6'} stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
        {:else if icon === 'alert'}
          <svg class="{compact ? 'h-5 w-5' : 'h-6 w-6'} stroke-current text-ruby-500" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        {:else}
          <!-- inbox default -->
          <svg class="{compact ? 'h-5 w-5' : 'h-6 w-6'} stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
          </svg>
        {/if}
      </slot>
    </div>
  {/if}

  <h4 class="{compact ? 'text-sm font-bold' : 'text-base font-bold'} font-display {isDark ? 'text-slate-200' : 'text-slate-800'}">
    {title}
  </h4>

  {#if description}
    <p class="{compact ? 'text-xs' : 'text-sm'} max-w-sm leading-relaxed mt-1 text-pretty {isDark ? 'text-slate-400' : 'text-slate-500'}">
      {description}
    </p>
  {/if}

  <slot />

  {#if actionText || $$slots.action}
    <div class="mt-4 flex items-center justify-center gap-3">
      <slot name="action">
        {#if actionHref}
          <a href={actionHref}>
            <Button variant={actionVariant} size="sm" {theme} class="shadow-sm font-semibold">
              {actionText}
            </Button>
          </a>
        {:else if actionText}
          <Button 
            variant={actionVariant} 
            size="sm" 
            {theme} 
            class="shadow-sm font-semibold"
            on:click={handleActionClick}
          >
            {actionText}
          </Button>
        {/if}
      </slot>
    </div>
  {/if}
</div>
