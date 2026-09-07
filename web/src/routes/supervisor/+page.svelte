<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, qrCodeUrl } from '$lib/api';
  import { authStore } from '$lib/stores/auth';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { toast } from '$lib/stores/toast';

  let roomName = '';
  let supervisorName = '';
  let students: any[] = [];
  let searchQuery = '';

  $: filteredStudents = students.filter(s => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase().trim();
    const idMatch = (s.no_id || '').toLowerCase().includes(q);
    const nameMatch = (s.nama_peserta || '').toLowerCase().includes(q);
    const classMatch = (s.nama_kelas || '').toLowerCase().includes(q);
    const mapelMatch = (s.nama_mapel || '').toLowerCase().includes(q);
    return idMatch || nameMatch || classMatch || mapelMatch;
  });

  let loading = true;
  let pollInterval: any;
  let activeToken = '';

  // Statistics counters
  let totalCount = 0;
  let activeCount = 0;
  let finishedCount = 0;
  let idleCount = 0;

  // Modals state
  let showProjectorModal = false;

  // Reset Student Confirmation Modal state
  let showResetModal = false;
  let resetTargetId: number | null = null;
  let resetTargetName = '';
  let resetLoading = false;

  // Unlocking action state
  let unlockingId: number | null = null;

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
    finishedCount = students.filter(s => s.status === 'submitted' || s.hasil_status === 'submitted').length;
    idleCount = totalCount - activeCount - finishedCount;
    if (idleCount < 0) idleCount = 0;
  }

  async function unlockStudent(pesertaId: number, sessionId: number | undefined, studentName: string) {
    unlockingId = pesertaId;
    try {
      const res = await api('/supervisor/unlock', {
        method: 'POST',
        body: JSON.stringify({ peserta_id: pesertaId, session_id: sessionId || 0 })
      });

      if (res.success) {
        toast.success(`Kunci ujian siswa "${studentName}" berhasil dibuka!`);
        await refreshRoomStatus();
      } else {
        toast.error('Gagal membuka kunci: ' + (res.error || 'Terjadi kesalahan'));
      }
    } catch (e: any) {
      toast.error('Gagal membuka kunci: ' + e.message);
    } finally {
      unlockingId = null;
    }
  }

  function promptResetStudent(pesertaId: number, studentName: string) {
    resetTargetId = pesertaId;
    resetTargetName = studentName;
    showResetModal = true;
  }

  async function handleConfirmReset() {
    if (!resetTargetId) return;
    resetLoading = true;
    try {
      const res = await api('/supervisor/reset', {
        method: 'POST',
        body: JSON.stringify({ peserta_id: resetTargetId })
      });

      if (res.success) {
        toast.success(`Sesi siswa "${resetTargetName}" berhasil direset!`);
        showResetModal = false;
        await refreshRoomStatus();
      } else {
        toast.error('Gagal mereset sesi: ' + (res.error || 'Terjadi kesalahan'));
      }
    } catch (e: any) {
      toast.error('Gagal mereset sesi: ' + e.message);
    } finally {
      resetLoading = false;
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
  <meta name="description" content="Dashboard pemantauan ruangan ujian pengawas Aether CBT" />
</svelte:head>

<div class="min-h-dvh bg-slate-50 flex flex-col justify-between select-none">
  <!-- Nav header -->
  <header class="bg-white border-b border-slate-200 sticky top-0 z-20 px-4 sm:px-6 py-3.5 shadow-sm">
    <div class="max-w-7xl mx-auto flex justify-between items-center">
      <div class="flex items-center gap-3">
        <span class="text-lg font-bold tracking-tight text-cobalt-600 font-display">AETHER CBT</span>
        <span class="text-[10px] px-2.5 py-0.5 bg-cobalt-50 text-cobalt-700 border border-cobalt-200 rounded-full font-bold uppercase tracking-wider font-mono">Pengawas Ruang</span>
      </div>

      <div class="flex items-center gap-4 sm:gap-6 text-sm">
        <div class="text-right">
          <div class="font-bold text-slate-800">{roomName}</div>
          <div class="text-xs text-slate-500 font-medium">@{supervisorName}</div>
        </div>
        <Button variant="ghost" size="sm" theme="light" class="text-ruby-600 hover:text-ruby-700 hover:bg-ruby-50" on:click={logout}>
          Keluar
        </Button>
      </div>
    </div>
  </header>

  <!-- Main Workspace -->
  <main class="flex-1 max-w-7xl w-full mx-auto p-4 sm:p-6 lg:p-8 flex flex-col gap-6 z-10">
    <!-- Header Summary -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-2xl font-bold text-slate-900 font-display">Pemantauan Ruangan Ujian</h1>
        <p class="text-slate-500 text-sm mt-0.5">Status pengerjaan peserta ruang real-time. Data diperbarui otomatis setiap 3 detik.</p>
      </div>
      
      <Button variant="secondary" size="sm" theme="light" class="flex items-center gap-2 font-medium" on:click={refreshRoomStatus}>
        <svg class="h-4 w-4 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 1121.21 8H18.2" />
        </svg>
        Segarkan Data
      </Button>
    </div>

    <!-- Statistics Panel Grid (Compact metric cards with tabular-nums font-mono) -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <Card padding="sm" class="border-slate-200 bg-white flex items-center justify-between shadow-sm">
        <div>
          <span class="text-xs text-slate-500 font-bold uppercase tracking-wider font-mono">Total Siswa</span>
          <div class="text-3xl font-extrabold text-slate-900 mt-1 font-mono tabular-nums tracking-tight">{totalCount}</div>
        </div>
        <div class="h-10 w-10 bg-slate-100 text-slate-600 rounded-xl flex items-center justify-center font-bold text-sm">∑</div>
      </Card>

      <Card padding="sm" class="border-slate-200 bg-white flex items-center justify-between shadow-sm">
        <div>
          <span class="text-xs text-cobalt-600 font-bold uppercase tracking-wider font-mono">Sedang Ujian</span>
          <div class="text-3xl font-extrabold text-cobalt-600 mt-1 font-mono tabular-nums tracking-tight">{activeCount}</div>
        </div>
        <div class="h-10 w-10 bg-cobalt-50 text-cobalt-600 rounded-xl flex items-center justify-center font-bold text-sm">✎</div>
      </Card>

      <Card padding="sm" class="border-slate-200 bg-white flex items-center justify-between shadow-sm">
        <div>
          <span class="text-xs text-emerald-600 font-bold uppercase tracking-wider font-mono">Selesai Ujian</span>
          <div class="text-3xl font-extrabold text-emerald-600 mt-1 font-mono tabular-nums tracking-tight">{finishedCount}</div>
        </div>
        <div class="h-10 w-10 bg-emerald-50 text-emerald-600 rounded-xl flex items-center justify-center font-bold text-sm">✓</div>
      </Card>

      <Card padding="sm" class="border-slate-200 bg-white flex items-center justify-between shadow-sm">
        <div>
          <span class="text-xs text-amber-600 font-bold uppercase tracking-wider font-mono">Belum Mulai</span>
          <div class="text-3xl font-extrabold text-amber-600 mt-1 font-mono tabular-nums tracking-tight">{idleCount}</div>
        </div>
        <div class="h-10 w-10 bg-amber-50 text-amber-600 rounded-xl flex items-center justify-center font-bold text-sm">⏰</div>
      </Card>
    </div>

    <!-- Core Layout Grid: High-Density Monitoring Table vs Token Card -->
    <div class="grid grid-cols-1 lg:grid-cols-4 gap-6 items-start">
      <!-- High-Density Monitoring Cockpit Table (3/4 Grid) -->
      <div class="lg:col-span-3 flex flex-col gap-3">
        <div class="flex items-center justify-between">
          <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 font-mono">Daftar Peserta Ruangan ({students.length})</h2>
          <span class="text-[11px] text-slate-500 font-mono">Kerapatan: Tinggi (Cockpit Mode)</span>
        </div>
        
        {#if loading}
          <div class="bg-white border border-slate-200 rounded-2xl p-16 flex flex-col items-center justify-center gap-3">
            <svg class="animate-spin h-8 w-8 text-cobalt-600" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <p class="text-sm font-semibold text-slate-500">Menghubungkan ke monitor proktor...</p>
          </div>
        {:else}
          <!-- Instant Search Toolbar for Cockpit Monitor -->
          <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-white p-3 rounded-2xl border border-slate-200/80 shadow-sm">
            <div class="relative flex-1 max-w-sm">
              <Input 
                type="search"
                placeholder="Cari peserta, NIS/ID, kelas, mapel..." 
                bind:value={searchQuery}
                theme="light"
              >
                <svg slot="iconLeft" class="h-4 w-4 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                <svelte:fragment slot="iconRight">
                  {#if searchQuery}
                    <button 
                      type="button" 
                      class="text-slate-400 hover:text-slate-600 p-1 rounded-full hover:bg-slate-100 transition-colors"
                      on:click={() => searchQuery = ''}
                      title="Bersihkan filter"
                      aria-label="Bersihkan filter"
                    >
                      <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  {/if}
                </svelte:fragment>
              </Input>
            </div>

            <div class="flex items-center gap-3 text-xs font-mono text-slate-500 px-2">
              {#if searchQuery}
                <span>Ditemukan: <strong class="text-slate-900 tabular-nums">{filteredStudents.length}</strong> dari <span class="tabular-nums">{students.length}</span> peserta</span>
              {:else}
                <span>Total: <strong class="text-slate-900 tabular-nums">{students.length}</strong> peserta ruangan</span>
              {/if}
              <span class="text-slate-300">|</span>
              <span class="text-[11px] text-slate-400">Auto-refresh 3s</span>
            </div>
          </div>

          <Table theme="light" compact={true}>
            <thead>
              <tr class="font-display">
                <th class="py-2 px-3">No. ID</th>
                <th class="py-2 px-3">Nama Peserta</th>
                <th class="py-2 px-3">Kelas</th>
                <th class="py-2 px-3">Status / Progres</th>
                <th class="py-2 px-3">Mapel Aktif</th>
                <th class="py-2 px-3">Mulai</th>
                <th class="py-2 px-3">Skor</th>
                <th class="py-2 px-3 text-center">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {#each filteredStudents as s}
                {@const isSubmitted = s.hasil_status === 'submitted' || s.status === 'submitted'}
                {@const isLocked = !!(s.locked || s.status === 'locked')}
                {@const isWorking = s.is_logged_in}
                
                <tr class="hover:bg-slate-50/70 transition-colors">
                  <td class="py-2 px-3 font-mono font-bold text-slate-700 tabular-nums text-xs whitespace-nowrap">{s.no_id}</td>
                  <td class="py-2 px-3 font-semibold text-slate-900 text-sm">
                    <div class="flex items-center gap-2">
                      <span class="truncate max-w-[180px]">{s.nama_peserta}</span>
                      {#if s.tab_switches > 0}
                        <Badge variant="ruby" theme="light" class="font-bold tabular-nums text-[10px] px-1.5 py-0" title="Siswa keluar dari layar ujian">
                          ⚠️ {s.tab_switches}x Tab
                        </Badge>
                      {/if}
                    </div>
                  </td>
                  <td class="py-2 px-3 text-slate-600 font-medium text-xs whitespace-nowrap">{s.nama_kelas}</td>
                  <td class="py-2 px-3">
                    <div class="flex flex-col gap-1">
                      <div class="flex items-center gap-1.5">
                        {#if isLocked}
                          <Badge variant="ruby" theme="light" class="font-extrabold text-[11px]">
                            TERKUNCI
                          </Badge>
                        {:else if isSubmitted}
                          <Badge variant="emerald" theme="light" class="font-semibold text-[11px]">Selesai</Badge>
                        {:else if isWorking}
                          <Badge variant="cobalt" theme="light" class="font-semibold text-[11px]">Mengerjakan</Badge>
                        {:else}
                          <Badge variant="slate" theme="light" class="font-semibold text-[11px]">Idle</Badge>
                        {/if}
                      </div>

                      {#if isWorking && s.total_questions > 0}
                        {@const percent = Math.round((s.answered_count / s.total_questions) * 100)}
                        <div class="w-28 flex items-center gap-1.5 mt-0.5">
                          <div class="flex-1 bg-slate-100 h-1.5 rounded-full overflow-hidden border border-slate-200/50">
                            <div class="bg-cobalt-600 h-full rounded-full transition-all duration-150" style="width: {percent}%"></div>
                          </div>
                          <span class="text-[10px] font-bold font-mono text-slate-500 tabular-nums whitespace-nowrap">{s.answered_count}/{s.total_questions}</span>
                        </div>
                      {/if}
                    </div>
                  </td>
                  <td class="py-2 px-3 font-medium text-slate-700 text-xs whitespace-nowrap">{s.nama_mapel || '—'}</td>
                  <td class="py-2 px-3 font-mono text-slate-600 text-xs tabular-nums whitespace-nowrap">{formatTime(s.login_time)}</td>
                  <td class="py-2 px-3 whitespace-nowrap">
                    {#if isSubmitted && s.skor !== undefined}
                      <span class="font-bold font-mono text-slate-900 tabular-nums text-sm">{s.skor}</span>
                      <span class="text-xs font-mono text-slate-500 tabular-nums"> / {s.skor_maks}</span>
                    {:else}
                      <span class="text-slate-400 font-mono text-xs tabular-nums">—</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3 text-center whitespace-nowrap">
                    {#if isWorking}
                      <div class="flex items-center justify-center gap-1.5">
                        {#if isLocked}
                          <Button 
                            variant="warning" 
                            size="sm" 
                            theme="light" 
                            class="px-2.5 py-1 text-xs font-semibold" 
                            disabled={unlockingId === s.id}
                            loading={unlockingId === s.id}
                            on:click={() => unlockStudent(s.id, s.session_id, s.nama_peserta)}
                          >
                            Buka Kunci
                          </Button>
                        {/if}
                        <Button 
                          variant="danger" 
                          size="sm" 
                          theme="light" 
                          class="px-2.5 py-1 text-xs font-semibold" 
                          on:click={() => promptResetStudent(s.id, s.nama_peserta)}
                        >
                          Reset Sesi
                        </Button>
                      </div>
                    {:else}
                      <span class="text-slate-400 font-mono text-xs tabular-nums">—</span>
                    {/if}
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="8" class="py-8">
                    {#if searchQuery}
                      <EmptyState
                        title="Peserta Tidak Ditemukan"
                        description={`Tidak ditemukan peserta ujian di ruangan ini yang cocok dengan pencarian "${searchQuery}".`}
                        icon="search"
                        actionText="Bersihkan Filter"
                        actionVariant="secondary"
                        on:action={() => searchQuery = ''}
                      />
                    {:else}
                      <EmptyState
                        title="Belum Ada Siswa di Ruangan Ini"
                        description={`Belum ada data peserta yang dialokasikan ke "${roomName}". Pastikan siswa telah terdaftar dengan ID Ruang ini di panel admin.`}
                        icon="users"
                        actionText=""
                      />
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </Table>
        {/if}
      </div>

      <!-- Token and QR Code sidebar card (1/4 Grid) -->
      <div class="lg:col-span-1 flex flex-col gap-3 sticky top-24">
        <h2 class="text-xs font-bold uppercase tracking-widest text-slate-500 font-mono">Token Ruangan</h2>

        <Card padding="md" class="border-slate-200 bg-white text-center shadow-sm relative overflow-hidden">
          <div class="text-[10px] text-slate-500 font-bold uppercase tracking-wider mb-2 font-mono">Token Ujian Aktif</div>
          <div class="text-3xl font-extrabold text-cobalt-600 font-mono tracking-widest mb-4 bg-cobalt-50/70 py-3 rounded-xl border border-cobalt-200/80 tabular-nums select-all">
            {activeToken || '—'}
          </div>

          <div class="text-[10px] text-slate-500 font-bold uppercase tracking-wider mb-2 font-mono">QR Code Verifikasi</div>
          
          {#if activeToken}
            <div class="bg-white p-3 border border-slate-200 rounded-2xl inline-block mx-auto mb-4 shadow-sm">
              <img src={qrCodeUrl(activeToken)} alt="QR Token" class="h-44 w-44 mx-auto object-contain" />
            </div>
          {/if}

          <p class="text-xs text-slate-500 leading-relaxed px-2 mb-4">
            Siswa dapat memindai QR Code di atas dengan perangkat mereka untuk melakukan verifikasi login secara instan.
          </p>

          <!-- Projector Presentation Button -->
          <Button 
            variant="primary" 
            size="md" 
            theme="light" 
            class="w-full flex items-center justify-center gap-2 font-semibold shadow-sm" 
            on:click={() => showProjectorModal = true}
          >
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M7 4v16M17 4v16M3 8h4m10 0h4M3 12h18M3 16h4m10 0h4M4 20h16a1 1 0 001-1V5a1 1 0 00-1-1H4a1 1 0 00-1 1v14a1 1 0 001 1z" />
            </svg>
            Tampilkan di Proyektor
          </Button>
        </Card>
      </div>
    </div>
  </main>

  <!-- Footer -->
  <footer class="bg-white border-t border-slate-200 py-4 px-6 text-center text-xs text-slate-500 font-mono">
    Aether CBT • Sistem Monitoring Ruangan Ujian Cerdas & Terintegrasi
  </footer>
</div>

<!-- Two-Step Confirmation Modal for Student Reset -->
<ConfirmModal
  bind:isOpen={showResetModal}
  title="Konfirmasi Reset Sesi Ujian"
  message={`Apakah Anda yakin ingin mereset sesi siswa "${resetTargetName}"? Tindakan ini akan mengeluarkan siswa dari ujian yang sedang berlangsung dan mengharuskan siswa login kembali.`}
  confirmLabel="Reset Sesi Siswa"
  cancelLabel="Batal"
  variant="danger"
  loading={resetLoading}
  theme="light"
  on:confirm={handleConfirmReset}
  on:cancel={() => { showResetModal = false; }}
/>

<!-- Projector Presentation Modal -->
<Modal 
  bind:show={showProjectorModal} 
  size="xl" 
  theme="dark" 
  title="Tampilan Proyektor Ruang Ujian"
>
  <div class="py-6 px-4 text-center flex flex-col items-center justify-center bg-slate-950 text-white rounded-xl">
    <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-cobalt-950/80 border border-cobalt-800/60 text-cobalt-300 text-xs font-mono font-bold uppercase tracking-widest mb-4">
      <span class="h-2 w-2 rounded-full bg-cobalt-400"></span>
      Token Masuk Ujian Aktif
    </div>

    <div class="text-6xl sm:text-7xl font-black font-mono tracking-widest text-white my-4 tabular-nums select-all px-8 py-4 bg-slate-900 border-2 border-slate-700 rounded-2xl shadow-inner">
      {activeToken || '—'}
    </div>

    {#if activeToken}
      <div class="bg-white p-6 rounded-2xl inline-block shadow-2xl mx-auto my-6 border-4 border-slate-800">
        <img 
          src={qrCodeUrl(activeToken)} 
          alt="QR Token Proyektor" 
          class="h-80 w-80 object-contain mx-auto" 
        />
      </div>
    {/if}

    <div class="max-w-md mx-auto space-y-2">
      <p class="text-base font-medium text-slate-200">
        Arahkan kamera perangkat siswa ke QR Code di atas untuk login langsung, atau masukkan token secara manual.
      </p>
      <p class="text-xs text-slate-400 font-mono">
        Ruangan: <span class="text-slate-200 font-bold">{roomName}</span> • Diperbarui otomatis
      </p>
    </div>
  </div>

  <div slot="footer" class="w-full flex items-center justify-between">
    <span class="text-xs text-slate-400 font-mono">Tekan tombol Esc untuk menutup tampilan proyektor</span>
    <Button 
      variant="secondary" 
      size="sm" 
      theme="dark" 
      on:click={() => showProjectorModal = false}
    >
      Tutup
    </Button>
  </div>
</Modal>
