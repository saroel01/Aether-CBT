<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '#lib/api.js';
  import Button from '#lib/components/ui/Button.svelte';
  import Card from '#lib/components/ui/Card.svelte';
  import Badge from '#lib/components/ui/Badge.svelte';
  import { toast } from '#lib/stores/toast.js';

  // The selection screen lists the sessions the student is eligible for via
  // /student/my-sessions (Requirement 5.3, 6.4). The legacy mapel fallback was removed
  // together with the server's mapel_id start path (audit M6).

  let pesertaId = '';
  let pesertaNoId = '';

  let examInfo = {
    exam_title: 'Memuat data...',
    is_exam_active: false,
    proctor_name: '',
    footer_text: ''
  };

  let sessions: any[] = [];
  let loading = true;

  function sessionLabel(s: any): string {
    return s.nama || `Sesi #${s.id}`;
  }

  onMount(async () => {
    pesertaId = localStorage.getItem('peserta_id') || '';
    pesertaNoId = localStorage.getItem('peserta_no_id') || '';

    if (!pesertaId) {
      toast.error('Sesi Anda habis. Silakan login kembali.');
      window.location.href = '/student/login';
      return;
    }

    try {
      const infoRes = await api('/student/active-info');
      if (infoRes.success) examInfo = infoRes.data;

      const sessRes = await api('/student/my-sessions').catch(() => ({ data: [] }));
      sessions = sessRes.data || [];
    } catch {
      toast.error('Gagal mengambil data sesi ujian. Pastikan koneksi server aktif.');
    }
    loading = false;
  });

  // Start via the session path (Requirement 7.3): sends session_id, server issues the
  // attempt_token + sets the content-session cookie.
  async function startSession(s: any) {
    try {
      loading = true;
      const res = await api('/student/start', {
        method: 'POST',
        body: JSON.stringify({
          peserta_id: parseInt(pesertaId),
          session_id: s.id,
          // M1: the server requires the token of this exact session (entered at login).
          token: localStorage.getItem('exam_token') || ''
        })
      });

      if (res.success) {
        localStorage.setItem('session_id', String(s.id));
        localStorage.setItem('selected_mapel_name', sessionLabel(s));
        localStorage.setItem('attempt_token', res.data?.attempt_token || '');
        toast.success(`Membuka: ${sessionLabel(s)}`);
        setTimeout(() => { window.location.href = '/student/exam'; }, 600);
      }
    } catch (e: any) {
      toast.error('Gagal memulai sesi ujian: ' + e.message);
      loading = false;
    }
  }

  function logout() {
    ['peserta_id', 'peserta_no_id', 'exam_token', 'aether_token', 'aether_user', 'session_id', 'attempt_token', 'selected_mapel_name'].forEach(k => {
      try { localStorage.removeItem(k); } catch {}
    });
    toast.info('Keluar dari sesi ujian.');
    window.location.href = '/student/login';
  }
</script>

<svelte:head>
  <title>Pilih Ujian - Aether CBT</title>
</svelte:head>

