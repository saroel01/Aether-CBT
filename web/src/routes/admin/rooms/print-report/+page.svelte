<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';

  let roomId = 0;
  let roomName = '';
  let examTitle = 'Penilaian Akhir Semester 2025/2026';
  let totalStudents = 0;
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
      const [studentsRes, roomsRes, settingsRes] = await Promise.all([
        api('/students'),
        api('/rooms'),
        api('/student/active-info')
      ]);

      const foundRoom = (roomsRes.data || []).find((r: any) => r.id === roomId);
      roomName = foundRoom ? foundRoom.nama_ruang : `ID: ${roomId}`;

      if (settingsRes.success && settingsRes.data) {
        examTitle = settingsRes.data.exam_title;
      }

      // Calculate total students assigned to this room
      const roomStudents = (studentsRes.data || []).filter((s: any) => s.ruang_id === roomId);
      totalStudents = roomStudents.length;
    } catch (e) {
      console.error('Failed to load data for printing proctor report:', e);
    }
    loading = false;
    // Auto trigger print after rendering completes
    setTimeout(() => {
      window.print();
    }, 1000);
  });

  function getLocalDateString(): string {
    const d = new Date();
    return d.toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
  }

  function getDayName(): string {
    const d = new Date();
    return d.toLocaleDateString('id-ID', { weekday: 'long' });
  }

  function getFormattedDate(): string {
    const d = new Date();
    return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' });
  }
</script>

<svelte:head>
  <title>Cetak Berita Acara - Aether CBT</title>
</svelte:head>

