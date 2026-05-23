<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';

  let students: any[] = [];
  let classesList: any[] = [];
  let roomsList: any[] = [];
  let activeToken = 'ujian2026';
  let examTitle = 'Ujian Akhir Semester 2025/2026';
  let loading = true;

  onMount(async () => {
    try {
      const [studentsRes, classesRes, roomsRes, settingsRes] = await Promise.all([
        api('/students'),
        api('/classes'),
        api('/rooms'),
        api('/student/active-info')
      ]);

      students = studentsRes.data || [];
      classesList = classesRes.data || [];
      roomsList = roomsRes.data || [];
      if (settingsRes.success && settingsRes.data) {
        examTitle = settingsRes.data.exam_title;
        // active-info settings doesn't output secret token, we fallback to default or seeded value
        activeToken = 'ujian2026';
      }
    } catch (e) {
      console.error('Failed to load data for printing cards:', e);
    }
    loading = false;
    // Auto trigger print after rendering completes
    setTimeout(() => {
      window.print();
    }, 1000);
  });

  function getClassName(id: number): string {
    const found = classesList.find(c => c.id === id);
    return found ? found.nama_kelas : `ID: ${id}`;
  }

  function getRoomName(id: number): string {
    const found = roomsList.find(r => r.id === id);
    return found ? found.nama_ruang : `ID: ${id}`;
  }

  // Generates the login URL to encode in the QR Code
  function getLoginURL(noId: string): string {
    return `http://${window.location.hostname}:5173/student/login?no_id=${noId}&password=siswa123&token=${activeToken}`;
  }
</script>

<svelte:head>
  <title>Cetak Kartu Peserta - Aether CBT</title>
</svelte:head>

<div class="min-h-screen bg-white text-black p-4 font-sans antialiased">
  <!-- floating print controller (hidden on print) -->
  <div class="no-print mb-6 p-4 bg-slate-100 border rounded-2xl flex justify-between items-center max-w-5xl mx-auto shadow-sm">
    <div>
      <h3 class="text-sm font-bold text-slate-800">Pratinjau Kartu Peserta Ujian</h3>
      <p class="text-xs text-slate-500">Gunakan pintasan browser Ctrl+P jika printer dialog tidak terbuka secara otomatis.</p>
    </div>
    <button 
      type="button"
      class="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white font-bold text-xs rounded-xl shadow transition"
      on:click={() => window.print()}
    >
      Cetak Halaman
    </button>
  </div>

  {#if loading}
    <div class="text-center py-20 text-slate-400 no-print">
      <p class="text-sm font-semibold">Menyusun layout kartu peserta...</p>
    </div>
  {:else}
    <!-- Exam Cards Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6 max-w-5xl mx-auto">
      {#each students as s}
        {@const qrUrl = getLoginURL(s.no_id)}
        <div class="border-2 border-dashed border-slate-400 p-5 rounded-2xl relative bg-white flex flex-col justify-between h-[230px] shadow-sm select-none break-inside-avoid">
          <!-- Card Header -->
          <div class="flex items-center justify-between border-b pb-2 mb-3">
            <div class="text-left">
              <div class="text-[10px] font-extrabold text-indigo-600 uppercase tracking-widest leading-none">KARTU PESERTA</div>
              <div class="text-xs font-bold text-slate-900 leading-tight mt-0.5 line-clamp-1">{examTitle}</div>
            </div>
            <div class="text-right font-extrabold text-xs text-slate-400 font-mono tracking-wider scale-90">AETHER-CBT</div>
          </div>

          <!-- Card Content Body -->
          <div class="flex gap-4 items-start">
            <!-- Left Info fields -->
            <div class="flex-1 space-y-1.5 text-xs text-left">
              <div class="grid grid-cols-3">
                <span class="text-slate-500 font-medium">Nomor ID</span>
                <span class="col-span-2 font-extrabold font-mono text-slate-800 text-sm">: {s.no_id}</span>
              </div>
              <div class="grid grid-cols-3">
                <span class="text-slate-500 font-medium">Nama</span>
                <span class="col-span-2 font-bold text-slate-800 line-clamp-1">: {s.nama_peserta}</span>
              </div>
              <div class="grid grid-cols-3">
                <span class="text-slate-500 font-medium">Kelas</span>
                <span class="col-span-2 font-semibold text-slate-700">: {getClassName(s.kelas_id)}</span>
              </div>
              <div class="grid grid-cols-3">
                <span class="text-slate-500 font-medium">Ruangan</span>
                <span class="col-span-2 font-semibold text-slate-700">: {getRoomName(s.ruang_id)}</span>
              </div>
              <div class="grid grid-cols-3 pt-1 border-t border-slate-100 mt-2">
                <span class="text-slate-500 font-medium">Sandi</span>
                <span class="col-span-2 font-mono font-bold text-indigo-600">: siswa123</span>
              </div>
            </div>

            <!-- Right QR code box for login -->
            <div class="w-[85px] text-center border-l pl-3.5 shrink-0">
              <img 
                src="http://localhost:3000/api/qrcode?text={encodeURIComponent(qrUrl)}" 
                alt="Login QR" 
                class="h-[75px] w-[75px] mx-auto border p-1 rounded-lg"
              />
              <span class="text-[8px] font-extrabold text-slate-400 uppercase tracking-widest mt-1 block font-mono scale-90">SCAN LOGIN</span>
            </div>
          </div>

          <!-- Dotted cut guide line -->
          <div class="absolute -bottom-[1px] left-4 right-4 h-0 border-t border-dashed border-slate-200 no-print"></div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  /* Print Specific Optimization Styles */
  @media print {
    .no-print {
      display: none !important;
    }
    :global(body) {
      background-color: white !important;
      color: black !important;
    }
    /* Hide admin sidebar and header during print */
    :global(aside), :global(header) {
      display: none !important;
    }
    :global(.flex-1.overflow-y-auto) {
      overflow: visible !important;
    }
    :global(main), :global(.min-h-screen) {
      background-color: white !important;
      padding: 0 !important;
    }
    .grid {
      display: grid !important;
      grid-template-columns: repeat(2, minmax(0, 1fr)) !important;
      gap: 1.5rem !important;
    }
    .break-inside-avoid {
      break-inside: avoid !important;
    }
  }
</style>
