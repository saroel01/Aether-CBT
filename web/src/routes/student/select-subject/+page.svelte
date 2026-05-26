<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
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
        localStorage.setItem('attempt_token', res.data?.attempt_token || '');
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

<div class="min-h-screen bg-slate-950 bg-grid-sovereign text-slate-100 flex flex-col justify-between select-none relative overflow-hidden">
  <!-- Top glow decoration -->
  <div class="absolute top-0 left-1/2 -translate-x-1/2 w-[700px] h-[300px] bg-indigo-500/5 rounded-full blur-[100px] pointer-events-none"></div>

  <!-- Nav header -->
  <header class="border-b border-slate-900 bg-slate-950/80 backdrop-blur-md sticky top-0 z-10 px-6 py-4">
    <div class="max-w-6xl mx-auto flex justify-between items-center">
      <div class="flex items-center gap-3">
        <span class="text-lg font-bold tracking-tight text-indigo-500 font-display">AETHER CBT</span>
        <span class="text-[10px] px-2.5 py-0.5 bg-indigo-950/60 text-indigo-400 border border-indigo-900/40 rounded-full font-semibold uppercase tracking-wider font-mono">Siswa</span>
      </div>

      <div class="flex items-center gap-6 text-sm">
        <div class="text-right hidden sm:block">
          <div class="font-bold text-slate-200">No. Peserta: {pesertaNoId}</div>
          <div class="text-xs text-slate-500 font-medium">{examInfo.exam_title}</div>
        </div>
        <Button variant="ghost" size="sm" class="text-red-400 hover:text-red-300 hover:bg-red-950/20 border border-transparent hover:border-red-950" on:click={logout}>
          Keluar
        </Button>
      </div>
    </div>
  </header>

  <!-- Main Content -->
  <main class="flex-1 max-w-6xl w-full mx-auto p-6 md:p-8 flex flex-col z-10">
    <!-- Active title banner (Premium Academic Banner) -->
    <div class="bg-gradient-to-r from-slate-900/60 via-indigo-950/20 to-slate-900/60 border border-slate-800 p-8 rounded-3xl mb-8 relative overflow-hidden">
      <!-- Glow detail -->
      <div class="absolute -top-12 -right-12 w-48 h-48 bg-indigo-500/10 rounded-full blur-3xl"></div>
      
      <div class="relative z-10">
        <span class="text-[10px] uppercase font-bold tracking-widest text-indigo-500 block mb-2 font-display">Dashboard Sesi Siswa</span>
        <h2 class="text-3xl md:text-4xl font-extrabold text-white tracking-tight mb-2 font-display">{examInfo.exam_title}</h2>
        <p class="text-slate-400 text-sm leading-relaxed max-w-2xl">
          Selamat datang kembali. Silakan pilih salah satu mata pelajaran aktif yang ditugaskan kepada Anda di bawah ini untuk memulai pengerjaan lembar ujian.
        </p>
      </div>
    </div>

    <!-- Section Title -->
    <h3 class="text-xs font-bold uppercase tracking-widest text-slate-500 mb-6 font-mono">
      Daftar Mata Pelajaran Aktif
    </h3>

    <!-- Subjects Display -->
    {#if loading}
      <div class="flex-1 flex flex-col items-center justify-center py-20 text-slate-500 gap-3">
        <svg class="animate-spin h-8 w-8 text-indigo-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        <p class="text-sm font-semibold">Mengambil daftar mata pelajaran...</p>
      </div>
    {:else}
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {#each subjects as s}
          <!-- Redesigned card (Minimalist premium with custom border accent and Outfit typography) -->
          <Card theme="dark" padding="lg" class="border-slate-900 bg-slate-900/40 hover:border-slate-800/80 transition-all duration-300 group flex flex-col justify-between h-52 relative overflow-hidden">
            <!-- Minimal top accent border -->
            <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-800 group-hover:bg-indigo-500/40 transition-colors"></div>

            <div>
              <div class="flex justify-between items-center">
                <span class="text-[10px] px-2.5 py-1 bg-indigo-950/40 text-indigo-400 border border-indigo-900/40 rounded-lg font-mono font-bold tracking-widest">
                  {s.kode_mapel}
                </span>
              </div>
              <h4 class="text-xl font-bold text-slate-100 mt-4 group-hover:text-indigo-400 transition-colors font-display leading-tight">
                {s.nama_mapel}
              </h4>
            </div>

            <div class="pt-4 z-10">
              <Button 
                variant="primary" 
                size="md" 
                class="w-full font-semibold"
                on:click={() => startExam(s.id, s.nama_mapel)}
              >
                Mulai Ujian
              </Button>
            </div>
          </Card>
        {:else}
          <div class="col-span-full bg-slate-900/20 border border-slate-900 rounded-3xl p-12 text-center text-slate-500">
            <svg class="h-10 w-10 mx-auto text-slate-600 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p class="font-bold text-slate-400 text-sm">Belum Ada Sesi Ujian Aktif</p>
            <p class="text-xs text-slate-600 mt-1.5 max-w-sm mx-auto leading-relaxed">
              Mata pelajaran belum diaktifkan atau kelas Anda tidak terdaftar pada modul kurikulum ujian hari ini. Silakan hubungi proktor/pengawas ruangan.
            </p>
          </div>
        {/each}
      </div>
    {/if}
  </main>

  <!-- Footer -->
  <footer class="border-t border-slate-900/50 bg-slate-950/50 py-4 px-6 text-center text-xs text-slate-600 font-mono">
    {examInfo.footer_text || 'Aether CBT • Sistem Ujian Komputasi Multi-Tenant Berkinerja Tinggi'}
  </footer>
</div>
