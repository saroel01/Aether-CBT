<script lang="ts">
  export let variant: 
    | 'cobalt' | 'info' 
    | 'emerald' | 'success' 
    | 'amber' | 'warning' 
    | 'ruby' | 'danger' 
    | 'slate' | 'neutral' = 'neutral';
  export let theme: 'light' | 'dark' = 'dark';
  export let title = '';
  export let dot = false;

  // Canonical mapping for backward compatibility and color tokens
  $: canonicalVariant = (
    variant === 'info' ? 'cobalt' :
    variant === 'success' ? 'emerald' :
    variant === 'warning' ? 'amber' :
    variant === 'danger' ? 'ruby' :
    variant === 'neutral' ? 'slate' :
    variant
  ) as 'cobalt' | 'emerald' | 'amber' | 'ruby' | 'slate';

  const classes = {
    dark: {
      cobalt: 'bg-cobalt-950/60 text-cobalt-300 border-cobalt-800/60',
      emerald: 'bg-emerald-950/60 text-emerald-300 border-emerald-800/60',
      amber: 'bg-amber-950/60 text-amber-300 border-amber-800/60',
      ruby: 'bg-ruby-950/60 text-ruby-300 border-ruby-800/60',
      slate: 'bg-slate-800/60 text-slate-200 border-slate-700/60'
    },
    light: {
      cobalt: 'bg-cobalt-50 text-cobalt-700 border-cobalt-200',
      emerald: 'bg-emerald-50 text-emerald-800 border-emerald-200',
      amber: 'bg-amber-50 text-amber-900 border-amber-200',
      ruby: 'bg-ruby-50 text-ruby-800 border-ruby-200',
      slate: 'bg-slate-100 text-slate-800 border-slate-200'
    }
  };

  const dotClasses = {
    cobalt: 'bg-cobalt-500',
    emerald: 'bg-emerald-500',
    amber: 'bg-amber-500',
    ruby: 'bg-ruby-500',
    slate: 'bg-slate-400'
  };

  $: activeTheme = classes[theme] || classes.dark;
  $: activeVariantClass = activeTheme[canonicalVariant] || activeTheme.slate;
  $: activeDotClass = dotClasses[canonicalVariant] || dotClasses.slate;
</script>

<span 
  {title}
  class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold border tabular-nums transition-colors duration-150 {activeVariantClass} {$$props.class || ''}"
  {...$$restProps}
>
  {#if dot}
    <span class="w-1.5 h-1.5 rounded-full mr-1.5 shrink-0 {activeDotClass}" aria-hidden="true"></span>
  {/if}
  <slot />
</span>
