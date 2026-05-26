<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import { toast } from '$lib/stores/toast';

  interface QuestionMetric {
    question_id: string;
    question_text: string;
    question_type: string;
    correct_count: number;
    total_count: number;
  }

  let metrics: QuestionMetric[] = [];
  let loading = true;
  let error = '';

  // Aggregated analytics
  let averagePassingRate = 0;
  let hardestQuestions: QuestionMetric[] = [];
  let easiestQuestions: QuestionMetric[] = [];

  onMount(async () => {
    await loadAnalytics();
  });

  async function loadAnalytics() {
    loading = true;
    try {
      const res = await api('/admin/results/analysis');
      if (res.success && res.data) {
        metrics = res.data || [];
        calculateDerivedMetrics();
      } else {
        throw new Error(res.error || 'Terjadi kesalahan sistem');
      }
    } catch (e: any) {
      error = e.message;
      toast.error('Gagal memuat analisis hasil: ' + e.message);
    }
    loading = false;
  }

  function calculateDerivedMetrics() {
    if (metrics.length === 0) return;

    let totalCorrect = 0;
    let totalAttempts = 0;

    // Map metrics to calculate individual success rates
    const itemsWithRates = metrics.map(q => {
      const rate = q.total_count > 0 ? (q.correct_count / q.total_count) * 100 : 0;
      totalCorrect += q.correct_count;
      totalAttempts += q.total_count;
      return { ...q, successRate: rate };
    });

    averagePassingRate = totalAttempts > 0 ? Math.round((totalCorrect / totalAttempts) * 100) : 0;

    // Sort by success rate to find hardest and easiest
    itemsWithRates.sort((a, b) => a.successRate - b.successRate);
    hardestQuestions = itemsWithRates.filter(q => q.successRate < 50).slice(0, 3);
    
    itemsWithRates.sort((a, b) => b.successRate - a.successRate);
    easiestQuestions = itemsWithRates.filter(q => q.successRate >= 80).slice(0, 3);
  }
</script>

