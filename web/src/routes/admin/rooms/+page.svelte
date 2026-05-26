<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import PasswordGenerator from '$lib/components/PasswordGenerator.svelte';
  import { toast } from '$lib/stores/toast';

  let items: any[] = [];
  let newName = '';
  let newUsername = '';
  let newPassword = '';
  let loading = true;
  let createLoading = false;

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

  async function deleteRoom(id: number, name: string) {
    const confirm = window.confirm(`Apakah Anda yakin ingin menghapus ruangan "${name}"?`);
    if (!confirm) return;
    try {
      await api(`/rooms/${id}`, { method: 'DELETE' });
      toast.success(`Ruang Ujian "${name}" berhasil dihapus!`);
      await loadRooms();
    } catch (e: any) {
      toast.error('Gagal menghapus ruangan: ' + e.message);
    }
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
      <h3 class="text-lg font-bold uppercase tracking-wider text-slate-500">Daftar Ruangan</h3>

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
            {#each items as r}
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
                      on:click={() => deleteRoom(r.id, r.nama_ruang)}
                    >
                      Hapus
                    </Button>
                  </div>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="6" class="text-center py-12 text-slate-400 font-medium">
                  Belum ada ruangan terdaftar. Gunakan panel kanan untuk menambah.
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
</div>