<div class="min-h-screen bg-white text-black p-6 font-sans antialiased">
  <!-- floating print controller (hidden on print) -->
  <div class="no-print mb-6 p-4 bg-slate-100 border rounded-2xl flex justify-between items-center max-w-3xl mx-auto shadow-sm">
    <div>
      <h3 class="text-sm font-bold text-slate-800">Berita Acara Ujian - {roomName || 'Semua Ruangan'}</h3>
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
      <p class="text-sm font-semibold">Menyusun berita acara ruangan...</p>
    </div>
  {:else if roomId <= 0}
    <div class="text-center py-20 text-red-600 no-print">
      <p class="text-sm font-bold">Galat: Room ID tidak ditemukan.</p>
    </div>
  {:else}
    <!-- Printable Sheet -->
    <div class="max-w-3xl mx-auto p-8 border border-slate-300 rounded shadow-sm bg-white select-none printable-container leading-relaxed text-sm">
      
      <!-- Document Header -->
      <div class="text-center border-b-2 border-double pb-4 mb-6">
        <h2 class="text-lg font-extrabold tracking-wide uppercase">BERITA ACARA PELAKSANAAN UJIAN</h2>
        <h3 class="text-xs font-bold uppercase tracking-tight text-slate-700 mt-1">{examTitle}</h3>
      </div>

      <!-- Minutes Paragraph Context -->
      <div class="space-y-4 text-left">
        <p>
          Pada hari ini, <strong class="underline">{getDayName()}</strong> tanggal <strong class="underline">{getFormattedDate()}</strong>, 
          telah diselenggarakan pelaksanaan ujian <strong class="underline">{examTitle}</strong> untuk mata pelajaran:
        </p>

        <div class="pl-8 space-y-1.5 font-semibold">
          <div class="grid grid-cols-4">
            <span class="text-slate-500">Mata Pelajaran</span>
            <span class="col-span-3 text-slate-900">: ——————————————————————————</span>
          </div>
          <div class="grid grid-cols-4">
            <span class="text-slate-500">Ruangan</span>
            <span class="col-span-3 text-slate-900">: {roomName}</span>
          </div>
          <div class="grid grid-cols-4">
            <span class="text-slate-500">Pukul / Waktu</span>
            <span class="col-span-3 text-slate-900">: ——————————— s/d ———————————</span>
          </div>
        </div>

        <p class="pt-3">
          Telah dilaksanakan pencatatan kehadiran dan ketertiban ruangan dengan rincian data sebagai berikut:
        </p>

        <!-- Summary Table (Hand fillable blanks) -->
        <table class="w-full text-xs text-left border-collapse border border-slate-400 my-6 font-semibold">
          <thead>
            <tr class="bg-slate-50">
              <th class="border border-slate-400 p-2.5 w-12 text-center">No.</th>
              <th class="border border-slate-400 p-2.5">Kategori Rekapitulasi Peserta</th>
              <th class="border border-slate-400 p-2.5 w-44 text-center">Jumlah Siswa</th>
            </tr>
          </thead>
          <tbody>
            <tr class="h-10">
              <td class="border border-slate-400 p-2 text-center">1.</td>
              <td class="border border-slate-400 p-2">Jumlah Peserta Terdaftar (Daftar Utama)</td>
              <td class="border border-slate-400 p-2 text-center font-bold text-sm text-slate-800">{totalStudents} orang</td>
            </tr>
            <tr class="h-10">
              <td class="border border-slate-400 p-2 text-center">2.</td>
              <td class="border border-slate-400 p-2">Jumlah Peserta Hadir (Mengikuti Ujian)</td>
              <td class="border border-slate-400 p-2 text-center text-slate-400 font-normal">. . . . . . . . . . . . . . . . . . . . . . orang</td>
            </tr>
            <tr class="h-10">
              <td class="border border-slate-400 p-2 text-center">3.</td>
              <td class="border border-slate-400 p-2">Jumlah Peserta Tidak Hadir (Absen / Sakit)</td>
              <td class="border border-slate-400 p-2 text-center text-slate-400 font-normal">. . . . . . . . . . . . . . . . . . . . . . orang</td>
            </tr>
          </tbody>
        </table>

        <!-- Un-attendance detailed note -->
        <div class="border border-slate-300 p-4 rounded-xl space-y-2 mt-4">
          <div class="text-xs font-bold text-slate-500 uppercase tracking-wider">Identitas Peserta Tidak Hadir / Keterangan Ujian:</div>
          <div class="h-5 border-b border-dotted border-slate-300"></div>
          <div class="h-5 border-b border-dotted border-slate-300"></div>
        </div>

        <!-- Special Incidents -->
        <div class="border border-slate-300 p-4 rounded-xl space-y-2 mt-4">
          <div class="text-xs font-bold text-slate-500 uppercase tracking-wider">Catatan Kejadian Khusus / Pelanggaran Tata Tertib Selama Ujian:</div>
          <p class="text-[10px] text-slate-400 leading-none mb-2">* E.g. Terjadi masalah perangkat komputer, peringatan kecurangan tab-switch berulang, dll.</p>
          <div class="h-5 border-b border-dotted border-slate-300"></div>
          <div class="h-5 border-b border-dotted border-slate-300"></div>
        </div>

        <p class="pt-4 text-xs font-medium leading-relaxed">
          Demikian berita acara ini dibuat dengan sebenar-benarnya untuk dipergunakan sebagaimana mestinya dan menjadi laporan resmi pelaksanaan ujian.
        </p>
      </div>

      <!-- Signatures Footer -->
      <div class="grid grid-cols-2 gap-4 text-xs font-semibold mt-16 text-left pt-6">
        <div>
          <div>Proktor Ruangan / IT Staff,</div>
          <div class="h-20"></div>
          <div class="font-extrabold text-slate-800">_________________________________</div>
          <div class="text-[10px] text-slate-400 font-medium mt-0.5">NIP/NUPTK. ————————————————</div>
        </div>
        <div class="text-right pr-6">
          <div>Pengawas Ruang Ujian,</div>
          <div class="h-20"></div>
          <div class="font-extrabold text-slate-800">_________________________________</div>
          <div class="text-[10px] text-slate-400 font-medium mt-0.5">NIP/NUPTK. ————————————————</div>
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