<svelte:head>
  <title>Analisis Kualitatif Soal - Admin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-8 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="border-b pb-6 flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
    <div>
      <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Detail Analisis Butir Soal</h1>
      <p class="text-slate-500 text-sm">Agregasi kualitatif dan tingkat kesulitan butir soal ujian dari berkas hasil iSpring XML.</p>
    </div>
    
    <Button 
      variant="secondary" 
      size="sm"
      theme="light"
      class="font-semibold shadow-sm flex items-center gap-2"
      on:click={loadAnalytics}
    >
      <svg class="h-4 w-4 text-slate-505" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 1121.21 8H18.2" />
      </svg>
      Segarkan Analisis
    </Button>
  </div>

  {#if loading}
    <div class="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
      <svg class="animate-spin h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
      <p class="text-sm font-semibold">Menganalisis butir jawaban siswa...</p>
    </div>
  {:else if error}
    <div class="p-6 bg-red-50 border border-red-100 rounded-3xl text-center text-red-600 font-medium">
      Gagal memuat analisis butir soal: {error}
    </div>
  {:else if metrics.length === 0}
    <div class="py-16 text-center bg-white border border-slate-100 rounded-3xl text-slate-400 font-medium shadow-sm">
      Belum ada data lembar hasil ujian yang diserahkan oleh peserta.<br>Analisis soal otomatis akan terbuat jika siswa menyelesaikan ujian.
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <Card padding="md" class="border-indigo-100/70 bg-indigo-50/20 hover:bg-indigo-50/30 transition-all duration-300 flex items-center justify-between shadow-sm rounded-2xl relative overflow-hidden">
        <div>
          <span class="text-xs text-indigo-600 font-bold uppercase tracking-wider font-mono">Akurasi Rata-Rata</span>
          <div class="text-4xl font-extrabold text-indigo-700 mt-1.5 font-display">{averagePassingRate}%</div>
        </div>
        <div class="h-12 w-12 bg-indigo-100/50 text-indigo-700 rounded-xl flex items-center justify-center font-bold text-lg select-none">📊</div>
      </Card>

      <Card padding="md" class="border-red-100/70 bg-red-50/20 hover:bg-red-50/30 transition-all duration-300 flex items-center justify-between shadow-sm rounded-2xl relative overflow-hidden">
        <div>
          <span class="text-xs text-red-600 font-bold uppercase tracking-wider font-mono">Soal Kategori Sulit</span>
          <div class="text-4xl font-extrabold text-red-700 mt-1.5 font-display">
            {metrics.filter(q => q.total_count > 0 && (q.correct_count / q.total_count) < 0.5).length}
          </div>
        </div>
        <div class="h-12 w-12 bg-red-100/50 text-red-700 rounded-xl flex items-center justify-center font-bold text-lg select-none">🔥</div>
      </Card>

      <Card padding="md" class="border-emerald-100/70 bg-emerald-50/20 hover:bg-emerald-50/30 transition-all duration-300 flex items-center justify-between shadow-sm rounded-2xl relative overflow-hidden">
        <div>
          <span class="text-xs text-emerald-600 font-bold uppercase tracking-wider font-mono">Soal Kategori Mudah</span>
          <div class="text-4xl font-extrabold text-emerald-700 mt-1.5 font-display">
            {metrics.filter(q => q.total_count > 0 && (q.correct_count / q.total_count) >= 0.8).length}
          </div>
        </div>
        <div class="h-12 w-12 bg-emerald-100/50 text-emerald-700 rounded-xl flex items-center justify-center font-bold text-lg select-none">✨</div>
      </Card>
    </div>

    <!-- Hardest / Easiest Lists -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
      <!-- Hardest -->
      <div class="space-y-4">
        <h3 class="text-lg font-bold text-slate-800 flex items-center gap-2">
          <span class="h-2.5 w-2.5 bg-red-500 rounded-full"></span>
          Soal Paling Banyak Salah (Difficulty &gt; 50%)
        </h3>
        <div class="flex flex-col gap-4">
          {#each hardestQuestions as q}
            {@const rate = Math.round((q.correct_count / q.total_count) * 100)}
            <Card padding="md" class="bg-white border-slate-200/60 shadow-sm flex flex-col gap-3">
              <div class="flex justify-between items-start gap-2">
                <Badge theme="light" variant="danger" class="font-mono">{q.question_id}</Badge>
                <span class="text-xs font-bold text-red-500">
                  Tingkat Kebenaran: {rate}%
                </span>
              </div>
              <p class="text-sm font-semibold text-slate-800 line-clamp-2 leading-relaxed">
                {q.question_text || '—'}
              </p>
              <div class="text-[11px] text-slate-400 font-medium">
                Salah: {q.total_count - q.correct_count} siswa | Total Percobaan: {q.total_count}
              </div>
            </Card>
          {:else}
            <div class="text-center p-8 bg-white border rounded-2xl text-slate-400 font-medium text-sm">
              Tidak ada soal dengan kesulitan tinggi.
            </div>
          {/each}
        </div>
      </div>

      <!-- Easiest -->
      <div class="space-y-4">
        <h3 class="text-lg font-bold text-slate-800 flex items-center gap-2">
          <span class="h-2.5 w-2.5 bg-emerald-500 rounded-full"></span>
          Soal Paling Banyak Benar (Akurasi &gt; 80%)
        </h3>
        <div class="flex flex-col gap-4">
          {#each easiestQuestions as q}
            {@const rate = Math.round((q.correct_count / q.total_count) * 100)}
            <Card padding="md" class="bg-white border-slate-200/60 shadow-sm flex flex-col gap-3">
              <div class="flex justify-between items-start gap-2">
                <Badge theme="light" variant="success" class="font-mono">{q.question_id}</Badge>
                <span class="text-xs font-bold text-emerald-600">
                  Tingkat Kebenaran: {rate}%
                </span>
              </div>
              <p class="text-sm font-semibold text-slate-800 line-clamp-2 leading-relaxed">
                {q.question_text || '—'}
              </p>
              <div class="text-[11px] text-slate-400 font-medium">
                Benar: {q.correct_count} siswa | Total Percobaan: {q.total_count}
              </div>
            </Card>
          {:else}
            <div class="text-center p-8 bg-white border rounded-2xl text-slate-400 font-medium text-sm">
              Tidak ada soal berkategori sangat mudah.
            </div>
          {/each}
        </div>
      </div>
    </div>

    <!-- Complete Grid -->
    <div class="flex flex-col gap-4 mt-4">
      <h3 class="text-lg font-bold text-slate-800">Daftar Agregasi Seluruh Pertanyaan</h3>
      <Table>
        <thead>
          <tr>
            <th class="w-24">ID Soal</th>
            <th>Teks Pertanyaan</th>
            <th class="w-32">Tipe</th>
            <th class="w-32 text-center">Benar / Percobaan</th>
            <th class="w-40">Akurasi Kelulusan</th>
          </tr>
        </thead>
        <tbody>
          {#each metrics as q}
            {@const rate = q.total_count > 0 ? Math.round((q.correct_count / q.total_count) * 100) : 0}
            <tr>
              <td class="font-mono font-bold text-slate-500">{q.question_id}</td>
              <td class="font-medium text-slate-800 line-clamp-1 max-w-lg leading-relaxed pt-3.5">
                {q.question_text || '—'}
              </td>
              <td>
                <Badge theme="light" variant="neutral" class="capitalize">{q.question_type || 'choice'}</Badge>
              </td>
              <td class="text-center font-semibold font-mono text-sm">
                <span class="text-emerald-600">{q.correct_count}</span>
                <span class="text-slate-300">/</span>
                <span class="text-slate-500">{q.total_count}</span>
              </td>
              <td>
                <div class="flex items-center gap-2">
                  <div class="flex-1 bg-slate-100 h-2 rounded-full overflow-hidden border border-slate-200/50">
                    <div 
                      class="h-full rounded-full transition-all duration-300 
                        {rate < 50 ? 'bg-red-500' : rate < 80 ? 'bg-indigo-600' : 'bg-emerald-500'}" 
                      style="width: {rate}%"
                    ></div>
                  </div>
                  <span 
                    class="text-xs font-bold font-mono min-w-[32px] text-right
                      {rate < 50 ? 'text-red-500' : rate < 80 ? 'text-indigo-600' : 'text-emerald-500'}"
                  >
                    {rate}%
                  </span>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </Table>
    </div>
  {/if}
</div>
