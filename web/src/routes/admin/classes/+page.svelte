<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import { toast } from '$lib/stores/toast';

  let items: any[] = [];
  let allSubjects: any[] = [];
  let mappedSubjects: any[] = [];
  
  let newName = '';
  let selectedClass: any = null;
  let selectedSubjectToLink = 0;

  let loading = true;
  let createLoading = false;
  let mappingLoading = false;

  onMount(async () => {
    await loadInitialData();
  });

  async function loadInitialData() {
    loading = true;
    try {
      const [classRes, subjectRes] = await Promise.all([
        api('/classes').catch(() => ({ data: [] })),
        api('/mapel').catch(() => ({ data: [] }))
      ]);
      items = classRes.data || [];
      allSubjects = subjectRes.data || [];
    } catch {
      toast.error('Gagal mengambil data kelas.');
    }
    loading = false;
  }

  async function createClass() {
    if (!newName) {
      toast.warning('Harap masukkan nama kelas baru!');
      return;
    }
    
    createLoading = true;
    try {
      await api('/classes', { 
        method: 'POST', 
        body: JSON.stringify({ nama_kelas: newName }) 
      });
      toast.success(`Kelas "${newName}" berhasil ditambahkan!`);
      newName = '';
      await loadInitialData();
    } catch (e: any) {
      toast.error('Gagal menambahkan kelas: ' + e.message);
    }
    createLoading = false;
  }

  async function selectClass(c: any) {
    selectedClass = c;
    await loadMappedSubjects(c.id);
  }

  async function loadMappedSubjects(classId: number) {
    mappingLoading = true;
    try {
      const res = await api(`/admin/curriculum/class/${classId}`);
      if (res.success) {
        mappedSubjects = res.data || [];
      }
    } catch {
      toast.error('Gagal memuat pemetaan kurikulum.');
    }
    mappingLoading = false;
  }

  async function linkSubject() {
    if (selectedSubjectToLink <= 0 || !selectedClass) {
      toast.warning('Harap pilih mata pelajaran!');
      return;
    }

    mappingLoading = true;
    try {
      const res = await api('/admin/curriculum/link', {
        method: 'POST',
        body: JSON.stringify({
          kelas_id: selectedClass.id,
          mapel_id: selectedSubjectToLink
        })
      });

      if (res.success) {
        toast.success('Mata pelajaran berhasil dipetakan ke kelas ini!');
        selectedSubjectToLink = 0;
        await loadMappedSubjects(selectedClass.id);
      }
    } catch (e: any) {
      toast.error('Gagal memetakan mata pelajaran: ' + e.message);
    }
    mappingLoading = false;
  }

  async function unlinkSubject(mapelId: number) {
    if (!selectedClass) return;
    
    mappingLoading = true;
    try {
      const res = await api('/admin/curriculum/unlink', {
        method: 'POST',
        body: JSON.stringify({
          kelas_id: selectedClass.id,
          mapel_id: mapelId
        })
      });

      if (res.success) {
        toast.success('Mata pelajaran berhasil dilepas dari kelas ini.');
        await loadMappedSubjects(selectedClass.id);
      }
    } catch (e: any) {
      toast.error('Gagal melepas mata pelajaran: ' + e.message);
    }
    mappingLoading = false;
  }

  async function deleteClass(id: number, className: string) {
    const confirm = window.confirm(`Apakah Anda yakin ingin menghapus kelas "${className}"?`);
    if (!confirm) return;

    try {
      const res = await api(`/classes/${id}`, { method: 'DELETE' });
      if (res.success) {
        toast.success(`Kelas "${className}" berhasil dihapus.`);
        if (selectedClass?.id === id) selectedClass = null;
        await loadInitialData();
      }
    } catch (e: any) {
      toast.error('Gagal menghapus kelas: ' + e.message);
    }
  }
</script>

