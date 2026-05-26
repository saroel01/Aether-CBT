<script lang="ts">
  export let label = '';
  export let value = '';
  export let placeholder = '';
  export let type = 'text';
  export let error = '';
  export let disabled = false;
  export let id = '';
  export let theme: 'light' | 'dark' = 'dark';

  let themeClasses = {
    dark: 'text-slate-100 bg-slate-950/40 border-slate-800 hover:border-slate-700 focus:border-indigo-500 focus:ring-indigo-500/10 placeholder-slate-600',
    light: 'text-slate-900 bg-slate-50/50 border-slate-200/80 hover:border-slate-300 focus:border-indigo-600 focus:ring-indigo-600/10 placeholder-slate-400'
  };

  let labelThemeClasses = {
    dark: 'text-slate-400',
    light: 'text-slate-500'
  };
</script>

<div class="flex flex-col gap-1.5 w-full {$$props.class || ''}">
  {#if label}
    <label for={id} class="text-xs font-semibold uppercase tracking-widest {labelThemeClasses[theme]}">{label}</label>
  {/if}
  
  <div class="relative flex items-center">
    {#if $$slots.iconLeft}
      <div class="absolute left-4 text-slate-500 z-10 flex items-center">
        <slot name="iconLeft" />
      </div>
    {/if}

    <input
      {id}
      {type}
      bind:value
      {placeholder}
      {disabled}
      class="w-full h-12 px-4 border rounded-2xl outline-none transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] focus:ring-4 disabled:opacity-50 disabled:bg-slate-900/10
      {$$slots.iconLeft ? 'pl-11' : ''} 
      {$$slots.iconRight ? 'pr-11' : ''} 
      {error ? (theme === 'dark' ? 'border-red-950 bg-red-950/10 focus:ring-red-500/10 focus:border-red-500' : 'border-red-300 bg-red-50/20 focus:ring-red-500/10 focus:border-red-500') : themeClasses[theme]}"
      on:input
      on:keydown
      on:blur
    />

    {#if $$slots.iconRight}
      <div class="absolute right-4 text-slate-500 z-10 flex items-center">
        <slot name="iconRight" />
      </div>
    {/if}
  </div>

  {#if error}
    <span class="text-xs font-medium mt-0.5 {theme === 'dark' ? 'text-red-400' : 'text-red-500'}">{error}</span>
  {/if}
</div>
