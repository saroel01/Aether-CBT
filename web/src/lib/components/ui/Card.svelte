<script lang="ts">
  export let elevated = false;
  export let padding: 'none' | 'sm' | 'md' | 'lg' = 'md';
  export let theme: 'light' | 'dark' = 'light';

  let paddingClasses = {
    none: 'p-0',
    sm: 'p-4',
    md: 'p-6',
    lg: 'p-8'
  };

  $: hasBg = ($$props.class || '').split(' ').some(c => c.startsWith('bg-'));
  $: hasBorder = ($$props.class || '').split(' ').some(c => c.startsWith('border-'));
  $: hasText = ($$props.class || '').split(' ').some(c => c.startsWith('text-'));

  $: bgClass = hasBg ? '' : (theme === 'dark' ? 'bg-[oklch(0.16_0.014_250)]' : 'bg-white');
  $: borderClass = hasBorder ? '' : (theme === 'dark' ? 'border-[oklch(0.22_0.016_250)]' : 'border-slate-100');
  $: textClass = hasText ? '' : (theme === 'dark' ? 'text-slate-100' : 'text-slate-800');
</script>

<div class="rounded-2xl border transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] 
  {elevated ? (theme === 'dark' ? 'shadow-[0_10px_30px_-5px_rgba(0,0,0,0.3)] border-[oklch(0.24_0.016_250)]' : 'shadow-[0_10px_30px_-5px_rgba(0,0,0,0.05)] border-slate-200/60') : (theme === 'dark' ? 'shadow-[0_4px_20px_-4px_rgba(0,0,0,0.2)]' : 'shadow-[0_4px_20px_-4px_rgba(0,0,0,0.02)]')} 
  {paddingClasses[padding]} 
  {bgClass} 
  {borderClass} 
  {textClass} 
  {$$props.class || ''}">
  {#if $$slots.header}
    <div class="border-b pb-4 mb-4 {theme === 'dark' ? 'border-[oklch(0.22_0.016_250)]' : 'border-slate-100'}">
      <slot name="header" />
    </div>
  {/if}
  
  <slot />

  {#if $$slots.footer}
    <div class="border-t pt-4 mt-4 {theme === 'dark' ? 'border-[oklch(0.22_0.016_250)]' : 'border-slate-100'}">
      <slot name="footer" />
    </div>
  {/if}
</div>

