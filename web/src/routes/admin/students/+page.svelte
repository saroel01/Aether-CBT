<script lang="ts">
  import { api, apiUrl, authHeaders } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import PasswordGenerator from '$lib/components/PasswordGenerator.svelte';
  import { toast } from '$lib/stores/toast';

  let students: any[] = [];
  let classesList: any[] = [];
  let roomsList: any[] = [];
  let searchQuery = '';

  $: filteredStudents = students.filter(s => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase().trim();
    const idMatch = (s.no_id || '').toLowerCase().includes(q);
    const nameMatch = (s.nama_peserta || '').toLowerCase().includes(q);
    const classMatch = getClassName(s.kelas_id).toLowerCase().includes(q);
    const roomMatch = getRoomName(s.ruang_id).toLowerCase().includes(q);
    return idMatch || nameMatch || classMatch || roomMatch;
  });
  
  let loading = true;
  let error = '';
  
  // Modal toggles
  let showAddModal = false;
  let showCSVModal = false;
  let csvFile: File | null = null;
  let importLoading = false;

  // Delete confirmation state
  let showDeleteModal = false;
  let studentToDelete: { id: number; name: string } | null = null;
  let deleteLoading = false;

  // Create form state
  let newNoID = '';
  let newNama = '';
  let newKelas = 1;
  let newRuang = 1;
  let newPass = '';
  let newJK: 'L' | 'P' = 'L';
  let createLoading = false;

  onMount(async () => {
    await loadInitialData();
  });

  async function loadInitialData() {
    loading = true;
    try {
      const [studentsRes, classesRes, roomsRes] = await Promise.all([
        api('/students').catch(() => ({ data: [] })),
        api('/classes').catch(() => ({ data: [] })),
        api('/rooms').catch(() => ({ data: [] }))
      ]);
      
      students = studentsRes.data || [];
      classesList = classesRes.data || [];
      roomsList = roomsRes.data || [];
    } catch (e: any) {
      error = e.message;
      toast.error('Gagal memuat data awal: ' + error);
    }
    loading = false;
  }

  async function createStudent() {
    if (!newNoID || !newNama || !newKelas || !newRuang) {
      toast.warning('Harap lengkapi seluruh kolom wajib!');
      return;
    }

    createLoading = true;
    try {
      await api('/students', {
        method: 'POST',
        body: JSON.stringify({
          no_id: newNoID,
          password: newPass,
          nama_peserta: newNama,
          kelas_id: newKelas,
          ruang_id: newRuang,
          jenis_kelamin: newJK
        })
      });
      
      toast.success(`Siswa "${newNama}" berhasil terdaftar!`);
      showAddModal = false;
      
      // Clear form
      newNoID = ''; newNama = ''; newPass = '';
      
      // reload
      await loadInitialData();
    } catch (e: any) {
      toast.error('Gagal menyimpan siswa: ' + e.message);
    }
    createLoading = false;
  }

  // Handle CSV File Change
  function handleCSVChange(event: Event) {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files[0]) {
      csvFile = input.files[0];
    }
  }

  // Upload student records CSV sheet to backend Go endpoint
  async function uploadCSV() {
    if (!csvFile) {
      toast.warning('Harap pilih berkas CSV terlebih dahulu!');
      return;
    }

    importLoading = true;
    try {
      const formData = new FormData();
      formData.append('file', csvFile);

      const res = await fetch(apiUrl('/admin/students/import-csv'), {
        method: 'POST',
        headers: authHeaders(),
        body: formData
      });

      const data = await res.json();

      if (res.ok && data.success) {
        toast.success(`Impor sukses! Berhasil mengimpor ${data.data.success_count} siswa.`);
        if (data.data.error_count > 0) {
          toast.warning(`${data.data.error_count} baris CSV gagal diproses karena data tidak valid.`);
        }
        showCSVModal = false;
        csvFile = null;
        await loadInitialData();
      } else {
        throw new Error(data.error || 'Terjadi kesalahan sistem');
      }
    } catch (e: any) {
      toast.error('Gagal mengimpor CSV: ' + e.message);
    }
    importLoading = false;
  }

  // Helper mapping names
  function getClassName(id: number): string {
    const found = classesList.find(c => c.id === id);
    return found ? found.nama_kelas : `ID: ${id}`;
  }

  // Set first class/room values as default once lists load!
  $: if (classesList.length > 0 && !newKelas) {
    newKelas = classesList[0].id;
  }
  $: if (roomsList.length > 0 && !newRuang) {
    newRuang = roomsList[0].id;
  }

  function getRoomName(id: number): string {
    const found = roomsList.find(r => r.id === id);
    return found ? found.nama_ruang : `ID: ${id}`;
  }

  function promptDeleteStudent(id: number, name: string) {
    studentToDelete = { id, name };
    showDeleteModal = true;
  }

  async function confirmDeleteStudent() {
    if (!studentToDelete) return;
    deleteLoading = true;
    try {
      await api(`/students/${studentToDelete.id}`, { method: 'DELETE' });
      toast.success(`Siswa "${studentToDelete.name}" berhasil dihapus!`);
      showDeleteModal = false;
      studentToDelete = null;
      await loadInitialData();
    } catch (e: any) {
      toast.error('Gagal menghapus siswa: ' + e.message);
    }
    deleteLoading = false;
  }
