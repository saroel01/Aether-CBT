<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte';
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

  // Delete confirmation state
  let showDeleteModal = false;
  let classToDelete: { id: number; name: string } | null = null;
  let deleteLoading = false;

  // Tingkat (grade level) inline-editing state (Requirement 1.1, 1.4).
  const TINGKAT_OPTIONS = ['X', 'XI', 'XII'];
  let editingTingkat: Record<number, string> = {};
  let tingkatLoading: Record<number, boolean> = {};

  onMount(async () => {
    await loadInitialData();
  });

  async function updateTingkat(kelasId: number, namaKelas: string) {
    const tingkat = editingTingkat[kelasId] ?? '';
    tingkatLoading[kelasId] = true;
    try {
      await api(`/classes/${kelasId}/tingkat`, {
        method: 'PUT',
        body: JSON.stringify({ tingkat })
      });
      toast.success(`Tingkat kelas "${namaKelas}" diperbarui menjadi "${tingkat || '—'}".`);
      await loadInitialData();
    } catch (e: any) {
      toast.error('Gagal memperbarui tingkat: ' + e.message);
    }
    tingkatLoading[kelasId] = false;
  }

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

  function promptDeleteClass(id: number, className: string) {
    classToDelete = { id, name: className };
    showDeleteModal = true;
  }

  async function confirmDeleteClass() {
    if (!classToDelete) return;
    deleteLoading = true;
    try {
      const res = await api(`/classes/${classToDelete.id}`, { method: 'DELETE' });
      if (res.success) {
        toast.success(`Kelas "${classToDelete.name}" berhasil dihapus.`);
        if (selectedClass?.id === classToDelete.id) selectedClass = null;
        showDeleteModal = false;
        classToDelete = null;
        await loadInitialData();
      }
    } catch (e: any) {
      toast.error('Gagal menghapus kelas: ' + e.message);
    }
    deleteLoading = false;
  }
</script>

