<script lang="ts">
  export let elevated = false;
  export let padding: 'none' | 'sm' | 'md' | 'lg' = 'md';
  export let padded: boolean | undefined = undefined;
  export let theme: 'light' | 'dark' = 'light';

  const paddingClasses = {
    none: 'p-0',
    sm: 'p-4',
    md: 'p-6',
    lg: 'p-8'
  };

  $: effectivePadding = padded === false ? 'none' : (padded === true && padding === 'none' ? 'md' : padding);

  $: hasBg = ($$props.class || '').split(' ').some((c: string) => c.startsWith('bg-'));
  $: hasBorder = ($$props.class || '').split(' ').some((c: string) => c.startsWith('border-'));
  $: hasText = ($$props.class || '').split(' ').some((c: string) => c.startsWith('text-'));
  $: hasRounded = ($$props.class || '').split(' ').some((c: string) => c.startsWith('rounded-'));

  $: bgClass = hasBg ? '' : (theme === 'dark' ? 'bg-slate-900/90' : 'bg-white');
  $: borderClass = hasBorder ? '' : (theme === 'dark' ? 'border-slate-800' : 'border-slate-200');
  $: textClass = hasText ? '' : (theme === 'dark' ? 'text-slate-100' : 'text-slate-900');
  $: roundedClass = hasRounded ? '' : 'rounded-2xl';
</script>

<div
  class="border transition-colors duration-150 {roundedClass} 
  {elevated ? 'shadow-md' : 'shadow-sm'} 
  {paddingClasses[effectivePadding] || paddingClasses.md} 
  {bgClass} 
  {borderClass} 
  {textClass} 
  {$$props.class || ''}"
  {...$$restProps}
>
  {#if $$slots.header}
    <div class="border-b pb-4 mb-4 {effectivePadding === 'none' ? 'px-6 pt-5' : ''} {theme === 'dark' ? 'border-slate-800' : 'border-slate-200'}">
      <slot name="header" />
    </div>
  {/if}
  
  <slot />

  {#if $$slots.footer}
    <div class="border-t pt-4 mt-4 {effectivePadding === 'none' ? 'px-6 pb-5' : ''} {theme === 'dark' ? 'border-slate-800' : 'border-slate-200'}">
      <slot name="footer" />
    </div>
  {/if}
</div>
