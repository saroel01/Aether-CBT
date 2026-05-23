<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import { toast } from '$lib/stores/toast';

  let pesertaId = '';
  let pesertaNoId = '';
  let examToken = '';
  
  let examInfo = {
    exam_title: 'Memuat data...',
    is_exam_active: false,
    proctor_name: '',
    footer_text: ''
  };

  let subjects: any[] = [];
  let loading = true;

  onMount(async () => {
    pesertaId = localStorage.getItem('peserta_id') || '';
    pesertaNoId = localStorage.getItem('peserta_no_id') || '';
    examToken = localStorage.getItem('exam_token') || '';

    if (!pesertaId) {
      toast.error('Sesi Anda habis. Silakan login kembali.');
      window.location.href = '/student/login';
      return;
    }

    try {
      // Load active exam settings
      const infoRes = await api('/student/active-info');
      if (infoRes.success) {
        examInfo = infoRes.data;
      }

      // Load subjects filtered by curriculum mapping for student's class!
      const mapelRes = await api(`/student/mapels?peserta_id=${pesertaId}`);
      if (mapelRes.success) {
        subjects = mapelRes.data || [];
      }
    } catch (e: any) {
      toast.error('Gagal mengambil data mata pelajaran. Pastikan koneksi server aktif.');
    }
    loading = false;
  });

  async function startExam(mapelId: number, mapelName: string) {
    try {
      loading = true;
      const res = await api('/student/start', {
        method: 'POST',
        body: JSON.stringify({
          peserta_id: parseInt(pesertaId),
          mapel_id: mapelId
        })
      });

      if (res.success) {
        localStorage.setItem('selected_mapel_id', mapelId.toString());
        localStorage.setItem('selected_mapel_name', mapelName);
        toast.success(`Membuka Ujian: ${mapelName}`);
        
        setTimeout(() => {
          window.location.href = '/student/exam';
        }, 600);
      }
    } catch (e: any) {
      toast.error('Gagal memulai sesi ujian: ' + e.message);
      loading = false;
    }
  }

  function logout() {
    localStorage.clear();
    toast.info('Keluar dari sesi ujian.');
    window.location.href = '/student/login';
  }
</script>

<svelte:head>
  <title>Pilih Ujian - Aether CBT</title>
</svelte:head>

<div class="min-h-screen bg-slate-900 text-white flex flex-col">
  <!-- Nav header -->
  <header class="border-b border-slate-800 bg-slate-950/80 backdrop-blur-md sticky top-0 z-10 px-6 py-4">
    <div class="max-w-6xl mx-auto flex justify-between items-center">
      <div class="flex items-center gap-3">
        <span class="text-xl font-bold tracking-tight text-indigo-400">Aether CBT</span>
        <span class="text-xs px-2 py-0.5 bg-indigo-950/60 text-indigo-300 border border-indigo-900 rounded-full font-medium">Siswa</span>
      </div>

      <div class="flex items-center gap-4 text-sm">
        <div class="text-right hidden sm:block">
          <div class="font-semibold text-slate-200">No. Peserta: {pesertaNoId}</div>
          <div class="text-xs text-slate-400">{examInfo.exam_title}</div>
        </div>
        <Button variant="ghost" size="sm" class="text-red-400 hover:text-red-300 hover:bg-red-950/30" on:click={logout}>
          Keluar
        </Button>
      </div>
    </div>
  </header>

  <main class="flex-1 max-w-6xl w-full mx-auto p-6 md:p-8 flex flex-col">
    <!-- Active title banner -->
    <div class="bg-gradient-to-r from-indigo-950/60 via-slate-900/40 to-indigo-950/60 border border-indigo-900/30 p-6 rounded-3xl mb-8 relative overflow-hidden">
      <div class="absolute -top-10 -right-10 w-40 h-40 bg-indigo-500/10 rounded-full blur-3xl"></div>
      <div class="relative z-10">
        <h2 class="text-3xl font-extrabold text-white tracking-tight mb-2">{examInfo.exam_title}</h2>
        <p class="text-slate-400 text-sm">Selamat datang. Silakan pilih mata pelajaran di bawah ini untuk memulai pengerjaan ujian.</p>
      </div>
    </div>

    <h3 class="text-lg font-semibold uppercase tracking-wider text-slate-400 mb-4">Mata Pelajaran Aktif</h3>

    {#if loading}
      <div class="flex-1 flex flex-col items-center justify-center py-20 text-slate-400 gap-3">
        <svg class="animate-spin h-8 w-8 text-indigo-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        <p class="text-sm font-medium">Memuat daftar mata pelajaran...</p>
      </div>
    {:else}
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {#each subjects as s}
          <Card padding="lg" class="border-slate-800/80 bg-slate-900/40 hover:bg-slate-900/60 transition-all duration-300 group flex flex-col justify-between h-48 relative overflow-hidden">
            <div class="absolute -bottom-8 -right-8 w-24 h-24 bg-indigo-500/5 rounded-full group-hover:scale-150 transition-all duration-300"></div>

            <div>
              <span class="text-xs px-2.5 py-1 bg-indigo-950/60 text-indigo-300 border border-indigo-900 rounded font-mono font-medium tracking-wide">
                {s.kode_mapel}
              </span>
              <h4 class="text-xl font-bold text-white mt-3 group-hover:text-indigo-400 transition-colors">
                {s.nama_mapel}
              </h4>
            </div>

            <div class="pt-4 z-10">
              <Button 
                variant="primary" 
                size="sm" 
                class="w-full bg-indigo-600 hover:bg-indigo-700 text-white font-medium shadow-md shadow-indigo-900/20 group-hover:scale-[1.02]"
                on:click={() => startExam(s.id, s.nama_mapel)}
              >
                Mulai Ujian
              </Button>
            </div>
          </Card>
        {:else}
          <div class="col-span-full bg-slate-900/30 border border-slate-800 rounded-3xl p-12 text-center text-slate-500">
            <svg class="h-12 w-12 mx-auto text-slate-600 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p class="font-medium text-slate-400">Belum ada mata pelajaran aktif.</p>
            <p class="text-xs text-slate-600 mt-1">Harap hubungi Admin atau Proktor sekolah untuk mengaktifkan soal.</p>
          </div>
        {/each}
      </div>
    {/if}
  </main>

  <footer class="border-t border-slate-800 bg-slate-950/50 py-4 px-6 text-center text-xs text-slate-600">
    {examInfo.footer_text || 'Aether CBT - Modern Computer-Based Testing Platform'}
  </footer>
</div>
