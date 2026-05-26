<script lang="ts">
  export let variant: 'primary' | 'secondary' | 'ghost' | 'danger' = 'primary';
  export let size: 'sm' | 'md' | 'lg' = 'md';
  export let disabled = false;
  export let loading = false;
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let theme: 'light' | 'dark' = 'dark';

  let classes = {
    dark: {
      primary: 'bg-indigo-600 hover:bg-indigo-700 text-white shadow-md shadow-indigo-600/10 border border-indigo-600/30 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-950',
      secondary: 'bg-slate-900/50 hover:bg-slate-900/80 text-slate-300 border border-slate-800 shadow-sm focus:ring-slate-700 focus:ring-offset-slate-950',
      ghost: 'bg-transparent hover:bg-slate-900/50 text-slate-400 hover:text-slate-200 focus:ring-slate-800',
      danger: 'bg-red-950/40 hover:bg-red-900/40 text-red-400 border border-red-900/30 hover:border-red-900/60 shadow-sm focus:ring-red-900 focus:ring-offset-slate-950'
    },
    light: {
      primary: 'bg-indigo-600 hover:bg-indigo-700 text-white shadow-md shadow-indigo-600/10 border border-indigo-600/20 focus:ring-indigo-600 focus:ring-offset-2',
      secondary: 'bg-white hover:bg-slate-50 text-slate-700 border border-slate-200 shadow-sm shadow-slate-100/50 focus:ring-slate-400',
      ghost: 'bg-transparent hover:bg-slate-100 text-slate-600 hover:text-slate-900 focus:ring-slate-300',
      danger: 'bg-red-50 hover:bg-red-100 text-red-600 border border-red-200/60 focus:ring-red-500'
    }
  };

  let sizeClasses = {
    sm: 'px-3.5 py-2 text-xs rounded-xl gap-1.5',
    md: 'px-5 py-2.5 text-sm rounded-2xl gap-2',
    lg: 'px-6 py-3.5 text-base rounded-2xl gap-2.5 font-semibold'
  };
</script>

<button
  {type}
  disabled={disabled || loading}
  class="inline-flex items-center justify-center font-semibold transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] focus:outline-none focus:ring-2 active:scale-[0.97] hover:scale-[1.015] disabled:opacity-50 disabled:pointer-events-none disabled:scale-100 {classes[theme][variant]} {sizeClasses[size]} {$$props.class || ''}"
  on:click
>
  {#if loading}
    <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-current" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
  {/if}
  <slot />
</button>