<svelte:head>
  <title>Kurikulum & Kelas - Admin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="border-b border-slate-200 pb-6">
    <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight font-display">Kelas & Kurikulum</h1>
    <p class="text-slate-500 text-sm mt-1">Kelola daftar tingkatan kelas dan petakan mata pelajaran aktif untuk masing-masing kelas.</p>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">
    <!-- List of classes (2/3) -->
    <div class="lg:col-span-2 flex flex-col gap-4">
      <h3 class="text-sm font-bold uppercase tracking-wider text-slate-500 font-mono">Daftar Kelas</h3>

      {#if loading}
        <div class="bg-white border border-slate-200 rounded-2xl p-16 flex flex-col items-center justify-center gap-3 shadow-sm">
          <svg class="animate-spin h-6 w-6 text-cobalt-600" fill="none" viewBox="0 0 24 24">
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
              <th class="w-40">Tingkat</th>
              <th class="text-center w-24">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#each items as c}
              {@const isSelected = selectedClass?.id === c.id}
              <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
              <tr
                class="cursor-pointer transition-colors duration-150 {isSelected ? 'bg-cobalt-50 text-cobalt-900 font-semibold ring-1 ring-cobalt-200 shadow-sm' : 'hover:bg-slate-50/50'}"
                on:click={() => selectClass(c)}
              >
                <td class="font-mono text-slate-500 font-bold tabular-nums">{c.id}</td>
                <td class="font-semibold text-slate-800">{c.nama_kelas}</td>
                <td on:click|stopPropagation>
                  <div class="flex items-center gap-2">
                    <select
                      class="h-9 px-3 border border-slate-200 rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-cobalt-500/20 focus:border-cobalt-600 bg-white transition-colors duration-150 text-slate-800 text-xs font-semibold"
                      value={c.tingkat ?? ''}
                      on:change={(e) => { editingTingkat[c.id] = e.currentTarget.value; updateTingkat(c.id, c.nama_kelas); }}
                      disabled={tingkatLoading[c.id]}
                    >
                      <option value="">— belum ditetapkan —</option>
                      {#each TINGKAT_OPTIONS as t}
                        <option value={t}>{t}</option>
                      {/each}
                    </select>
                    {#if tingkatLoading[c.id]}
                      <svg class="animate-spin h-4 w-4 text-cobalt-600 shrink-0" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                      </svg>
                    {/if}
                  </div>
                </td>
                <td class="text-center">
                  <Button
                    variant="danger"
                    size="sm"
                    theme="light"
                    on:click={(e) => { e.stopPropagation(); promptDeleteClass(c.id, c.nama_kelas); }}
                  >
                    Hapus
                  </Button>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="4" class="text-center py-12 text-slate-400 font-medium">
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
      <Card padding="md" class="border-slate-200 bg-white shadow-sm rounded-2xl">
        <h3 class="text-base font-bold text-slate-800 mb-4 pb-2 border-b border-slate-100 font-display">Tambah Kelas Baru</h3>
        
        <div class="space-y-4">
          <Input 
            id="nama_kelas"
            label="Nama Kelas" 
            placeholder="Contoh: XII IPA 1" 
            bind:value={newName}
            disabled={createLoading}
            theme="light"
          />

          <Button 
            variant="primary" 
            size="sm" 
            theme="light"
            class="w-full shadow-sm" 
            on:click={createClass}
            loading={createLoading}
          >
            Simpan Kelas
          </Button>
        </div>
      </Card>

      <!-- Mapping Curriculum Panel (Only when selected) -->
      {#if selectedClass}
        <Card padding="md" class="border-slate-200 bg-white shadow-sm rounded-2xl">
          <div slot="header">
            <h3 class="text-base font-bold text-slate-800 font-display">Pemetaan Soal: {selectedClass.nama_kelas}</h3>
            <p class="text-xs text-slate-500 mt-1">Petakan mata pelajaran aktif untuk kelas ini.</p>
          </div>

          {#if mappingLoading}
            <div class="py-6 flex items-center justify-center gap-2">
              <svg class="animate-spin h-5 w-5 text-cobalt-600" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <span class="text-xs text-slate-500 font-medium">Memproses...</span>
            </div>
          {:else}
            <!-- Subject Mapped List -->
            <div class="space-y-3 max-h-48 overflow-y-auto mb-4 border-b border-slate-100 pb-4">
              {#each mappedSubjects as sub}
                <div class="flex items-center justify-between bg-slate-50 border border-slate-200 p-2.5 rounded-xl text-sm">
                  <div class="flex flex-col">
                    <span class="font-bold text-slate-800">{sub.nama_mapel}</span>
                    <span class="text-[10px] text-slate-500 font-mono">{sub.kode_mapel}</span>
                  </div>
                  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
                  <button 
                    type="button"
                    class="text-ruby-500 hover:text-ruby-700 font-bold p-1 rounded-lg hover:bg-ruby-50 transition-colors"
                    on:click={() => unlinkSubject(sub.id)}
                    title="Lepas Pemetaan"
                    aria-label="Lepas Pemetaan Mata Pelajaran"
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
                <select id="link_subject_select" bind:value={selectedSubjectToLink} class="w-full h-11 px-4 border border-slate-200 rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-cobalt-500/20 focus:border-cobalt-600 bg-white transition-colors duration-150 text-slate-800 text-sm font-semibold">
                  <option value={0}>Pilih Mata Pelajaran...</option>
                  {#each allSubjects as sub}
                    <!-- Hide if already linked -->
                    {#if !mappedSubjects.some(m => m.id === sub.id)}
                      <option value={sub.id}>{sub.nama_mapel} ({sub.kode_mapel})</option>
                    {/if}
                  {/each}
                </select>
                <Button variant="primary" size="sm" theme="light" class="shadow-sm" on:click={linkSubject}>
                  Petakan
                </Button>
              </div>
            </div>
          {/if}
        </Card>
      {/if}
    </div>
  </div>

  <!-- Delete Class Confirm Modal -->
  <ConfirmModal 
    bind:isOpen={showDeleteModal}
    title="Hapus Kelas"
    message={`Apakah Anda yakin ingin menghapus kelas "${classToDelete?.name || ''}"? Tindakan ini tidak dapat dibatalkan.`}
    confirmLabel="Hapus Kelas"
    cancelLabel="Batal"
    variant="danger"
    loading={deleteLoading}
    on:confirm={confirmDeleteClass}
    on:cancel={() => { showDeleteModal = false; classToDelete = null; }}
  />
</div>