</script>

<svelte:head>
  <title>Manajemen Peserta - Aether CBT</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-slate-200/60 pb-6">
    <div>
      <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight font-display">Peserta Ujian (Siswa)</h1>
      <p class="text-slate-500 text-sm">Kelola daftar registrasi peserta ujian resmi di sekolah.</p>
    </div>

    <!-- Header Action Buttons with theme="light" -->
    <div class="flex items-center gap-3">
      <a href="/admin/students/print-cards" target="_blank">
        <Button variant="secondary" size="sm" theme="light" class="font-semibold shadow-sm">
          Cetak Kartu Ujian
        </Button>
      </a>
      <Button variant="secondary" size="sm" theme="light" class="font-semibold shadow-sm" on:click={() => showCSVModal = true}>
        Impor CSV Massal
      </Button>
      <Button variant="primary" size="sm" theme="light" class="font-semibold shadow-sm" on:click={() => showAddModal = true}>
        Tambah Siswa
      </Button>
    </div>
  </div>

  {#if loading}
    <div class="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
      <svg class="animate-spin h-8 w-8 text-cobalt-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
      <p class="text-sm font-semibold text-slate-500">Memuat daftar siswa...</p>
    </div>
  {:else if error}
    <div class="p-6 bg-ruby-50 border border-ruby-200 rounded-2xl text-center text-ruby-600 font-medium">
      Gagal mengambil data siswa: {error}
    </div>
  {:else}
    <!-- Instant Search & Quick Stats Toolbar -->
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-white p-3 rounded-2xl border border-slate-200/80 shadow-sm">
      <div class="relative flex-1 max-w-md">
        <Input 
          type="search"
          placeholder="Cari nama peserta, nomor ID/NIS, kelas, atau ruang..." 
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

      <div class="flex items-center gap-2 text-xs text-slate-500 font-mono px-2">
        {#if searchQuery}
          <span>Menampilkan <strong class="text-slate-800 tabular-nums">{filteredStudents.length}</strong> dari <span class="tabular-nums">{students.length}</span> siswa</span>
        {:else}
          <span>Total: <strong class="text-slate-800 tabular-nums">{students.length}</strong> siswa terdaftar</span>
        {/if}
      </div>
    </div>

    <Table>
      <thead>
        <tr class="font-display">
          <th>No. ID</th>
          <th>Nama Lengkap</th>
          <th>Jenis Kelamin</th>
          <th>Kelas</th>
          <th>Ruang Ujian</th>
          <th>Sandi Default</th>
          <th class="text-center">Aksi</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredStudents as s}
          <tr class="hover:bg-slate-50/50 transition-colors">
            <!-- No ID using monospace bold tabular-nums -->
            <td class="font-mono font-bold text-slate-600 tabular-nums">{s.no_id}</td>
            <td class="font-semibold text-slate-800">{s.nama_peserta}</td>
            <td>
              {#if s.jenis_kelamin === 'L'}
                <Badge variant="info" theme="light">Laki-laki</Badge>
              {:else if s.jenis_kelamin === 'P'}
                <Badge variant="warning" theme="light">Perempuan</Badge>
              {:else}
                <Badge variant="neutral" theme="light">—</Badge>
              {/if}
            </td>
            <td class="font-semibold text-slate-700">{getClassName(s.kelas_id)}</td>
            <td class="font-semibold text-slate-700">{getRoomName(s.ruang_id)}</td>
            <td class="font-mono text-slate-400 text-xs">{s.password ? '••••••' : '—'}</td>
            <td class="text-center">
              <Button 
                variant="danger" 
                size="sm" 
                theme="light"
                on:click={() => promptDeleteStudent(s.id, s.nama_peserta)}
              >
                Hapus
              </Button>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="7" class="py-8">
              {#if searchQuery}
                <EmptyState
                  title="Siswa Tidak Ditemukan"
                  description={`Tidak ditemukan siswa dengan kata kunci "${searchQuery}". Coba periksa ejaan nomor ID atau nama siswa.`}
                  icon="search"
                  actionText="Bersihkan Pencarian"
                  actionVariant="secondary"
                  on:action={() => searchQuery = ''}
                />
              {:else}
                <EmptyState
                  title="Belum Ada Siswa Terdaftar"
                  description="Belum ada data siswa peserta ujian resmi. Klik tombol di bawah untuk menambah siswa baru atau impor berkas CSV."
                  icon="users"
                  actionText="Tambah Siswa Baru"
                  actionVariant="primary"
                  on:action={() => showAddModal = true}
                />
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </Table>
  {/if}

  <!-- Manual Add Student Modal (Clean Light Inputs) -->
  <Modal show={showAddModal} title="Tambah Siswa Baru" size="md">
    <div class="space-y-4 text-slate-800 p-1">
      <Input 
        id="no_id"
        label="Nomor ID / Nomor Peserta *" 
        placeholder="Contoh: 2024009" 
        bind:value={newNoID}
        theme="light"
      />
      
      <Input 
        id="nama"
        label="Nama Lengkap *" 
        placeholder="Contoh: Muhammad Rian" 
        bind:value={newNama}
        theme="light"
      />

      <div class="grid grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="kelas_select" class="text-xs font-semibold text-slate-500 uppercase tracking-widest block mb-1">Kelas *</label>
          <select id="kelas_select" bind:value={newKelas} class="w-full h-12 px-4 border border-slate-200 rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-cobalt-500/20 focus:border-cobalt-600 bg-white transition-colors duration-150 text-slate-800 text-sm font-semibold">
            {#each classesList as c}
              <option value={c.id}>{c.nama_kelas}</option>
            {/each}
          </select>
        </div>

        <div class="flex flex-col gap-1.5">
          <label for="ruang_select" class="text-xs font-semibold text-slate-500 uppercase tracking-widest block mb-1">Ruang Ujian *</label>
          <select id="ruang_select" bind:value={newRuang} class="w-full h-12 px-4 border border-slate-200 rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-cobalt-500/20 focus:border-cobalt-600 bg-white transition-colors duration-150 text-slate-800 text-sm font-semibold">
            {#each roomsList as r}
              <option value={r.id}>{r.nama_ruang}</option>
            {/each}
          </select>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4 items-end">
        <div class="flex flex-col gap-1.5">
          <label for="jk_select" class="text-xs font-semibold text-slate-500 uppercase tracking-widest block mb-1">Jenis Kelamin *</label>
          <select id="jk_select" bind:value={newJK} class="w-full h-12 px-4 border border-slate-200 rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-cobalt-500/20 focus:border-cobalt-600 bg-white transition-colors duration-150 text-slate-800 text-sm font-semibold">
            <option value="L">Laki-laki</option>
            <option value="P">Perempuan</option>
          </select>
        </div>

        <!-- Password Generator themed light -->
        <PasswordGenerator 
          bind:value={newPass} 
          length={10}
          label="Kata Sandi Login"
          placeholder="Buat password login otomatis"
          theme="light"
        />
      </div>
    </div>

    <div slot="footer" class="flex gap-3 justify-end">
      <Button variant="secondary" size="sm" theme="light" on:click={() => showAddModal = false} disabled={createLoading}>Batal</Button>
      <Button variant="primary" size="sm" theme="light" class="shadow-sm" on:click={createStudent} {createLoading}>Simpan</Button>
    </div>
  </Modal>

  <!-- CSV Import Modal (Light Theme Elegant styling) -->
  <Modal show={showCSVModal} title="Impor Data Siswa CSV Massal" size="md">
    <div class="space-y-5 text-slate-800 p-1">
      <p class="text-sm text-slate-500 leading-relaxed">
        Anda dapat mendaftarkan siswa secara sekaligus dengan mengunggah lembar spreadsheet dalam format CSV (.csv). 
      </p>

      <div class="bg-cobalt-50/50 border border-cobalt-100 p-4 rounded-xl text-xs space-y-3 text-slate-800 font-medium shadow-sm">
        <div class="font-bold uppercase tracking-wider text-cobalt-700 mb-1 flex items-center gap-1.5">
          <span>📋</span> Skema Kolom CSV Resmi:
        </div>
        <div class="flex flex-wrap gap-1 font-mono text-[11px] bg-white p-2 border border-cobalt-100 rounded-xl shadow-inner">
          <span class="px-1.5 py-0.5 bg-cobalt-50 text-cobalt-700 rounded font-semibold">no_id</span>
          <span class="text-slate-300">,</span>
          <span class="px-1.5 py-0.5 bg-cobalt-50 text-cobalt-700 rounded font-semibold">nama_peserta</span>
          <span class="text-slate-300">,</span>
          <span class="px-1.5 py-0.5 bg-cobalt-50 text-cobalt-700 rounded font-semibold">kelas_id</span>
          <span class="text-slate-300">,</span>
          <span class="px-1.5 py-0.5 bg-cobalt-50 text-cobalt-700 rounded font-semibold">ruang_id</span>
          <span class="text-slate-300">,</span>
          <span class="px-1.5 py-0.5 bg-cobalt-50 text-cobalt-700 rounded font-semibold">jenis_kelamin</span>
          <span class="text-slate-300">,</span>
          <span class="px-1.5 py-0.5 bg-cobalt-50 text-cobalt-700 rounded font-semibold">password</span>
        </div>
        <div class="text-slate-500 text-[11px] leading-relaxed pt-1 space-y-1">
          <p>• <strong>kelas_id</strong> dan <strong>ruang_id</strong> diisi berdasarkan angka ID relational database.</p>
          <p>• <strong>jenis_kelamin</strong> wajib diisi <strong>"L"</strong> atau <strong>"P"</strong>.</p>
          <p>• Baris pertama berkas CSV bertindak sebagai tajuk/header dan dilewati otomatis oleh sistem.</p>
        </div>
      </div>

      <!-- Dropzone Widget -->
      <div class="flex flex-col gap-2 pt-1">
        <label for="csv_file_input" class="text-xs font-semibold text-slate-500 uppercase tracking-widest">Pilih Berkas CSV</label>
        <div class="border-2 border-dashed border-cobalt-200 bg-cobalt-50/20 rounded-xl p-6 flex flex-col items-center justify-center text-center cursor-pointer hover:bg-cobalt-50/40 hover:border-cobalt-300 transition-colors duration-150 relative h-32 overflow-hidden">
          <input 
            id="csv_file_input"
            type="file" 
            accept=".csv" 
            on:change={handleCSVChange}
            class="absolute inset-0 opacity-0 cursor-pointer z-10 w-full h-full"
          />
          {#if csvFile}
            <span class="text-2xl mb-1.5">📄</span>
            <span class="text-xs font-bold text-cobalt-700 truncate max-w-[300px]">{csvFile.name}</span>
            <span class="text-[10px] text-slate-500 font-semibold mt-1">Ukuran: {Math.round(csvFile.size / 1024)} KB • Klik untuk mengganti berkas</span>
          {:else}
            <span class="text-2xl mb-1.5">📂</span>
            <span class="text-xs font-bold text-slate-700">Tarik berkas CSV ke sini atau klik untuk mencari</span>
            <span class="text-[10px] text-slate-400 font-semibold mt-1">Mendukung berkas ekstensi .csv hingga 5MB</span>
          {/if}
        </div>
      </div>
    </div>

    <div slot="footer" class="flex gap-3 justify-end">
      <Button variant="secondary" size="sm" theme="light" on:click={() => showCSVModal = false} disabled={importLoading}>Batal</Button>
      <Button variant="primary" size="sm" theme="light" class="shadow-sm" on:click={uploadCSV} {importLoading}>Unggah & Proses</Button>
    </div>
  </Modal>

  <!-- Delete Student Confirm Modal -->
  <ConfirmModal 
    bind:isOpen={showDeleteModal}
    title="Hapus Peserta Ujian"
    message={`Apakah Anda yakin ingin menghapus data siswa "${studentToDelete?.name || ''}"? Tindakan ini tidak dapat dibatalkan.`}
    confirmLabel="Hapus Siswa"
    cancelLabel="Batal"
    variant="danger"
    loading={deleteLoading}
    on:confirm={confirmDeleteStudent}
    on:cancel={() => { showDeleteModal = false; studentToDelete = null; }}
  />
</div>