<div class="min-h-dvh bg-slate-950 bg-grid-sovereign text-slate-100 flex flex-col justify-between select-none relative overflow-hidden">
  <header class="border-b border-slate-800 bg-slate-950/80 backdrop-blur-md sticky top-0 z-10 px-6 py-4">
    <div class="max-w-6xl mx-auto flex justify-between items-center">
      <div class="flex items-center gap-3">
        <span class="text-lg font-bold tracking-tight text-cobalt-400 font-display">AETHER CBT</span>
        <Badge variant="cobalt" theme="dark">Siswa</Badge>
      </div>
      <div class="flex items-center gap-6 text-sm">
        <div class="text-right hidden sm:block">
          <div class="font-bold text-slate-200">No. Peserta: <span class="font-mono tabular-nums">{pesertaNoId}</span></div>
          <div class="text-xs text-slate-400 font-medium">{examInfo.exam_title}</div>
        </div>
        <Button variant="ghost" size="sm" class="text-ruby-400 hover:text-ruby-300 hover:bg-ruby-950/30" on:click={logout}>
          Keluar
        </Button>
      </div>
    </div>
  </header>

  <main class="flex-1 max-w-6xl w-full mx-auto p-6 md:p-8 flex flex-col z-10">
    <div class="bg-slate-900 border border-slate-800 p-6 md:p-8 rounded-2xl mb-8 shadow-xs">
      <span class="text-xs uppercase font-bold tracking-wider text-cobalt-400 block mb-2 font-display">Dashboard Sesi Siswa</span>
      <h2 class="text-2xl md:text-3xl font-extrabold text-white tracking-tight mb-2 font-display">{examInfo.exam_title}</h2>
      <p class="text-slate-400 text-sm leading-relaxed max-w-2xl text-pretty">
        Selamat datang. Pilih salah satu sesi ujian aktif yang dijadwalkan untuk Anda di bawah ini untuk memulai pengerjaan.
      </p>
    </div>

    {#if loading}
      <div class="flex-1 flex flex-col items-center justify-center py-20 text-slate-400 gap-3">
        <svg class="animate-spin h-8 w-8 text-cobalt-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
        </svg>
        <p class="text-sm font-semibold">Mengambil daftar sesi ujian...</p>
      </div>
    {:else}
      <!-- Session-model list (preferred) -->
      {#if sessions.length > 0}
        <h3 class="text-xs font-bold uppercase tracking-wider text-slate-400 mb-6 font-mono">Sesi Ujian Aktif</h3>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {#each sessions as s}
            <Card theme="dark" padding="lg" class="border-slate-800 bg-slate-900/90 hover:border-slate-700 transition-colors shadow-xs flex flex-col justify-between rounded-2xl min-h-56 group">
              <div>
                <div class="flex justify-between items-center gap-2">
                  <Badge variant="cobalt" theme="dark">{s.status}</Badge>
                  {#if s.enterable}
                    <Badge variant="emerald" theme="dark">Dapat Dimasuki</Badge>
                  {/if}
                </div>
                <h4 class="text-xl font-bold text-slate-100 mt-4 group-hover:text-cobalt-400 transition-colors font-display leading-tight">{sessionLabel(s)}</h4>
                <p class="text-xs text-slate-400 mt-2 font-mono tabular-nums">
                  {new Date(s.waktu_mulai).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' })}
                  → {new Date(s.waktu_selesai).toLocaleString('id-ID', { timeStyle: 'short' })}
                </p>
              </div>
              <div class="pt-5 z-10">
                <Button
                  variant="primary"
                  size="md"
                  class="w-full font-semibold"
                  disabled={!s.enterable}
                  on:click={() => startSession(s)}
                >
                  {s.enterable ? 'Mulai Ujian' : 'Belum Dapat Dimasuki'}
                </Button>
              </div>
            </Card>
          {/each}
        </div>
      {:else}
        <div class="col-span-full bg-slate-900/40 border border-slate-800 rounded-2xl p-12 text-center text-slate-400 shadow-xs">
          <svg class="h-10 w-10 mx-auto text-slate-500 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="font-bold text-slate-200 text-base font-display">Belum Ada Sesi Ujian Aktif</p>
          <p class="text-xs text-slate-400 mt-1.5 max-w-sm mx-auto leading-relaxed text-pretty">
            Tidak ada sesi ujian yang dijadwalkan untuk Anda saat ini. Silakan hubungi pengawas ruangan.
          </p>
        </div>
      {/if}
    {/if}
  </main>

  <footer class="border-t border-slate-800 bg-slate-950/60 py-4 px-6 text-center text-xs text-slate-400 font-mono">
    {examInfo.footer_text || 'Aether CBT • Sistem Ujian Komputasi Multi-Tenant Berkinerja Tinggi'}
  </footer>
</div>
