<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import PasswordGenerator from '$lib/components/PasswordGenerator.svelte';
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { toast } from '$lib/stores/toast';

  let items: any[] = [];
  let searchQuery = '';

  $: filteredItems = items.filter(r => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase().trim();
    const nameMatch = (r.nama_ruang || '').toLowerCase().includes(q);
    const userMatch = (r.username || '').toLowerCase().includes(q);
    const idMatch = String(r.id || '').includes(q);
    return nameMatch || userMatch || idMatch;
  });

  let newName = '';
  let newUsername = '';
  let newPassword = '';
  let loading = true;
  let createLoading = false;

  // Delete confirmation state
  let showDeleteModal = false;
  let roomToDelete: { id: number; name: string } | null = null;
  let deleteLoading = false;

  let visiblePasswords: Record<number, boolean> = {};

  function togglePassword(id: number) {
    visiblePasswords[id] = !visiblePasswords[id];
    visiblePasswords = visiblePasswords;
  }

  onMount(async () => {
    await loadRooms();
  });

  async function loadRooms() {
    loading = true;
    try {
      const res = await api('/rooms');
      items = res.data || [];
    } catch {
      toast.error('Gagal mengambil data ruangan.');
    }
    loading = false;
  }

  async function createRoom() {
    if (!newName || !newUsername || !newPassword) {
      toast.warning('Harap lengkapi nama ruangan, username, dan password!');
      return;
    }

    createLoading = true;
    try {
      await api('/rooms', {
        method: 'POST',
        body: JSON.stringify({ nama_ruang: newName, username: newUsername, password: newPassword })
      });
      toast.success(`Ruang Ujian "${newName}" berhasil terdaftar!`);
      newName = '';
      newUsername = '';
      newPassword = '';
      await loadRooms();
    } catch (e: any) {
      toast.error('Gagal menyimpan ruangan: ' + e.message);
    }
    createLoading = false;
  }

  function promptDeleteRoom(id: number, name: string) {
    roomToDelete = { id, name };
    showDeleteModal = true;
  }

  async function confirmDeleteRoom() {
    if (!roomToDelete) return;
    deleteLoading = true;
    try {
      await api(`/rooms/${roomToDelete.id}`, { method: 'DELETE' });
      toast.success(`Ruang Ujian "${roomToDelete.name}" berhasil dihapus!`);
      showDeleteModal = false;
      roomToDelete = null;
      await loadRooms();
    } catch (e: any) {
      toast.error('Gagal menghapus ruangan: ' + e.message);
    }
    deleteLoading = false;
  }
</script>

