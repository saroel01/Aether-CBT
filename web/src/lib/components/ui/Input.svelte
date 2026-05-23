<script lang="ts">
  export let label = '';
  export let value = '';
  export let placeholder = '';
  export let type = 'text';
  export let error = '';
  export let disabled = false;
  export let id = '';
</script>

<div class="flex flex-col gap-1.5 w-full {$$props.class || ''}">
  {#if label}
    <label for={id} class="text-xs font-semibold text-slate-500 uppercase tracking-wider">{label}</label>
  {/if}
  
  <div class="relative flex items-center">
    {#if $$slots.iconLeft}
      <div class="absolute left-4 text-slate-400">
        <slot name="iconLeft" />
      </div>
    {/if}

    {#if type === 'password'}
      <input
        {id}
        type="password"
        bind:value
        {placeholder}
        {disabled}
        class="w-full h-11 px-4 text-slate-800 bg-white border rounded-xl outline-none transition-all duration-150 ease-out focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:opacity-50 disabled:bg-slate-50
        {$$slots.iconLeft ? 'pl-11' : ''} 
        {$$slots.iconRight ? 'pr-11' : ''} 
        {error ? 'border-red-300 bg-red-50/10 focus:ring-red-500 focus:border-red-500' : 'border-slate-200/80 hover:border-slate-300'}"
        on:input
        on:keydown
        on:blur
      />
    {:else if type === 'number'}
      <input
        {id}
        type="number"
        bind:value
        {placeholder}
        {disabled}
        class="w-full h-11 px-4 text-slate-800 bg-white border rounded-xl outline-none transition-all duration-150 ease-out focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:opacity-50 disabled:bg-slate-50
        {$$slots.iconLeft ? 'pl-11' : ''} 
        {$$slots.iconRight ? 'pr-11' : ''} 
        {error ? 'border-red-300 bg-red-50/10 focus:ring-red-500 focus:border-red-500' : 'border-slate-200/80 hover:border-slate-300'}"
        on:input
        on:keydown
        on:blur
      />
    {:else}
      <input
        {id}
        type="text"
        bind:value
        {placeholder}
        {disabled}
        class="w-full h-11 px-4 text-slate-800 bg-white border rounded-xl outline-none transition-all duration-150 ease-out focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:opacity-50 disabled:bg-slate-50
        {$$slots.iconLeft ? 'pl-11' : ''} 
        {$$slots.iconRight ? 'pr-11' : ''} 
        {error ? 'border-red-300 bg-red-50/10 focus:ring-red-500 focus:border-red-500' : 'border-slate-200/80 hover:border-slate-300'}"
        on:input
        on:keydown
        on:blur
      />
    {/if}

    {#if $$slots.iconRight}
      <div class="absolute right-4 text-slate-400">
        <slot name="iconRight" />
      </div>
    {/if}
  </div>

  {#if error}
    <span class="text-xs text-red-500 font-medium">{error}</span>
  {/if}
</div>
