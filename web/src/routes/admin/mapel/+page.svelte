<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import { toast } from '$lib/stores/toast';

  let items: any[] = [];
  let newName = '';
  let newCode = '';
  let loading = true;
  let createLoading = false;

  onMount(async () => {
    await loadMapels();
  });

  async function loadMapels() {
    loading = true;
    try {
      const res = await api('/mapel');
      items = res.data || [];
    } catch {
      toast.error('Gagal mengambil data mata pelajaran.');
    }
    loading = false;
  }

  async function createMapel() {
    if (!newName || !newCode) {
      toast.warning('Harap lengkapi nama dan kode mata pelajaran!');
      return;
    }

    createLoading = true;
    try {
      await api('/mapel', {
        method: 'POST',
        body: JSON.stringify({ nama_mapel: newName, kode_mapel: newCode })
      });
      toast.success(`Mata Pelajaran "${newName}" berhasil terdaftar!`);
      newName = '';
      newCode = '';
      await loadMapels();
    } catch (e: any) {
      toast.error('Gagal menyimpan mata pelajaran: ' + e.message);
    }
    createLoading = false;
  }

  async function deleteMapel(id: number, name: string) {
    const confirm = window.confirm(`Apakah Anda yakin ingin menghapus mata pelajaran "${name}"?`);
    if (!confirm) return;
    try {
      await api(`/mapel/${id}`, { method: 'DELETE' });
      toast.success(`Mata Pelajaran "${name}" berhasil dihapus!`);
      await loadMapels();
    } catch (e: any) {
      toast.error('Gagal menghapus mata pelajaran: ' + e.message);
    }
  }
</script>

<svelte:head>
  <title>Kelola Mapel - Aether CBT</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="border-b pb-6">
    <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Mata Pelajaran Ujian</h1>
    <p class="text-slate-500 text-sm">Kelola daftar silabus mata pelajaran aktif di dalam sistem.</p>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">
    <!-- List of mapels (2/3) -->
    <div class="lg:col-span-2 flex flex-col gap-4">
      <h3 class="text-lg font-bold uppercase tracking-wider text-slate-500">Daftar Mata Pelajaran</h3>

      {#if loading}
        <div class="bg-white border rounded-2xl p-16 flex flex-col items-center justify-center gap-3">
          <svg class="animate-spin h-6 w-6 text-indigo-600" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span class="text-xs font-semibold text-slate-500">Memuat data mapel...</span>
        </div>
      {:else}
        <Table>
          <thead>
            <tr>
              <th class="w-20">ID</th>
              <th class="w-32">Kode Mapel</th>
              <th>Nama Mata Pelajaran</th>
              <th>Dibuat Pada</th>
              <th class="text-center w-28">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#each items as m}
              <tr>
                <td class="font-mono text-slate-400 font-bold">{m.id}</td>
                <td>
                  <span class="px-2 py-0.5 bg-slate-100 text-slate-700 font-mono font-bold text-xs border rounded">
                    {m.kode_mapel}
                  </span>
                </td>
                <td class="font-semibold text-slate-800">{m.nama_mapel}</td>
                <td class="text-xs text-slate-400 font-mono">
                  {new Date(m.created_at).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' })}
                </td>
                <td class="text-center">
                  <Button 
                    variant="danger" 
                    size="sm" 
                    class="bg-red-50 text-red-600 hover:bg-red-600 hover:text-white border-red-100 font-semibold"
                    on:click={() => deleteMapel(m.id, m.nama_mapel)}
                  >
                    Hapus
                  </Button>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="5" class="text-center py-12 text-slate-400 font-medium">
                  Belum ada mata pelajaran terdaftar. Gunakan panel kanan untuk menambah.
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
        <h3 class="text-base font-bold text-slate-800 mb-4 pb-2 border-b">Tambah Mapel Baru</h3>
        
        <div class="space-y-4">
          <Input 
            id="kode_mapel"
            label="Kode Mapel" 
            placeholder="Contoh: MTK" 
            bind:value={newCode}
            disabled={createLoading}
          />

          <Input 
            id="nama_mapel"
            label="Nama Mata Pelajaran" 
            placeholder="Contoh: Matematika" 
            bind:value={newName}
            disabled={createLoading}
          />

          <Button 
            variant="primary" 
            size="sm" 
            class="w-full bg-indigo-600 border-none hover:bg-indigo-700 font-semibold" 
            on:click={createMapel}
            loading={createLoading}
          >
            Simpan Mapel
          </Button>
        </div>
      </Card>
    </div>
  </div>
</div>
