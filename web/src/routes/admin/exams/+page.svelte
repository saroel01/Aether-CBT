<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import { toast } from '$lib/stores/toast';

  // Requirement 2.1-2.6: exam definition CRUD + link soal package.
  let items: any[] = [];
  let mapelList: any[] = [];
  let packageList: any[] = [];

  let loading = true;
  let modalOpen = false;
  let editingId: number | null = null;
  let saving = false;

  // Form state.
  let fMapelID = 0;
  let fTingkat = '';
  let fPackageID = 0; // 0 = none
  let fDurasi = 90;
  let fKKM = 0;
  let fShuffleQ = false;
  let fShuffleA = false;
  let fNama = '';

  function resetForm() {
    editingId = null;
    fMapelID = 0;
    fTingkat = '';
    fPackageID = 0;
    fDurasi = 90;
    fKKM = 0;
    fShuffleQ = false;
    fShuffleA = false;
    fNama = '';
  }

  onMount(async () => {
    await loadAll();
  });

  async function loadAll() {
    loading = true;
    try {
      const [examRes, mapelRes, pkgRes] = await Promise.all([
        api('/admin/exams').catch(() => ({ data: [] })),
        api('/mapel').catch(() => ({ data: [] })),
        api('/admin/soal-packages').catch(() => ({ data: [] }))
      ]);
      items = examRes.data || [];
      mapelList = mapelRes.data || [];
      packageList = pkgRes.data || [];
    } catch {
      toast.error('Gagal memuat data ujian.');
    }
    loading = false;
  }

  function openCreate() {
    resetForm();
    modalOpen = true;
  }

  function openEdit(e: any) {
    editingId = e.id;
    fMapelID = e.mapel_id;
    fTingkat = e.tingkat ?? '';
    fPackageID = e.soal_package_id ?? 0;
    fDurasi = e.durasi_menit ?? 90;
    fKKM = e.kkm ?? 0;
    fShuffleQ = !!e.shuffle_questions;
    fShuffleA = !!e.shuffle_answers;
    fNama = e.nama ?? '';
    modalOpen = true;
  }

  async function saveExam() {
    if (!fMapelID) {
      toast.warning('Pilih mata pelajaran terlebih dahulu.');
      return;
    }
    const payload = {
      mapel_id: fMapelID,
      tingkat: fTingkat || null,
      soal_package_id: fPackageID > 0 ? fPackageID : null,
      durasi_menit: Number(fDurasi) || 90,
      kkm: Number(fKKM) || 0,
      shuffle_questions: fShuffleQ,
      shuffle_answers: fShuffleA,
      nama: fNama || null
    };
    saving = true;
    try {
      if (editingId) {
        await api(`/admin/exams/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) });
        toast.success('Definisi ujian diperbarui.');
      } else {
        await api('/admin/exams', { method: 'POST', body: JSON.stringify(payload) });
        toast.success('Definisi ujian dibuat.');
      }
      modalOpen = false;
      resetForm();
      await loadAll();
    } catch (e: any) {
      // 400 = invalid mapel/package reference (Requirement 2.2, 2.3); backend explains.
      toast.error('Gagal menyimpan ujian: ' + e.message);
    }
    saving = false;
  }

  async function deleteExam(id: number, nama: string) {
    const confirm = window.confirm(`Hapus ujian "${nama}"?`);
    if (!confirm) return;
    try {
      await api(`/admin/exams/${id}`, { method: 'DELETE' });
      toast.success(`Ujian "${nama}" dihapus.`);
      await loadAll();
    } catch (e: any) {
      // 409 = has scheduled/active session (Requirement 2.5).
      toast.error('Gagal menghapus ujian: ' + e.message);
    }
  }

  function mapelName(id: number): string {
    return mapelList.find((m) => m.id === id)?.nama_mapel ?? '—';
  }
  function packageName(id: number | null | undefined): string {
    if (!id) return '— belum ditaut —';
    return packageList.find((p) => p.id === id)?.nama ?? '—';
  }
</script>

<svelte:head>
  <title>Definisi Ujian - Admin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <div class="border-b pb-6 flex items-end justify-between gap-4">
    <div>
      <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Definisi Ujian</h1>
      <p class="text-slate-500 text-sm">Kombinasi mapel + tingkat + paket soal + durasi + KKM yang dapat dijadwalkan ulang lewat sesi.</p>
    </div>
    <Button variant="primary" theme="light" on:click={openCreate}>+ Buat Ujian</Button>
  </div>

  {#if loading}
    <div class="bg-white border rounded-2xl p-16 flex flex-col items-center justify-center gap-3">
      <svg class="animate-spin h-6 w-6 text-indigo-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
      </svg>
      <span class="text-xs font-semibold text-slate-500">Memuat data ujian...</span>
    </div>
  {:else}
    <Table>
      <thead>
        <tr>
          <th class="w-16">ID</th>
          <th>Nama</th>
          <th>Mapel</th>
          <th class="w-20">Tingkat</th>
          <th>Paket Soal</th>
          <th class="w-20">Durasi</th>
          <th class="w-20">KKM</th>
          <th class="text-center w-28">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#each items as e}
          <tr>
            <td class="font-mono text-slate-400 font-bold">{e.id}</td>
            <td class="font-semibold text-slate-800">{e.nama || '—'}</td>
            <td class="text-slate-600">{mapelName(e.mapel_id)}</td>
            <td>
              {#if e.tingkat}
                <Badge>{e.tingkat}</Badge>
              {:else}
                <span class="text-xs text-slate-400">—</span>
              {/if}
            </td>
            <td>
              {#if e.soal_package_id}
                <span class="text-slate-600">{packageName(e.soal_package_id)}</span>
              {:else}
                <span class="text-xs text-amber-600 font-semibold">belum ditaut</span>
              {/if}
            </td>
            <td class="font-mono text-xs text-slate-500">{e.durasi_menit}m</td>
            <td class="font-mono text-xs text-slate-500">{e.kkm}</td>
            <td class="text-center">
              <div class="flex items-center justify-center gap-2">
                <Button variant="secondary" size="sm" theme="light" on:click={() => openEdit(e)}>Sunting</Button>
                <Button variant="danger" size="sm" theme="light" on:click={() => deleteExam(e.id, e.nama || `#${e.id}`)}>Hapus</Button>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="8" class="text-center py-12 text-slate-400 font-medium">
              Belum ada definisi ujian. Klik "+ Buat Ujian" untuk membuat.
            </td>
          </tr>
        {/each}
      </tbody>
    </Table>
  {/if}
</div>

<Modal bind:show={modalOpen} title={editingId ? 'Sunting Ujian' : 'Buat Ujian'} size="lg" theme="light">
  <div class="space-y-4">
    <Input id="exam_nama" label="Nama Ujian (opsional)" placeholder="Contoh: UAS Kimia XII" bind:value={fNama} disabled={saving} theme="light" />

    <div class="flex flex-col gap-2">
      <label for="exam_mapel" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Mata Pelajaran *</label>
      <select id="exam_mapel" bind:value={fMapelID} disabled={saving}
        class="w-full h-12 px-4 border border-slate-200 rounded-2xl outline-none hover:border-slate-300 focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-white transition-all duration-300 text-slate-800 text-sm font-semibold">
        <option value={0}>Pilih Mapel...</option>
        {#each mapelList as m}
          <option value={m.id}>{m.nama_mapel} ({m.kode_mapel || '—'})</option>
        {/each}
      </select>
    </div>

    <div class="grid grid-cols-2 gap-4">
      <div class="flex flex-col gap-2">
        <label for="exam_tingkat" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Tingkat</label>
        <select id="exam_tingkat" bind:value={fTingkat} disabled={saving}
          class="w-full h-12 px-4 border border-slate-200 rounded-2xl outline-none hover:border-slate-300 focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-white transition-all duration-300 text-slate-800 text-sm font-semibold">
          <option value="">—</option>
          <option value="X">X</option>
          <option value="XI">XI</option>
          <option value="XII">XII</option>
        </select>
      </div>
      <div class="flex flex-col gap-2">
        <label for="exam_pkg" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Paket Soal</label>
        <select id="exam_pkg" bind:value={fPackageID} disabled={saving}
          class="w-full h-12 px-4 border border-slate-200 rounded-2xl outline-none hover:border-slate-300 focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-white transition-all duration-300 text-slate-800 text-sm font-semibold">
          <option value={0}>— belum ditaut (draft) —</option>
          {#each packageList as p}
            <option value={p.id}>{p.nama}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="grid grid-cols-2 gap-4">
      <Input id="exam_durasi" type="number" label="Durasi (menit)" bind:value={fDurasi} disabled={saving} theme="light" />
      <Input id="exam_kkm" type="number" label="KKM" bind:value={fKKM} disabled={saving} theme="light" />
    </div>

    <div class="flex items-center gap-6 pt-2">
      <label class="flex items-center gap-2 cursor-pointer">
        <input type="checkbox" bind:checked={fShuffleQ} disabled={saving} class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600/20" />
        <span class="text-sm font-semibold text-slate-600">Acak Soal</span>
      </label>
      <label class="flex items-center gap-2 cursor-pointer">
        <input type="checkbox" bind:checked={fShuffleA} disabled={saving} class="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-600/20" />
        <span class="text-sm font-semibold text-slate-600">Acak Jawaban</span>
      </label>
    </div>
  </div>

  <div slot="footer" class="flex items-center justify-end gap-3 w-full">
    <Button variant="ghost" size="sm" theme="light" on:click={() => { modalOpen = false; resetForm(); }} disabled={saving}>Batal</Button>
    <Button variant="primary" size="sm" theme="light" on:click={saveExam} loading={saving}>{editingId ? 'Simpan Perubahan' : 'Buat Ujian'}</Button>
  </div>
</Modal>
