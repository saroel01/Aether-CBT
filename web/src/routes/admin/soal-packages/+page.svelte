<script lang="ts">
  import { api, apiUrl, authHeaders } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte';
  import { toast } from '$lib/stores/toast';

  // Requirement 3.8/3.9/3.10: list, upload (with progress), delete unlinked packages.
  let items: any[] = [];
  let loading = true;
  let uploadLoading = false;
  let uploadProgress = 0;

  let selectedFile: File | null = null;
  let displayName = '';

  // Delete confirmation state
  let showDeleteModal = false;
  let pkgToDelete: { id: number; name: string } | null = null;
  let deleteLoading = false;

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
  }

  onMount(async () => {
    await loadPackages();
  });

  async function loadPackages() {
    loading = true;
    try {
      const res = await api('/admin/soal-packages');
      items = res.data || [];
    } catch {
      toast.error('Gagal mengambil daftar paket soal.');
    }
    loading = false;
  }

  function onFileChange(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0] ?? null;
    selectedFile = file;
    if (file && !displayName) {
      // Default the display name to the file stem (minus .zip), matching the backend.
      displayName = file.name.replace(/\.zip$/i, '');
    }
  }

  // Upload via raw fetch (not api()) because it is multipart/form-data, not JSON, and we
  // want XMLHttpRequest progress reporting. Uses apiUrl + authHeaders (Requirement 12.5:
  // no hardcoded URL/token).
  function uploadPackage() {
    if (!selectedFile) {
      toast.warning('Pilih berkas ZIP paket iSpring terlebih dahulu.');
      return;
    }
    if (!/\.zip$/i.test(selectedFile.name)) {
      toast.warning('Hanya berkas .zip yang diterima.');
      return;
    }

    uploadLoading = true;
    uploadProgress = 0;

    const formData = new FormData();
    formData.append('file', selectedFile);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', apiUrl('/admin/soal-packages/upload'));
    const headers = authHeaders();
    // Do NOT set Content-Type for FormData; the browser sets the multipart boundary.
    for (const [k, v] of Object.entries(headers)) {
      xhr.setRequestHeader(k, v);
    }

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        uploadProgress = Math.round((e.loaded / e.total) * 100);
      }
    };

    xhr.onload = () => {
      uploadLoading = false;
      let body: any = null;
      try { body = JSON.parse(xhr.responseText); } catch { /* non-JSON error page */ }
      if (xhr.status >= 200 && xhr.status < 300) {
        toast.success(`Paket "${displayName || selectedFile.name}" berhasil diunggah!`);
        selectedFile = null;
        displayName = '';
        uploadProgress = 0;
        loadPackages();
      } else {
        toast.error('Gagal mengunggah paket: ' + (body?.error || body?.message || `HTTP ${xhr.status}`));
      }
    };

    xhr.onerror = () => {
      uploadLoading = false;
      toast.error('Gangguan jaringan saat mengunggah paket. Coba lagi.');
    };

    xhr.send(formData);
  }

  function promptDeletePackage(id: number, nama: string) {
    pkgToDelete = { id, name: nama };
    showDeleteModal = true;
  }

  async function confirmDeletePackage() {
    if (!pkgToDelete) return;
    deleteLoading = true;
    try {
      await api(`/admin/soal-packages/${pkgToDelete.id}`, { method: 'DELETE' });
      toast.success(`Paket "${pkgToDelete.name}" dihapus.`);
      showDeleteModal = false;
      pkgToDelete = null;
      await loadPackages();
    } catch (e: any) {
      // 409 = linked to an exam; the backend message explains the reason (Requirement 3.10).
      toast.error('Gagal menghapus paket: ' + e.message);
    }
    deleteLoading = false;
  }
</script>

