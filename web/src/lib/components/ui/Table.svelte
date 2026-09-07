<script lang="ts">
  export let theme: 'light' | 'dark' = 'light';
  export let density: 'compact' | 'normal' | 'comfortable' = 'normal';
  export let compact = false;
  export let stickyHeader = false;
  export let ariaLabel = 'Tabel Data';

  $: activeTheme = theme === 'dark' ? 'dark' : 'light';
  $: currentDensity = compact ? 'compact' : density;
</script>

<div 
  class="aether-table-container w-full overflow-x-auto rounded-xl border shadow-sm transition-colors focus:outline-none focus-visible:ring-1 focus-visible:ring-cobalt-500
  {activeTheme === 'dark' ? 'bg-slate-900/90 border-slate-800 text-slate-100' : 'bg-white border-slate-200 text-slate-800'} 
  {stickyHeader ? 'has-sticky-header' : ''}
  density-{currentDensity} 
  theme-{activeTheme} 
  {$$props.class || ''}"
  tabindex="0"
  role="region"
  aria-label={ariaLabel}
  {...$$restProps}
>
  <table class="w-full text-sm text-left border-collapse">
    <slot />
  </table>
</div>

<style>
  .aether-table-container.has-sticky-header :global(table th) {
    position: sticky;
    top: 0;
    z-index: 10;
  }

  .aether-table-container :global(table th) {
    font-weight: 600;
    text-transform: uppercase;
    font-size: 0.75rem;
    letter-spacing: 0.05em;
    transition: background-color 150ms ease;
  }

  .aether-table-container :global(table td) {
    font-size: 0.875rem;
    transition: background-color 150ms ease;
  }

  .aether-table-container :global(table tr:last-child td) {
    border-bottom: none;
  }

  /* Light Theme */
  .theme-light :global(table th) {
    background-color: #f8fafc;
    color: #334155;
    border-bottom: 1px solid #e2e8f0;
  }

  .theme-light :global(table td) {
    color: #0f172a;
    border-bottom: 1px solid #f1f5f9;
  }

  .theme-light :global(table tbody tr:hover td) {
    background-color: #f8fafc;
  }

  /* Dark Theme */
  .theme-dark :global(table th) {
    background-color: #0f172a;
    color: #cbd5e1;
    border-bottom: 1px solid #1e293b;
  }

  .theme-dark :global(table td) {
    color: #f1f5f9;
    border-bottom: 1px solid #1e293b;
  }

  .theme-dark :global(table tbody tr:hover td) {
    background-color: #1e293b;
  }

  /* Density: Compact */
  .density-compact :global(table th) {
    padding: 0.5rem 0.75rem;
  }

  .density-compact :global(table td) {
    padding: 0.5rem 0.75rem;
  }

  /* Density: Normal */
  .density-normal :global(table th) {
    padding: 0.75rem 1rem;
  }

  .density-normal :global(table td) {
    padding: 0.875rem 1rem;
  }

  /* Density: Comfortable */
  .density-comfortable :global(table th) {
    padding: 0.875rem 1.5rem;
  }

  .density-comfortable :global(table td) {
    padding: 1rem 1.5rem;
  }
</style>
