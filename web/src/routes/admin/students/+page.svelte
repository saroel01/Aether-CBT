<script lang="ts">
  import { api, apiUrl, authHeaders } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import PasswordGenerator from '$lib/components/PasswordGenerator.svelte';
  import { toast } from '$lib/stores/toast';

  let students: any[] = [];
  let classesList: any[] = [];
  let roomsList: any[] = [];
  
  let loading = true;
  let error = '';
  
  // Modal toggles
  let showAddModal = false;
  let showCSVModal = false;
  let csvFile: File | null = null;
  let importLoading = false;

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

  function getRoomName(id: number): string {
    const found = roomsList.find(r => r.id === id);
    return found ? found.nama_ruang : `ID: ${id}`;
  }

  async function deleteStudent(id: number, name: string) {
    const confirm = window.confirm(`Apakah Anda yakin ingin menghapus siswa "${name}"?`);
    if (!confirm) return;
    try {
      await api(`/students/${id}`, { method: 'DELETE' });
      toast.success(`Siswa "${name}" berhasil dihapus!`);
      await loadInitialData();
    } catch (e: any) {
      toast.error('Gagal menghapus siswa: ' + e.message);
    }
  }
</script>

<svelte:head>
  <title>Manajemen Peserta - Aether CBT</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b pb-6">
    <div>
      <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Peserta Ujian (Siswa)</h1>
      <p class="text-slate-500 text-sm">Kelola daftar registrasi peserta ujian resmi di sekolah.</p>
    </div>

    <div class="flex items-center gap-3">
      <a href="/admin/students/print-cards" target="_blank">
        <Button variant="secondary" size="md" class="font-semibold border-indigo-200 text-indigo-700 hover:bg-indigo-50">
          Cetak Kartu Ujian
        </Button>
      </a>
      <Button variant="secondary" size="md" class="font-semibold" on:click={() => showCSVModal = true}>
        Impor CSV Massal
      </Button>
      <Button variant="primary" size="md" class="bg-indigo-600 border-none hover:bg-indigo-700 font-semibold shadow-md shadow-indigo-100" on:click={() => showAddModal = true}>
        Tambah Siswa
      </Button>
    </div>
  </div>

  {#if loading}
    <div class="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
      <svg class="animate-spin h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
      <p class="text-sm font-semibold">Memuat daftar siswa...</p>
    </div>
  {:else if error}
    <div class="p-6 bg-red-50 border border-red-100 rounded-3xl text-center text-red-600 font-medium">
      Gagal mengambil data siswa: {error}
    </div>
  {:else}
    <Table>
      <thead>
        <tr>
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
        {#each students as s}
          <tr>
            <td class="font-mono font-bold text-slate-500">{s.no_id}</td>
            <td class="font-semibold text-slate-800">{s.nama_peserta}</td>
            <td>
              {#if s.jenis_kelamin === 'L'}
                <Badge variant="info">Laki-laki</Badge>
              {:else if s.jenis_kelamin === 'P'}
                <Badge variant="warning">Perempuan</Badge>
              {:else}
                <Badge variant="neutral">—</Badge>
              {/if}
            </td>
            <td class="font-medium text-slate-600">{getClassName(s.kelas_id)}</td>
            <td class="font-medium text-slate-600">{getRoomName(s.ruang_id)}</td>
            <td class="font-mono text-xs text-slate-400">siswa123</td>
            <td class="text-center">
              <Button 
                variant="danger" 
                size="sm" 
                class="bg-red-50 text-red-600 hover:bg-red-600 hover:text-white border-red-100 font-semibold"
                on:click={() => deleteStudent(s.id, s.nama_peserta)}
              >
                Hapus
              </Button>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="7" class="text-center py-16 text-slate-400 font-medium">
              Belum ada data siswa terdaftar. Klik "Tambah Siswa" atau "Impor CSV Massal" untuk memasukkan data.
            </td>
          </tr>
        {/each}
      </tbody>
    </Table>
  {/if}

  <!-- Manual Add Student Modal -->
  <Modal show={showAddModal} title="Tambah Siswa Baru" size="md">
    <div class="space-y-4 text-slate-800">
      <Input 
        id="no_id"
        label="Nomor ID / Nomor Peserta *" 
        placeholder="Contoh: 2024009" 
        bind:value={newNoID}
      />
      
      <Input 
        id="nama"
        label="Nama Lengkap *" 
        placeholder="Contoh: Muhammad Rian" 
        bind:value={newNama}
      />

      <div class="grid grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="kelas_select" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Kelas *</label>
          <select id="kelas_select" bind:value={newKelas} class="w-full h-11 px-4 border rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 bg-white">
            {#each classesList as c}
              <option value={c.id}>{c.nama_kelas}</option>
            {/each}
          </select>
        </div>

        <div class="flex flex-col gap-1.5">
          <label for="ruang_select" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Ruang Ujian *</label>
          <select id="ruang_select" bind:value={newRuang} class="w-full h-11 px-4 border rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 bg-white">
            {#each roomsList as r}
              <option value={r.id}>{r.nama_ruang}</option>
            {/each}
          </select>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div class="flex flex-col gap-1.5">
          <label for="jk_select" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Jenis Kelamin *</label>
          <select id="jk_select" bind:value={newJK} class="w-full h-11 px-4 border rounded-xl outline-none hover:border-slate-300 focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 bg-white">
            <option value="L">Laki-laki</option>
            <option value="P">Perempuan</option>
          </select>
        </div>

        <PasswordGenerator 
          bind:value={newPass} 
          length={10}
          label="Kata Sandi Login"
          placeholder="Klik Generate untuk membuat password"
        />
      </div>
    </div>

    <div slot="footer" class="flex gap-2">
      <Button variant="secondary" size="sm" on:click={() => showAddModal = false} disabled={createLoading}>Batal</Button>
      <Button variant="primary" size="sm" class="bg-indigo-600 border-none hover:bg-indigo-700" on:click={createStudent} {createLoading}>Simpan</Button>
    </div>
  </Modal>

  <!-- CSV Import Modal -->
  <Modal show={showCSVModal} title="Impor Data Siswa CSV Massal" size="md">
    <div class="space-y-4 text-slate-800">
      <p class="text-sm text-slate-500 leading-relaxed">
        Anda dapat mendaftarkan siswa secara sekaligus dengan mengunggah lembar spreadsheet dalam format CSV (.csv). 
      </p>

      <div class="bg-indigo-50/50 border border-indigo-100 p-4 rounded-2xl text-xs space-y-2 text-indigo-950 font-medium">
        <div class="font-bold uppercase tracking-wider text-indigo-700 mb-1">Skema Kolom CSV:</div>
        <p class="font-mono">no_id, nama_peserta, kelas_id, ruang_id, jenis_kelamin, password</p>
        <p class="text-slate-500 text-[10px] leading-tight">
          * kelas_id dan ruang_id diisi berdasarkan angka ID relational.<br>
          * jenis_kelamin diisi "L" atau "P".<br>
          * baris pertama berkas CSV bertindak sebagai tajuk/header dan akan dilewati otomatis.
        </p>
      </div>

      <div class="flex flex-col gap-1.5 pt-2">
        <label for="csv_file_input" class="text-xs font-semibold text-slate-500 uppercase tracking-wider">Pilih Berkas CSV</label>
        <input 
          id="csv_file_input"
          type="file" 
          accept=".csv" 
          on:change={handleCSVChange}
          class="block w-full text-sm text-slate-500 file:mr-4 file:py-2.5 file:px-4 file:rounded-xl file:border-0 file:text-xs file:font-semibold file:bg-indigo-50 file:text-indigo-700 file:cursor-pointer hover:file:bg-indigo-100"
        />
      </div>
    </div>

    <div slot="footer" class="flex gap-2">
      <Button variant="secondary" size="sm" on:click={() => showCSVModal = false} disabled={importLoading}>Batal</Button>
      <Button variant="primary" size="sm" class="bg-indigo-600 border-none hover:bg-indigo-700" on:click={uploadCSV} {importLoading}>Unggah & Proses</Button>
    </div>
  </Modal>
</div>