<svelte:head>
  <title>Paket Soal iSpring - Admin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <div class="border-b border-slate-200/60 pb-6">
    <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight font-display">Paket Soal iSpring</h1>
    <p class="text-slate-500 text-sm">Unggah arsip ekspor iSpring QuizMaker HTML5, kelola daftar paket per tenant, dan hapus paket yang tidak tertaut.</p>
  </div>

  <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">
    <!-- Package list (2/3) -->
    <div class="lg:col-span-2 flex flex-col gap-4">
      <h3 class="text-lg font-bold uppercase tracking-wider text-slate-500">Daftar Paket</h3>

      {#if loading}
        <div class="bg-white border rounded-2xl p-16 flex flex-col items-center justify-center gap-3">
          <svg class="animate-spin h-6 w-6 text-indigo-600" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <span class="text-xs font-semibold text-slate-500">Memuat paket soal...</span>
        </div>
      {:else}
        <Table>
          <thead>
            <tr>
              <th class="w-16">ID</th>
              <th>Nama Paket</th>
              <th class="w-28">Versi iSpring</th>
              <th class="w-24">Ukuran</th>
              <th class="w-36">Diunggah</th>
              <th class="text-center w-24">Aksi</th>
            </tr>
          </thead>
          <tbody>
            {#each items as p}
              <tr>
                <td class="font-mono text-slate-400 font-bold">{p.id}</td>
                <td class="font-semibold text-slate-800">
                  {p.nama}
                  <div class="text-[10px] font-mono text-slate-400">{p.package_uuid}</div>
                </td>
                <td>
                  {#if p.ispring_version}
                    <Badge>{p.ispring_version}</Badge>
                  {:else}
                    <span class="text-xs text-slate-400">tidak diketahui</span>
                  {/if}
                </td>
                <td class="font-mono text-xs text-slate-500">{formatBytes(p.total_size)}</td>
                <td class="text-xs text-slate-400 font-mono">
                  {new Date(p.created_at).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })}
                </td>
                <td class="text-center">
                  <Button variant="danger" size="sm" theme="light" on:click={() => promptDeletePackage(p.id, p.nama)}>
                    Hapus
                  </Button>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="6" class="text-center py-12 text-slate-400 font-medium">
                  Belum ada paket soal. Unggah arsip ZIP iSpring via panel kanan.
                </td>
              </tr>
            {/each}
          </tbody>
        </Table>
      {/if}
    </div>

    <!-- Upload panel (1/3) -->
    <div class="lg:col-span-1">
      <Card padding="md" class="border-slate-200/50 bg-white shadow-sm">
        <h3 class="text-base font-bold text-slate-800 mb-4 pb-2 border-b">Unggah Paket Baru</h3>

        <div class="space-y-4">
          <div class="flex flex-col gap-2">
            <label for="soal_file" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Berkas ZIP iSpring</label>
            <input
              id="soal_file"
              type="file"
              accept=".zip"
              class="block w-full text-xs text-slate-600 file:mr-3 file:py-2.5 file:px-4 file:rounded-2xl file:border-0 file:text-xs file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100 file:cursor-pointer file:transition-colors cursor-pointer"
              on:change={onFileChange}
              disabled={uploadLoading}
            />
            {#if selectedFile}
              <span class="text-[11px] text-slate-400">{selectedFile.name} ({formatBytes(selectedFile.size)})</span>
            {/if}
          </div>

          <Input
            id="soal_nama"
            label="Nama Tampil (opsional)"
            placeholder="Contoh: UAS Kimia XII 2025"
            bind:value={displayName}
            disabled={uploadLoading}
            theme="light"
          />

          {#if uploadLoading}
            <div class="space-y-1.5">
              <div class="flex items-center justify-between text-xs font-semibold text-slate-500">
                <span>Mengunggah...</span>
                <span class="font-mono text-blue-600">{uploadProgress}%</span>
              </div>
              <div class="w-full h-2 bg-slate-100 rounded-full overflow-hidden">
                <div class="h-full bg-blue-600 transition-all duration-200" style="width: {uploadProgress}%"></div>
              </div>
            </div>
          {/if}

          <Button variant="primary" size="sm" theme="light" class="w-full" on:click={uploadPackage} loading={uploadLoading} disabled={!selectedFile}>
            Unggah Paket
          </Button>

          <p class="text-[11px] text-slate-400 leading-relaxed pt-2 border-t">
            Pastikan arsip memuat <code class="font-mono text-blue-600">index.html</code> di akar.
            Paket disimpan terisolasi per tenant.
          </p>
        </div>
      </Card>
    </div>
  </div>

  <ConfirmModal
    show={showDeleteModal}
    title="Hapus Paket Soal"
    message={`Hapus paket soal "${pkgToDelete?.name || ''}"? Tindakan ini tidak dapat dibatalkan.`}
    confirmText="Hapus Paket"
    cancelText="Batal"
    variant="danger"
    loading={deleteLoading}
    on:confirm={confirmDeletePackage}
    on:cancel={() => (showDeleteModal = false)}
  />
</div>
