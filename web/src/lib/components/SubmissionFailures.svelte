<script lang="ts">
  // Audit H1: dead-lettered exam submissions, so staff notice a lost result and act on it.
  import { onMount } from 'svelte';
  import { api } from '#lib/api.js';
  import Card from '#lib/components/ui/Card.svelte';

  type SubmissionFailure = {
    id: number;
    no_id: string;
    nama?: string;
    error_message: string;
    submitted_at?: string;
    created_at: string;
  };

  let failures: SubmissionFailure[] = [];
  let loadError = '';

  export async function refresh() {
    try {
      const res = await api('/supervisor/submission-failures');
      if (res.success) {
        failures = res.data || [];
        loadError = '';
      }
    } catch {
      loadError = 'Gagal memuat daftar pengiriman gagal.';
    }
  }

  onMount(refresh);
</script>

{#if failures.length > 0 || loadError}
  <Card class="border-red-200 bg-red-50/60" padding="sm">
    <section aria-labelledby="submission-failures-title">
      <h2 id="submission-failures-title" class="text-sm font-bold text-red-800">
        Pengiriman hasil gagal ({failures.length})
      </h2>
      <p class="text-xs text-red-700 mt-1">
        Hasil siswa berikut tidak tersimpan. Periksa penyebabnya dan reset sesi siswa bila perlu agar siswa dapat mengirim ulang.
      </p>
      {#if loadError}
        <p class="text-xs text-red-700 mt-2" role="alert">{loadError}</p>
      {:else}
        <ul class="mt-3 divide-y divide-red-100 text-sm">
          {#each failures as f (f.id)}
            <li class="py-2 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-1">
              <span class="font-semibold text-slate-800">
                {f.no_id}{#if f.nama}<span class="font-normal text-slate-600"> · {f.nama}</span>{/if}
              </span>
              <span class="text-xs text-red-700 break-all">{f.error_message}</span>
              <time class="text-xs text-slate-500 font-mono" datetime={f.submitted_at || f.created_at}>
                {f.submitted_at || f.created_at}
              </time>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  </Card>
{/if}
