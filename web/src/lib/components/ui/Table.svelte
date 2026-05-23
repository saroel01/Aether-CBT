<div class="w-full overflow-x-auto rounded-2xl border border-slate-200/60 shadow-sm bg-white {$$props.class || ''}">
  <table class="w-full text-sm text-left border-collapse">
    <slot />
  </table>
</div>

<style>
  :global(table th) {
    background-color: #F8FAFC;
    color: #475569;
    font-weight: 600;
    text-transform: uppercase;
    font-size: 0.75rem;
    letter-spacing: 0.05em;
    padding: 0.875rem 1.5rem;
    border-bottom: 1px solid #E2E8F0;
  }

  :global(table td) {
    padding: 1rem 1.5rem;
    color: #0F172A;
    border-bottom: 1px solid #F1F5F9;
  }

  :global(table tr:last-child td) {
    border-bottom: none;
  }

  :global(table tr:hover td) {
    background-color: #F8FAFC;
  }
</style>
