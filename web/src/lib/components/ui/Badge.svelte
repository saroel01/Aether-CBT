<script lang="ts">
  export let variant: 'success' | 'warning' | 'danger' | 'info' | 'neutral' = 'neutral';

  let classes = {
    success: 'bg-emerald-50 text-emerald-700 border-emerald-200/50',
    warning: 'bg-amber-50 text-amber-700 border-amber-200/50',
    danger: 'bg-red-50 text-red-700 border-red-200/50',
    info: 'bg-indigo-50 text-indigo-700 border-indigo-200/50',
    neutral: 'bg-slate-50 text-slate-700 border-slate-200/50'
  };
</script>

<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold border {classes[variant]} {$$props.class || ''}">
  <slot />
</span>