<svelte:head>
  <title>Kelola Ruangan - Aether CBT</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="border-b pb-6">
    <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Ruangan Ujian</h1>
    <p class="text-slate-500 text-sm">Kelola daftar ruangan fisik tempat dilangsungkannya ujian.</p>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">
    <!-- List of rooms (2/3) -->
    <div class="lg:col-span-2 flex flex-col gap-4">
      <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
        <div>
          <h3 class="text-lg font-bold uppercase tracking-wider text-slate-500 font-mono">Daftar Ruangan</h3>
          {#if searchQuery}
            <span class="text-xs text-slate-400 font-mono">Ditemukan <strong class="text-slate-700 tabular-nums">{filteredItems.length}</strong> dari <span class="tabular-nums">{items.length}</span> ruangan</span>
          {/if}
        </div>

        <div class="relative w-full sm:w-64">
          <Input 
            type="search"
            placeholder="Cari ruangan atau user..." 
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
                  title="Bersihkan pencarian"
                  aria-label="Bersihkan pencarian"
                >
                  <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              {/if}
            </svelte:fragment>
          </Input>
        </div>
      </div>

      {#if loading}
        <div class="bg-white border rounded-2xl p-16 flex flex-col items-center justify-center gap-3">
          <svg class="animate-spin h-6 w-6 text-indigo-600" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span class="text-xs font-semibold text-slate-500">Memuat data ruangan...</span>
        </div>
      {:else}
        <Table>
          <thead>
            <tr>
              <th class="w-20">ID</th>
              <th>Nama Ruangan</th>
              <th>Username Pengawas</th>
              <th>Sandi default</th>
              <th>Dibuat Pada</th>
              <th class="text-center w-28">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#each filteredItems as r}
              <tr>
                <td class="font-mono text-slate-400 font-bold">{r.id}</td>
                <td class="font-semibold text-slate-800">{r.nama_ruang}</td>
                <td>
                  <span class="font-mono text-xs text-indigo-600 font-bold">
                    @{r.username}
                  </span>
                </td>
                <td class="font-mono text-xs text-slate-500">
                  <div class="flex items-center gap-2 justify-between min-w-[100px]">
                    <span class="font-bold select-text">
                      {visiblePasswords[r.id] ? (r.password || 'ruang123') : '••••••••'}
                    </span>
                    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
                    <button 
                      type="button" 
                      class="text-xs text-slate-400 hover:text-indigo-600 transition"
                      on:click={() => togglePassword(r.id)}
                      title={visiblePasswords[r.id] ? "Sembunyikan" : "Tampilkan"}
                    >
                      {visiblePasswords[r.id] ? '👁️' : '🔑'}
                    </button>
                  </div>
                </td>
                <td class="text-xs text-slate-400 font-mono">
                  {new Date(r.created_at).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' })}
                </td>
                <td class="text-center">
                  <div class="flex items-center justify-center gap-2">
                    <a href="/admin/rooms/print-attendance?room_id={r.id}" target="_blank" title="Cetak Daftar Hadir">
                      <Button variant="secondary" size="sm" theme="light">
                        Absen
                      </Button>
                    </a>
                    <a href="/admin/rooms/print-report?room_id={r.id}" target="_blank" title="Cetak Berita Acara">
                      <Button variant="secondary" size="sm" theme="light">
                        Acara
                      </Button>
                    </a>
                    <Button 
                      variant="danger" 
                      size="sm" 
                      theme="light"
                      on:click={() => promptDeleteRoom(r.id, r.nama_ruang)}
                    >
                      Hapus
                    </Button>
                  </div>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="6" class="py-8">
                  {#if searchQuery}
                    <EmptyState
                      title="Ruangan Tidak Ditemukan"
                      description={`Tidak ditemukan ruangan dengan kata kunci "${searchQuery}".`}
                      icon="search"
                      actionText="Bersihkan Pencarian"
                      actionVariant="secondary"
                      on:action={() => searchQuery = ''}
                    />
                  {:else}
                    <EmptyState
                      title="Belum Ada Ruangan Terdaftar"
                      description="Belum ada ruangan fisik ujian yang didaftarkan. Gunakan formulir di sebelah kanan untuk menambahkan ruangan baru."
                      icon="room"
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

    <!-- Create card (1/3) -->
    <div class="lg:col-span-1">
      <Card padding="md" class="border-slate-200/50 bg-white shadow-sm">
        <h3 class="text-base font-bold text-slate-800 mb-4 pb-2 border-b">Tambah Ruangan Baru</h3>
        
        <div class="space-y-4">
          <Input 
            id="nama_ruang"
            label="Nama Ruangan" 
            placeholder="Contoh: Ruang A" 
            bind:value={newName}
            disabled={createLoading}
            theme="light"
          />

          <Input 
            id="username_ruang"
            label="Username Ruangan (Login)" 
            placeholder="Contoh: ruang_a" 
            bind:value={newUsername}
            disabled={createLoading}
            theme="light"
          />

          <PasswordGenerator 
            bind:value={newPassword} 
            length={12}
            label="Password Pengawas"
            placeholder="Klik Generate untuk password kuat"
            theme="light"
          />

          <Button 
            variant="primary" 
            size="sm" 
            theme="light"
            class="w-full" 
            on:click={createRoom}
            loading={createLoading}
          >
            Simpan Ruangan
          </Button>
        </div>
      </Card>
    </div>
  </div>

  <ConfirmModal
    show={showDeleteModal}
    title="Hapus Ruang Ujian"
    message={`Apakah Anda yakin ingin menghapus ruangan "${roomToDelete?.name || ''}"? Tindakan ini tidak dapat dibatalkan.`}
    confirmText="Hapus Ruangan"
    cancelText="Batal"
    variant="danger"
    loading={deleteLoading}
    on:confirm={confirmDeleteRoom}
    on:cancel={() => (showDeleteModal = false)}
  />
</div>
