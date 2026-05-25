<script lang="ts">
  import { api, qrCodeUrl } from '$lib/api';
  import { authStore } from '$lib/stores/auth';
  import { onMount, onDestroy } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import { toast } from '$lib/stores/toast';

  let roomName = '';
  let supervisorName = '';
  let students: any[] = [];
  let loading = true;
  let pollInterval: any;
  let activeToken = '';

  // Statistics counters
  let totalCount = 0;
  let activeCount = 0;
  let finishedCount = 0;
  let idleCount = 0;

  onMount(async () => {
    // Check authentication
    if (!$authStore.isAuthenticated || $authStore.user?.role !== 'supervisor') {
      const storedToken = localStorage.getItem('aether_token');
      const storedUser = localStorage.getItem('aether_user');
      
      if (storedToken && storedUser) {
        const u = JSON.parse(storedUser);
        if (u.role === 'supervisor') {
          authStore.login(storedToken, u);
        } else {
          window.location.href = '/supervisor/login';
          return;
        }
      } else {
        window.location.href = '/supervisor/login';
        return;
      }
    }

    roomName = $authStore.user?.full_name || 'Ruang Ujian';
    supervisorName = $authStore.user?.username || 'Pengawas';

    try {
      // Get exam token from settings
      const settingsRes = await api('/supervisor/settings');
      if (settingsRes.success) {
        activeToken = settingsRes.data?.token || activeToken;
      }
    } catch {}

    // Load data initially
    await refreshRoomStatus();
    loading = false;

    // Polling every 3 seconds to keep monitoring active
    pollInterval = setInterval(refreshRoomStatus, 3000);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  async function refreshRoomStatus() {
    try {
      const res = await api('/supervisor/room-status');
      if (res.success) {
        students = res.data || [];
        calculateStats();
      }
    } catch (e: any) {
      console.error('Failed to poll supervisor status:', e);
    }
  }

  function calculateStats() {
    totalCount = students.length;
    activeCount = students.filter(s => s.is_logged_in).length;
    finishedCount = students.filter(s => s.hasil_status === 'submitted').length;
    idleCount = totalCount - activeCount - finishedCount;
    if (idleCount < 0) idleCount = 0;
  }

  async function resetStudent(pesertaId: number, studentName: string) {
    const confirm = window.confirm(`Apakah Anda yakin ingin mereset sesi siswa "${studentName}"?\nTindakan ini akan mengeluarkan siswa dari ujian aktif.`);
    if (!confirm) return;

    try {
      const res = await api('/supervisor/reset', {
        method: 'POST',
        body: JSON.stringify({ peserta_id: pesertaId })
      });

      if (res.success) {
        toast.success(`Sesi siswa "${studentName}" berhasil direset!`);
        await refreshRoomStatus();
      }
    } catch (e: any) {
      toast.error('Gagal mereset sesi: ' + e.message);
    }
  }

  function logout() {
    authStore.logout();
    toast.info('Logout berhasil.');
    window.location.href = '/supervisor/login';
  }

  function formatTime(timeStr: string | undefined): string {
    if (!timeStr) return '—';
    try {
      const d = new Date(timeStr);
      return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    } catch {
      return '—';
    }
  }
</script>

<svelte:head>
  <title>Dashboard Pengawas: {roomName} - Aether CBT</title>
</svelte:head>

<div class="min-h-screen bg-slate-50 flex flex-col">
  <!-- Nav header -->
  <header class="bg-white border-b sticky top-0 z-10 px-6 py-4">
    <div class="max-w-7xl mx-auto flex justify-between items-center">
      <div class="flex items-center gap-3">
        <span class="text-xl font-bold tracking-tight text-indigo-600">Aether CBT</span>
        <span class="text-xs px-2 py-0.5 bg-teal-100 text-teal-700 rounded-full font-semibold">Pengawas</span>
      </div>

      <div class="flex items-center gap-4 text-sm">
        <div class="text-right">
          <div class="font-bold text-slate-800">{roomName}</div>
          <div class="text-xs text-slate-500">Pengawas: @{supervisorName}</div>
        </div>
        <Button variant="ghost" size="sm" class="text-red-600 hover:text-red-700 hover:bg-red-50" on:click={logout}>
          Keluar
        </Button>
      </div>
    </div>
  </header>

  <main class="flex-1 max-w-7xl w-full mx-auto p-6 md:p-8 flex flex-col gap-8">
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h2 class="text-3xl font-bold text-slate-900">Pemantauan Ruangan Live</h2>
        <p class="text-slate-500 text-sm">Status real-time pengerjaan peserta ujian di {roomName}. Data diperbarui otomatis.</p>
      </div>
      
      <Button variant="secondary" size="sm" class="flex items-center gap-2" on:click={refreshRoomStatus}>
        <svg class="h-4 w-4 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 1121.21 8H18.2" />
        </svg>
        Segarkan
      </Button>
    </div>

    <!-- Statistics Panel Grid -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-6">
      <Card padding="md" class="border-slate-100/80 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div class="absolute left-0 top-0 h-full w-[4px] bg-slate-400"></div>
        <div>
          <span class="text-xs text-slate-400 font-bold uppercase tracking-wider">Total Siswa</span>
          <div class="text-4xl font-extrabold text-slate-800 mt-1">{totalCount}</div>
        </div>
      </Card>

      <Card padding="md" class="border-indigo-100/80 bg-indigo-50/10 flex items-center justify-between shadow-sm relative overflow-hidden">
        <div class="absolute left-0 top-0 h-full w-[4px] bg-indigo-500"></div>
        <div>
          <span class="text-xs text-indigo-500 font-bold uppercase tracking-wider">Sedang Mengerjakan</span>
          <div class="text-4xl font-extrabold text-indigo-700 mt-1">{activeCount}</div>
        </div>
      </Card>

      <Card padding="md" class="border-emerald-100/80 bg-emerald-50/10 flex items-center justify-between shadow-sm relative overflow-hidden">
        <div class="absolute left-0 top-0 h-full w-[4px] bg-emerald-500"></div>
        <div>
          <span class="text-xs text-emerald-500 font-bold uppercase tracking-wider">Selesai Ujian</span>
          <div class="text-4xl font-extrabold text-emerald-700 mt-1">{finishedCount}</div>
        </div>
      </Card>

      <Card padding="md" class="border-amber-100/80 bg-amber-50/10 flex items-center justify-between shadow-sm relative overflow-hidden">
        <div class="absolute left-0 top-0 h-full w-[4px] bg-amber-500"></div>
        <div>
          <span class="text-xs text-amber-500 font-bold uppercase tracking-wider">Idle / Belum Mulai</span>
          <div class="text-4xl font-extrabold text-amber-700 mt-1">{idleCount}</div>
        </div>
      </Card>
    </div>

    <!-- Core Layout Grid: List vs Token Card -->
    <div class="grid grid-cols-1 lg:grid-cols-4 gap-8 items-start">
      <!-- Live list (3/4) -->
      <div class="lg:col-span-3 flex flex-col gap-4">
        <h3 class="text-lg font-bold uppercase tracking-wider text-slate-500">Daftar Peserta Ruang</h3>
        
        {#if loading}
          <div class="bg-white border rounded-2xl p-20 flex flex-col items-center justify-center gap-3">
            <svg class="animate-spin h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <p class="text-sm font-semibold text-slate-500">Memuat status siswa...</p>
          </div>
        {:else}
          <Table>
            <thead>
              <tr>
                <th>No. ID</th>
                <th>Nama Peserta</th>
                <th>Kelas</th>
                <th>Status / Progres</th>
                <th>Mapel Aktif</th>
                <th>Mulai</th>
                <th>Skor</th>
                <th class="text-center">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#each students as s}
                {@const isSubmitted = s.hasil_status === 'submitted'}
                {@const isWorking = s.is_logged_in}
                
                <tr>
                  <td class="font-mono font-bold text-slate-500">{s.no_id}</td>
                  <td class="font-semibold">
                    <div class="flex items-center gap-2">
                      <span class="text-slate-800">{s.nama_peserta}</span>
                      {#if s.tab_switches > 0 && s.tab_switches < 3}
                        <span class="px-2 py-0.5 bg-red-50 text-red-600 text-[10px] font-bold border border-red-100 rounded-lg flex items-center gap-1 animate-pulse" title="Siswa keluar dari layar ujian">
                          ⚠️ {s.tab_switches}x Tab
                        </span>
                      {/if}
                    </div>
                  </td>
                  <td class="text-slate-600 font-medium">{s.nama_kelas}</td>
                  <td>
                    <div class="flex flex-col gap-1.5">
                      <div class="flex items-center gap-1.5">
                        {#if s.tab_switches >= 3}
                          <span class="px-2.5 py-0.5 bg-red-600 text-white text-[10px] font-extrabold border border-red-500 rounded-md animate-pulse">
                            ⚠️ TERKUNCI
                          </span>
                        {:else if isSubmitted}
                          <Badge variant="success">Selesai</Badge>
                        {:else if isWorking}
                          <Badge variant="info">Mengerjakan</Badge>
                        {:else}
                          <Badge variant="neutral">Idle</Badge>
                        {/if}
                      </div>

                      {#if isWorking && s.total_questions > 0}
                        {@const percent = Math.round((s.answered_count / s.total_questions) * 100)}
                        <div class="w-32 flex items-center gap-2">
                          <div class="flex-1 bg-slate-200 h-1.5 rounded-full overflow-hidden">
                            <div class="bg-indigo-600 h-full rounded-full" style="width: {percent}%"></div>
                          </div>
                          <span class="text-[10px] font-bold font-mono text-slate-500">{s.answered_count}/{s.total_questions}</span>
                        </div>
                      {/if}
                    </div>
                  </td>
                  <td class="font-medium text-slate-600">{s.nama_mapel || '—'}</td>
                  <td class="font-mono text-xs">{formatTime(s.login_time)}</td>
                  <td>
                    {#if isSubmitted && s.skor !== undefined}
                      <span class="font-bold text-slate-800">{s.skor}</span>
                      <span class="text-xs text-slate-400">/ {s.skor_maks}</span>
                    {:else}
                      <span class="text-slate-400">—</span>
                    {/if}
                  </td>
                  <td class="text-center">
                    {#if isWorking}
                      <Button variant="danger" size="sm" class="px-3 py-1 font-semibold" on:click={() => resetStudent(s.id, s.nama_peserta)}>
                        Reset Sesi
                      </Button>
                    {:else}
                      <span class="text-slate-400 font-mono text-xs">—</span>
                    {/if}
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="8" class="text-center py-16 text-slate-500 font-medium">
                    Belum ada siswa terdaftar di {roomName}.<br>Harap daftarkan siswa dengan Ruang ID ini di panel admin.
                  </td>
                </tr>
              {/each}
            </tbody>
          </Table>
        {/if}
      </div>

      <!-- Token and QR Code card (1/4) -->
      <div class="lg:col-span-1 flex flex-col gap-6 sticky top-24">
        <h3 class="text-lg font-bold uppercase tracking-wider text-slate-500">Token Ruangan</h3>

        <Card padding="md" class="border-slate-200/50 bg-white text-center shadow-sm">
          <div class="text-xs text-slate-400 font-bold uppercase tracking-wider mb-2">Token Ujian Aktif</div>
          <div class="text-3xl font-extrabold text-indigo-600 font-mono tracking-wider mb-6 bg-indigo-50/50 py-3 rounded-2xl border border-indigo-100">
            {activeToken}
          </div>

          <div class="text-xs text-slate-400 font-bold uppercase tracking-wider mb-3">Tampilkan QR Code Ujian</div>
          
          {#if activeToken}
            <div class="bg-slate-50 p-4 border rounded-3xl inline-block mx-auto mb-4">
              <!-- Fetch QR Code from backend Go endpoint using the active token! -->
              <img src={qrCodeUrl(activeToken)} alt="QR Token" class="h-44 w-44 mx-auto" />
            </div>
          {/if}

          <p class="text-xs text-slate-500 leading-relaxed px-2">
            Siswa dapat memindai QR Code di atas dengan perangkat mereka untuk melakukan verifikasi token login secara instan.
          </p>
        </Card>
      </div>
    </div>
  </main>

  <footer class="bg-white border-t py-4 px-6 text-center text-xs text-slate-500">
    Aether CBT • Sistem Monitoring Ruangan Ujian Cerdas & Terintegrasi
  </footer>
</div>
