<script lang="ts">
  export let label = '';
  export let value: string | number = '';
  export let placeholder = '';
  export let type = 'text';
  export let error = '';
  export let disabled = false;
  export let id = '';
  export let name = '';
  export let required = false;
  export let autocomplete = '';
  export let theme: 'light' | 'dark' = 'dark';

  const defaultId = `input-${Math.random().toString(36).slice(2, 9)}`;
  $: inputId = id || defaultId;

  const themeClasses = {
    dark: 'text-slate-100 bg-slate-950 border-slate-800 hover:border-slate-700 focus:border-cobalt-500 focus-visible:ring-cobalt-500 placeholder-slate-500',
    light: 'text-slate-900 bg-white border-slate-200 hover:border-slate-300 focus:border-cobalt-600 focus-visible:ring-cobalt-600 placeholder-slate-400'
  };

  const labelThemeClasses = {
    dark: 'text-slate-400',
    light: 'text-slate-700'
  };

  $: activeTheme = theme === 'light' ? 'light' : 'dark';
</script>

<div class="flex flex-col gap-1.5 w-full {$$props.class || ''}">
  {#if label}
    <label for={inputId} class="text-xs font-semibold uppercase tracking-wider {labelThemeClasses[activeTheme]}">
      {label}
      {#if required}
        <span class="text-ruby-500 ml-0.5" aria-hidden="true">*</span>
      {/if}
    </label>
  {/if}
  
  <div class="relative flex items-center">
    {#if $$slots.iconLeft}
      <div class="absolute left-3.5 text-slate-400 pointer-events-none z-10 flex items-center">
        <slot name="iconLeft" />
      </div>
    {/if}

    <input
      id={inputId}
      {name}
      {type}
      bind:value
      {placeholder}
      {disabled}
      {required}
      {autocomplete}
      aria-invalid={error ? true : undefined}
      aria-describedby={error ? `${inputId}-error` : undefined}
      class="w-full h-11 px-3.5 border rounded-xl outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed shadow-sm text-sm
      {$$slots.iconLeft ? 'pl-10' : ''} 
      {$$slots.iconRight ? 'pr-10' : ''} 
      {activeTheme === 'dark' ? 'focus-visible:ring-offset-slate-950' : 'focus-visible:ring-offset-white'}
      {error ? (activeTheme === 'dark' ? 'border-ruby-500 bg-ruby-950/20 text-slate-100 focus:border-ruby-500 focus-visible:ring-ruby-500' : 'border-ruby-500 bg-ruby-50/50 text-slate-900 focus:border-ruby-500 focus-visible:ring-ruby-500') : themeClasses[activeTheme]}"
      {...$$restProps}
      on:input
      on:keydown
      on:keyup
      on:blur
      on:focus
      on:change
    />

    {#if $$slots.iconRight}
      <div class="absolute right-3.5 text-slate-400 z-10 flex items-center">
        <slot name="iconRight" />
      </div>
    {/if}
  </div>

  {#if error}
    <span id="{inputId}-error" role="alert" class="text-xs font-medium mt-0.5 {activeTheme === 'dark' ? 'text-ruby-400' : 'text-ruby-600'}">{error}</span>
  {/if}
</div>