<svelte:head>
  <title>Kurikulum & Kelas - Admin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="border-b pb-6">
    <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Kelas & Kurikulum</h1>
    <p class="text-slate-500 text-sm">Kelola daftar tingkatan kelas dan petakan mata pelajaran aktif untuk masing-masing kelas.</p>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">
    <!-- List of classes (2/3) -->
    <div class="lg:col-span-2 flex flex-col gap-4">
      <h3 class="text-lg font-bold uppercase tracking-wider text-slate-500">Daftar Kelas</h3>

      {#if loading}
        <div class="bg-white border rounded-2xl p-16 flex flex-col items-center justify-center gap-3">
          <svg class="animate-spin h-6 w-6 text-indigo-600" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span class="text-xs font-semibold text-slate-500">Memuat data kelas...</span>
        </div>
      {:else}
        <Table>
          <thead>
            <tr>
              <th class="w-20">ID</th>
              <th>Nama Kelas</th>
              <th class="text-center w-24">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#each items as c}
              {@const isSelected = selectedClass?.id === c.id}
              <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
              <tr 
                class="cursor-pointer transition-colors duration-150 {isSelected ? 'bg-indigo-50/40 border-l-4 border-l-indigo-600' : ''}"
                on:click={() => selectClass(c)}
              >
                <td class="font-mono text-slate-400 font-bold">{c.id}</td>
                <td class="font-semibold text-slate-800">{c.nama_kelas}</td>
                <td class="text-center">
                  <button 
                    type="button"
                    class="text-red-500 hover:text-red-700 font-semibold text-xs p-1.5 hover:bg-red-50 rounded"
                    on:click|stopPropagation={() => deleteClass(c.id, c.nama_kelas)}
                  >
                    Hapus
                  </button>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="3" class="text-center py-12 text-slate-400 font-medium">
                  Belum ada kelas terdaftar. Gunakan panel kanan untuk menambah.
                </td>
              </tr>
            {/each}
          </tbody>
        </Table>
      {/if}
    </div>

    <!-- Right Panels (1/3) -->
    <div class="lg:col-span-1 flex flex-col gap-6">
      <!-- Create Class Panel -->
      <Card padding="md" class="border-slate-200/50 bg-white shadow-sm">
        <h3 class="text-base font-bold text-slate-800 mb-4 pb-2 border-b">Tambah Kelas Baru</h3>
        
        <div class="space-y-4">
          <Input 
            id="nama_kelas"
            label="Nama Kelas" 
            placeholder="Contoh: XII IPA 1" 
            bind:value={newName}
            disabled={createLoading}
          />

          <Button 
            variant="primary" 
            size="sm" 
            class="w-full bg-indigo-600 border-none hover:bg-indigo-700 font-semibold" 
            on:click={createClass}
            loading={createLoading}
          >
            Simpan Kelas
          </Button>
        </div>
      </Card>

      <!-- Mapping Curriculum Panel (Only when selected) -->
      {#if selectedClass}
        <Card padding="md" class="border-slate-200/50 bg-white shadow-sm">
          <div slot="header">
            <h3 class="text-base font-bold text-slate-800">Pemetaan Soal: {selectedClass.nama_kelas}</h3>
            <p class="text-xs text-slate-400">Petakan mata pelajaran aktif untuk kelas ini.</p>
          </div>

          {#if mappingLoading}
            <div class="py-6 flex items-center justify-center gap-2">
              <svg class="animate-spin h-5 w-5 text-indigo-600" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <span class="text-xs text-slate-400 font-medium">Memproses...</span>
            </div>
          {:else}
            <!-- Subject Mapped List -->
            <div class="space-y-3.5 max-h-48 overflow-y-auto mb-4 border-b pb-4">
              {#each mappedSubjects as sub}
                <div class="flex items-center justify-between bg-slate-50 border p-2.5 rounded-xl text-sm">
                  <div class="flex flex-col">
                    <span class="font-bold text-slate-800">{sub.nama_mapel}</span>
                    <span class="text-[10px] text-slate-400 font-mono">{sub.kode_mapel}</span>
                  </div>
                  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
                  <button 
                    type="button"
                    class="text-red-500 hover:text-red-700 font-bold p-1"
                    on:click={() => unlinkSubject(sub.id)}
                    title="Lepas Pemetaan"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </div>
              {:else}
                <p class="text-xs text-slate-400 text-center py-4">Belum ada mata pelajaran dipetakan.</p>
              {/each}
            </div>

            <!-- Subject Connector Form -->
            <div class="flex flex-col gap-2 pt-2">
              <label for="link_subject_select" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Petakan Baru</label>
              <div class="flex gap-2">
                <select id="link_subject_select" bind:value={selectedSubjectToLink} class="w-full h-11 px-4 border rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-indigo-500 bg-white text-sm">
                  <option value={0}>Pilih Mata Pelajaran...</option>
                  {#each allSubjects as sub}
                    <!-- Hide if already linked -->
                    {#if !mappedSubjects.some(m => m.id === sub.id)}
                      <option value={sub.id}>{sub.nama_mapel} ({sub.kode_mapel})</option>
                    {/if}
                  {/each}
                </select>
                <Button variant="primary" size="sm" class="bg-indigo-600 border-none font-bold" on:click={linkSubject}>
                  Petakan
                </Button>
              </div>
            </div>
          {/if}
        </Card>
      {/if}
    </div>
  </div>
</div>
