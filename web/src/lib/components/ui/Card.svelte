<script lang="ts">
  export let elevated = false;
  export let padding: 'none' | 'sm' | 'md' | 'lg' = 'md';

  let paddingClasses = {
    none: 'p-0',
    sm: 'p-4',
    md: 'p-6',
    lg: 'p-8'
  };
</script>

<div class="bg-white rounded-2xl border border-slate-100 transition-all duration-300 ease-out {elevated ? 'shadow-lg shadow-slate-100/80 border-slate-200/50' : 'shadow-sm shadow-slate-100/50'} {paddingClasses[padding]} {$$props.class || ''}">
  {#if $$slots.header}
    <div class="border-b border-slate-100 pb-4 mb-4">
      <slot name="header" />
    </div>
  {/if}
  
  <slot />

  {#if $$slots.footer}
    <div class="border-t border-slate-100 pt-4 mt-4">
      <slot name="footer" />
    </div>
  {/if}
</div>
