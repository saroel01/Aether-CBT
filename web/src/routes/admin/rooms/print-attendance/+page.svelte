<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';

  let roomId = 0;
  let roomName = '';
  let examTitle = 'Penilaian Akhir Semester 2025/2026';
  let students: any[] = [];
  let classesList: any[] = [];
  let loading = true;

  onMount(async () => {
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      roomId = parseInt(params.get('room_id') || '0');
    }

    if (roomId <= 0) {
      loading = false;
      return;
    }

    try {
      const [studentsRes, classesRes, roomsRes, settingsRes] = await Promise.all([
        api('/students'),
        api('/classes'),
        api('/rooms'),
        api('/student/active-info')
      ]);

      classesList = classesRes.data || [];
      const foundRoom = (roomsRes.data || []).find((r: any) => r.id === roomId);
      roomName = foundRoom ? foundRoom.nama_ruang : `ID: ${roomId}`;

      if (settingsRes.success && settingsRes.data) {
        examTitle = settingsRes.data.exam_title;
      }

      // Filter students by room ID
      students = (studentsRes.data || []).filter((s: any) => s.ruang_id === roomId);
      
      // Sort alphabetically or by No ID
      students.sort((a, b) => a.no_id.localeCompare(b.no_id));
    } catch (e) {
      console.error('Failed to load data for printing attendance:', e);
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

  // Get current local date string (e.g. Sabtu, 23 Mei 2026)
  function getLocalDateString(): string {
    const d = new Date();
    return d.toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
  }
</script>

<svelte:head>
  <title>Cetak Daftar Hadir - Aether CBT</title>
</svelte:head>

<div class="min-h-screen bg-white text-black p-6 font-sans antialiased">
  <!-- floating print controller (hidden on print) -->
  <div class="no-print mb-6 p-4 bg-slate-100 border rounded-2xl flex justify-between items-center max-w-4xl mx-auto shadow-sm">
    <div>
      <h3 class="text-sm font-bold text-slate-800">Daftar Hadir - {roomName || 'Semua Ruangan'}</h3>
      <p class="text-xs text-slate-500">Gunakan pintasan browser Ctrl+P jika printer dialog tidak terbuka secara otomatis.</p>
    </div>
    <Button 
      variant="primary" 
      size="sm"
      theme="light"
      class="font-semibold shadow-sm"
      on:click={() => window.print()}
    >
      Cetak Halaman
    </Button>
  </div>

  {#if loading}
    <div class="text-center py-20 text-slate-400 no-print">
      <p class="text-sm font-semibold">Menyusun daftar hadir ruangan...</p>
    </div>
  {:else if roomId <= 0}
    <div class="text-center py-20 text-red-600 no-print">
      <p class="text-sm font-bold">Galat: Room ID tidak ditemukan.</p>
    </div>
  {:else}
    <!-- Printable Sheet -->
    <div class="max-w-4xl mx-auto p-4 border border-slate-300 rounded shadow-sm bg-white select-none printable-container">
      
      <!-- Document Header -->
      <div class="text-center border-b-2 border-double pb-4 mb-6">
        <h2 class="text-lg font-extrabold tracking-wide uppercase">DAFTAR HADIR PESERTA UJIAN</h2>
        <h3 class="text-sm font-bold uppercase tracking-tight text-slate-700 mt-1">{examTitle}</h3>
      </div>

      <!-- Metadata Box -->
      <div class="grid grid-cols-2 gap-4 text-xs text-left mb-6 font-semibold">
        <div class="space-y-1">
          <div class="grid grid-cols-3">
            <span class="text-slate-500">Mata Pelajaran</span>
            <span class="col-span-2 text-slate-900">: ————————</span>
          </div>
          <div class="grid grid-cols-3">
            <span class="text-slate-500">Hari / Tanggal</span>
            <span class="col-span-2 text-slate-900">: {getLocalDateString()}</span>
          </div>
        </div>
        <div class="space-y-1">
          <div class="grid grid-cols-3">
            <span class="text-slate-500">Ruangan</span>
            <span class="col-span-2 text-slate-900">: {roomName}</span>
          </div>
          <div class="grid grid-cols-3">
            <span class="text-slate-500">Waktu Ujian</span>
            <span class="col-span-2 text-slate-900">: ————————</span>
          </div>
        </div>
      </div>

      <!-- Attendance Table -->
      <table class="w-full text-xs text-left border-collapse border border-slate-400">
        <thead>
          <tr class="bg-slate-50">
            <th class="border border-slate-400 p-2.5 text-center w-12">No.</th>
            <th class="border border-slate-400 p-2.5 w-32">Nomor Peserta</th>
            <th class="border border-slate-400 p-2.5">Nama Lengkap</th>
            <th class="border border-slate-400 p-2.5 w-24">Kelas</th>
            <th class="border border-slate-400 p-2.5 text-center w-48">Tanda Tangan</th>
          </tr>
        </thead>
        <tbody>
          {#each students as s, idx}
            {@const isEven = idx % 2 === 0}
            <tr class="h-11">
              <td class="border border-slate-400 p-2 text-center font-bold text-slate-600">{idx + 1}.</td>
              <td class="border border-slate-400 p-2 font-mono font-bold text-slate-700">{s.no_id}</td>
              <td class="border border-slate-400 p-2 font-bold text-slate-800">{s.nama_peserta}</td>
              <td class="border border-slate-400 p-2 font-medium text-slate-600">{getClassName(s.kelas_id)}</td>
              <td class="border border-slate-400 p-2 relative">
                {#if isEven}
                  <span class="absolute left-3 text-[10px] text-slate-400 font-mono font-bold">{idx + 1}.</span>
                  <span class="absolute bottom-1 left-8 w-24 border-b border-dotted border-slate-400"></span>
                {:else}
                  <span class="absolute left-24 text-[10px] text-slate-400 font-mono font-bold">{idx + 1}.</span>
                  <span class="absolute bottom-1 left-28 w-24 border-b border-dotted border-slate-400"></span>
                {/if}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="5" class="text-center py-12 text-slate-400 font-medium">
                Belum ada siswa terdaftar di ruangan ini.
              </td>
            </tr>
          {/each}
        </tbody>
      </table>

      <!-- Signatures Footer -->
      <div class="grid grid-cols-3 gap-4 text-xs font-semibold mt-12 text-left">
        <div>
          <div>Pengawas Ujian Ruangan,</div>
          <div class="h-20"></div>
          <div class="font-extrabold text-slate-800">1. ————————————————</div>
          <div class="text-[10px] text-slate-400 font-medium mt-0.5">NIP. —————————————</div>
        </div>
        <div>
          <div>Saksi Ruangan,</div>
          <div class="h-20"></div>
          <div class="font-extrabold text-slate-800">2. ————————————————</div>
          <div class="text-[10px] text-slate-400 font-medium mt-0.5">NIP. —————————————</div>
        </div>
        <div class="text-center">
          <div>Proktor Utama / Kepala Ruang,</div>
          <div class="h-20"></div>
          <div class="font-extrabold text-slate-800">Drs. ———————————————</div>
          <div class="text-[10px] text-slate-400 font-medium mt-0.5">NIP. —————————————</div>
        </div>
      </div>

    </div>
  {/if}
</div>

<style>
  /* Print Optimization Styles */
  @media print {
    .no-print {
      display: none !important;
    }
    :global(body) {
      background-color: white !important;
      color: black !important;
    }
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
    .printable-container {
      border: none !important;
      box-shadow: none !important;
      padding: 0 !important;
      width: 100% !important;
      max-width: none !important;
    }
    table {
      width: 100% !important;
      border-color: black !important;
    }
    th, td {
      border-color: black !important;
    }
  }
</style>
