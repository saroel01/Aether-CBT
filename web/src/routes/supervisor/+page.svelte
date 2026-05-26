<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, qrCodeUrl } from '$lib/api';
  import { authStore } from '$lib/stores/auth';
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

<div class="min-h-screen bg-slate-50 flex flex-col justify-between select-none">
  <!-- Nav header -->
  <header class="bg-white border-b sticky top-0 z-10 px-6 py-4 shadow-sm shadow-slate-100/50">
    <div class="max-w-7xl mx-auto flex justify-between items-center">
      <div class="flex items-center gap-3">
        <span class="text-lg font-bold tracking-tight text-indigo-600 font-display">AETHER CBT</span>
        <span class="text-[10px] px-2.5 py-0.5 bg-indigo-50 text-indigo-700 rounded-full font-bold uppercase tracking-wider font-mono">Pengawas</span>
      </div>

      <div class="flex items-center gap-6 text-sm">
        <div class="text-right">
          <div class="font-bold text-slate-800">{roomName}</div>
          <div class="text-xs text-slate-400 font-medium">@{supervisorName}</div>
        </div>
        <Button variant="ghost" size="sm" theme="light" class="text-red-600 hover:text-red-750" on:click={logout}>
          Keluar
        </Button>
      </div>
    </div>
  </header>

  <!-- Main Workspace -->
  <main class="flex-1 max-w-7xl w-full mx-auto p-6 md:p-8 flex flex-col gap-8 z-10">
    <!-- Header Summary -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h2 class="text-2xl font-bold text-slate-900 font-display">Pemantauan Ruangan Ujian</h2>
        <p class="text-slate-500 text-sm">Status pengerjaan peserta ruang real-time. Data melakukan penyegaran otomatis.</p>
      </div>
      
      <Button variant="secondary" size="sm" theme="light" class="flex items-center gap-2" on:click={refreshRoomStatus}>
        <svg class="h-4 w-4 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 1121.21 8H18.2" />
        </svg>
        Segarkan Data
      </Button>
    </div>

    <!-- Statistics Panel Grid (Restrained Outlines instead of Side-stripe outlines) -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-6">
      <Card padding="md" class="border-slate-200/60 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div>
          <span class="text-[10px] text-slate-400 font-bold uppercase tracking-wider font-mono">Total Siswa</span>
          <div class="text-3xl font-extrabold text-slate-800 mt-1 font-display">{totalCount}</div>
        </div>
        <div class="h-10 w-10 bg-slate-100 text-slate-500 rounded-xl flex items-center justify-center font-bold">∑</div>
      </Card>

      <Card padding="md" class="border-indigo-100 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div>
          <span class="text-[10px] text-indigo-500 font-bold uppercase tracking-wider font-mono">Sedang Ujian</span>
          <div class="text-3xl font-extrabold text-indigo-600 mt-1 font-display">{activeCount}</div>
        </div>
        <div class="h-10 w-10 bg-indigo-50 text-indigo-600 rounded-xl flex items-center justify-center font-bold">✎</div>
      </Card>

      <Card padding="md" class="border-emerald-100 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div>
          <span class="text-[10px] text-emerald-500 font-bold uppercase tracking-wider font-mono">Selesai Ujian</span>
          <div class="text-3xl font-extrabold text-emerald-600 mt-1 font-display">{finishedCount}</div>
        </div>
        <div class="h-10 w-10 bg-emerald-50 text-emerald-600 rounded-xl flex items-center justify-center font-bold">✓</div>
      </Card>

      <Card padding="md" class="border-amber-100 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div>
          <span class="text-[10px] text-amber-500 font-bold uppercase tracking-wider font-mono">Belum Mulai</span>
          <div class="text-3xl font-extrabold text-amber-600 mt-1 font-display">{idleCount}</div>
        </div>
        <div class="h-10 w-10 bg-amber-50 text-amber-600 rounded-xl flex items-center justify-center font-bold">⏰</div>
      </Card>
    </div>

    <!-- Core Layout Grid: List vs Token Card -->
    <div class="grid grid-cols-1 lg:grid-cols-4 gap-8 items-start">
      <!-- Live list (3/4 Grid) -->
      <div class="lg:col-span-3 flex flex-col gap-4">
        <h3 class="text-xs font-bold uppercase tracking-widest text-slate-400 font-mono">Daftar Peserta Ruang</h3>
        
        {#if loading}
          <div class="bg-white border border-slate-100 rounded-2xl p-20 flex flex-col items-center justify-center gap-3">
            <svg class="animate-spin h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <p class="text-sm font-semibold text-slate-400">Menghubungkan ke monitor proktor...</p>
          </div>
        {:else}
          <Table>
            <thead>
              <tr class="font-display">
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
                
                <tr class="hover:bg-slate-50/50 transition-colors">
                  <td class="font-mono font-bold text-slate-400">{s.no_id}</td>
                  <td class="font-semibold text-slate-800">
                    <div class="flex items-center gap-2">
                      <span>{s.nama_peserta}</span>
                      {#if s.tab_switches > 0 && s.tab_switches < 3}
                      <Badge variant="danger" theme="light" class="animate-pulse font-bold" title="Siswa keluar dari layar ujian">
                        ⚠️ {s.tab_switches}x Tab
                      </Badge>
                      {/if}
                    </div>
                  </td>
                  <td class="text-slate-600 font-medium">{s.nama_kelas}</td>
                  <td>
                    <div class="flex flex-col gap-1.5">
                      <div class="flex items-center gap-1.5">
                        {#if s.tab_switches >= 3}
                          <Badge variant="danger" theme="light" class="animate-pulse font-extrabold">
                            ⚠️ TERKUNCI
                          </Badge>
                        {:else if isSubmitted}
                          <Badge variant="success" theme="light">Selesai</Badge>
                        {:else if isWorking}
                          <Badge variant="info" theme="light">Mengerjakan</Badge>
                        {:else}
                          <Badge variant="neutral" theme="light">Idle</Badge>
                        {/if}
                      </div>

                      {#if isWorking && s.total_questions > 0}
                        {@const percent = Math.round((s.answered_count / s.total_questions) * 100)}
                        <div class="w-32 flex items-center gap-2 mt-1">
                          <div class="flex-1 bg-slate-100 h-1.5 rounded-full overflow-hidden">
                            <div class="bg-indigo-600 h-full rounded-full" style="width: {percent}%"></div>
                          </div>
                          <span class="text-[10px] font-bold font-mono text-slate-400">{s.answered_count}/{s.total_questions}</span>
                        </div>
                      {/if}
                    </div>
                  </td>
                  <td class="font-semibold text-slate-700">{s.nama_mapel || '—'}</td>
                  <td class="font-mono text-slate-500 text-xs">{formatTime(s.login_time)}</td>
                  <td>
                    {#if isSubmitted && s.skor !== undefined}
                      <span class="font-bold text-slate-800">{s.skor}</span>
                      <span class="text-xs text-slate-400">/ {s.skor_maks}</span>
                    {:else}
                      <span class="text-slate-350">—</span>
                    {/if}
                  </td>
                  <td class="text-center">
                    {#if isWorking}
                      <Button variant="danger" size="sm" theme="light" class="px-3 py-1 font-semibold" on:click={() => resetStudent(s.id, s.nama_peserta)}>
                        Reset Sesi
                      </Button>
                    {:else}
                      <span class="text-slate-350 font-mono text-xs">—</span>
                    {/if}
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="8" class="text-center py-16 text-slate-400 font-medium">
                    Belum ada siswa terdaftar di {roomName}.<br>Harap daftarkan siswa dengan Ruang ID ini di panel admin.
                  </td>
                </tr>
              {/each}
            </tbody>
          </Table>
        {/if}
      </div>

      <!-- Token and QR Code card (1/4 Grid) -->
      <div class="lg:col-span-1 flex flex-col gap-4 sticky top-24">
        <h3 class="text-xs font-bold uppercase tracking-widest text-slate-400 font-mono">Token Ruangan</h3>

        <Card padding="md" class="border-slate-200/60 bg-white text-center shadow-sm relative overflow-hidden">
          <!-- Minimal top border -->
          <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-100"></div>

          <div class="text-[10px] text-slate-400 font-bold uppercase tracking-wider mb-2 font-mono">Token Ujian Aktif</div>
          <div class="text-3xl font-extrabold text-indigo-600 font-mono tracking-wider mb-6 bg-indigo-50/50 py-3 rounded-2xl border border-indigo-100">
            {activeToken}
          </div>

          <div class="text-[10px] text-slate-400 font-bold uppercase tracking-wider mb-3 font-mono">QR Code Verifikasi</div>
          
          {#if activeToken}
            <div class="bg-slate-50 p-4 border border-slate-100 rounded-3xl inline-block mx-auto mb-4 hover:scale-[1.01] transition-transform duration-300">
              <img src={qrCodeUrl(activeToken)} alt="QR Token" class="h-44 w-44 mx-auto" />
            </div>
          {/if}

          <p class="text-xs text-slate-500 leading-relaxed px-2">
            Siswa dapat memindai QR Code di atas dengan perangkat mereka untuk melakukan verifikasi login secara instan.
          </p>
        </Card>
      </div>
    </div>
  </main>

  <!-- Footer -->
  <footer class="bg-white border-t border-slate-150 py-4 px-6 text-center text-xs text-slate-400 font-mono">
    Aether CBT • Sistem Monitoring Ruangan Ujian Cerdas & Terintegrasi
  </footer>
</div>
