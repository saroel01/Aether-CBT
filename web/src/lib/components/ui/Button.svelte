<script lang="ts">
  export let variant: 'primary' | 'secondary' | 'ghost' | 'danger' | 'warning' = 'primary';
  export let size: 'sm' | 'md' | 'lg' = 'md';
  export let disabled = false;
  export let loading = false;
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let theme: 'light' | 'dark' = 'dark';

  const classes = {
    dark: {
      primary: 'bg-cobalt-600 hover:bg-cobalt-700 active:bg-cobalt-800 text-white shadow-sm border border-cobalt-500/30 focus-visible:ring-cobalt-500 focus-visible:ring-offset-slate-950',
      secondary: 'bg-slate-800 hover:bg-slate-700 active:bg-slate-600 text-slate-200 border border-slate-700 shadow-sm focus-visible:ring-slate-500 focus-visible:ring-offset-slate-950',
      ghost: 'bg-transparent hover:bg-slate-800/60 active:bg-slate-800 text-slate-300 hover:text-slate-100 focus-visible:ring-slate-600 focus-visible:ring-offset-slate-950',
      danger: 'bg-ruby-600 hover:bg-ruby-700 active:bg-ruby-800 text-white shadow-sm border border-ruby-500/30 focus-visible:ring-ruby-500 focus-visible:ring-offset-slate-950',
      warning: 'bg-amber-500 hover:bg-amber-600 active:bg-amber-700 text-slate-950 font-semibold shadow-sm border border-amber-400 focus-visible:ring-amber-500 focus-visible:ring-offset-slate-950'
    },
    light: {
      primary: 'bg-cobalt-600 hover:bg-cobalt-700 active:bg-cobalt-800 text-white shadow-sm border border-cobalt-600/30 focus-visible:ring-cobalt-600 focus-visible:ring-offset-white',
      secondary: 'bg-slate-100 hover:bg-slate-200 active:bg-slate-300 text-slate-800 border border-slate-200 shadow-sm focus-visible:ring-slate-400 focus-visible:ring-offset-white',
      ghost: 'bg-transparent hover:bg-slate-100 active:bg-slate-200 text-slate-700 hover:text-slate-900 focus-visible:ring-slate-400 focus-visible:ring-offset-white',
      danger: 'bg-ruby-600 hover:bg-ruby-700 active:bg-ruby-800 text-white shadow-sm border border-ruby-600/30 focus-visible:ring-ruby-600 focus-visible:ring-offset-white',
      warning: 'bg-amber-500 hover:bg-amber-600 active:bg-amber-700 text-slate-950 font-semibold shadow-sm border border-amber-400 focus-visible:ring-amber-500 focus-visible:ring-offset-white'
    }
  };

  const sizeClasses = {
    sm: 'px-3 py-1.5 text-xs rounded-xl gap-1.5',
    md: 'px-4 py-2 text-sm rounded-xl gap-2',
    lg: 'px-5 py-2.5 text-base rounded-xl gap-2.5 font-semibold'
  };

  const spinnerSizes = {
    sm: 'h-3.5 w-3.5',
    md: 'h-4 w-4',
    lg: 'h-5 w-5'
  };

  $: activeTheme = classes[theme] || classes.dark;
  $: activeVariant = activeTheme[variant] || activeTheme.primary;
  $: activeSize = sizeClasses[size] || sizeClasses.md;
  $: activeSpinnerSize = spinnerSizes[size] || spinnerSizes.md;
</script>

<button
  {type}
  disabled={disabled || loading}
  aria-busy={loading ? true : undefined}
  aria-disabled={disabled || loading ? true : undefined}
  class="inline-flex items-center justify-center font-medium transition-colors duration-150 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:opacity-50 disabled:pointer-events-none select-none {activeVariant} {activeSize} {$$props.class || ''}"
  {...$$restProps}
  on:click
>
  {#if loading}
    <svg class="animate-spin shrink-0 {activeSpinnerSize} text-current" fill="none" viewBox="0 0 24 24" aria-hidden="true">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
  {/if}
  <slot />
</button>
