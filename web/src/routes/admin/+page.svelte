<script lang="ts">
  import { authStore } from '$lib/stores/auth';
  import { api, auth as apiAuth } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import { toast } from '$lib/stores/toast';

  let username = 'admin';
  let password = 'admin123';
  let error = '';
  let loading = false;
  let loggedIn = false;

  let stats = {
    students: 0,
    classes: 0,
    rooms: 0,
    mapel: 0
  };

  let activeToken = 'ujian2026';
  let activeExamTitle = 'Ujian Akhir Semester 2025/2026';

  // React to auth store
  $: {
    loggedIn = $authStore.isAuthenticated;
    if (loggedIn) {
      loadStats();
    }
  }

  async function login() {
    error = '';
    loading = true;
    try {
      const res = await apiAuth.login(username, password);
      if (res.success && res.data?.token) {
        authStore.login(res.data.token, res.data.user);
        toast.success('Login Admin berhasil!');
      } else {
        error = 'Login gagal';
        toast.error(error);
      }
    } catch (e: any) {
      error = e.message || 'Error login. Apakah server backend menyala?';
      toast.error(error);
    }
    loading = false;
  }

  async function loadStats() {
    try {
      const [studentsRes, classesRes, roomsRes, mapelRes, activeInfo] = await Promise.all([
        api('/students').catch(() => ({ data: [] })),
        api('/classes').catch(() => ({ data: [] })),
        api('/rooms').catch(() => ({ data: [] })),
        api('/mapel').catch(() => ({ data: [] })),
        api('/student/active-info').catch(() => ({ data: {} }))
      ]);

      stats.students = studentsRes.data?.length || 0;
      stats.classes = classesRes.data?.length || 0;
      stats.rooms = roomsRes.data?.length || 0;
      stats.mapel = mapelRes.data?.length || 0;
      
      if (activeInfo.success && activeInfo.data) {
        activeExamTitle = activeInfo.data.exam_title;
      }
    } catch (e) {
      console.warn('Could not load stats', e);
    }
  }

  // Triggers downloading the CSV sheet containing exam scores directly from backend Go endpoint!
  function downloadResultsCSV() {
    try {
      const token = localStorage.getItem('aether_token');
      // Create a native link to trigger file download, passing JWT token via URL query parameter if headers are not possible,
      // or we can simply use fetch to get it and generate a local URL! That is 100% compliant with auth headers!
      loading = true;
      toast.info('Menyiapkan berkas ekspor hasil...');
      
      fetch('http://localhost:3000/api/admin/results/export-csv', {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`,
          'X-Tenant-ID': '1'
        }
      })
      .then(res => {
        if (!res.ok) throw new Error('Download failed');
        return res.blob();
      })
      .then(blob => {
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'rekap_skor_ujian.csv';
        document.body.appendChild(a);
        a.click();
        a.remove();
        toast.success('Ekspor CSV berhasil diunduh!');
      })
      .catch(err => {
        toast.error('Gagal mengekspor data: ' + err.message);
      })
      .finally(() => {
        loading = false;
      });
    } catch (e: any) {
      toast.error('Gagal memicu ekspor: ' + e.message);
    }
  }
</script>

<svelte:head>
  <title>Admin Dashboard - Aether CBT</title>
</svelte:head>

<div class="p-8 flex flex-col gap-8 max-w-7xl mx-auto">
  {#if !loggedIn}
    <!-- Login Cockpit -->
    <div class="min-h-[80vh] flex items-center justify-center">
      <div class="w-full max-w-md">
        <Card padding="lg" class="border-slate-200 bg-white shadow-xl relative overflow-hidden">
          <div class="absolute top-0 left-0 w-full h-[3px] bg-gradient-to-r from-indigo-600 to-indigo-700"></div>
          
          <div class="text-center mb-8">
            <h1 class="text-3xl font-extrabold text-slate-800 tracking-tight">Aether CBT</h1>
            <p class="text-slate-500 text-xs mt-1">Masuk ke Panel Proktor Utama</p>
          </div>

          <form on:submit|preventDefault={login} class="space-y-4">
            <Input 
              id="username"
              label="Username Admin" 
              placeholder="Contoh: admin" 
              bind:value={username}
              disabled={loading}
            >
              <span slot="iconLeft">
                <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
              </span>
            </Input>

            <Input 
              id="password"
              label="Password Admin" 
              type="password" 
              placeholder="Masukkan password admin" 
              bind:value={password}
              disabled={loading}
            >
              <span slot="iconLeft">
                <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </span>
            </Input>

            {#if error}
              <div class="p-3 bg-red-50 border border-red-100 rounded-xl text-red-600 text-xs font-semibold text-center">
                {error}
              </div>
            {/if}

            <Button 
              type="submit" 
              variant="primary" 
              size="lg" 
              class="w-full bg-indigo-600 hover:bg-indigo-700 text-white font-semibold shadow-md py-3.5 border-none" 
              {loading}
            >
              {loading ? 'Masuk...' : 'Sign In'}
            </Button>
          </form>

          <p class="text-xs text-center text-slate-400 mt-6 leading-relaxed">
            Kredensial Default: <span class="font-mono text-indigo-600 font-bold">admin</span> / <span class="font-mono text-indigo-600 font-bold">admin123</span>
          </p>
        </Card>
      </div>
    </div>
  {:else}
    <!-- Dashboard Overview Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b pb-6">
      <div>
        <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Ikhtisar CBT</h1>
        <p class="text-slate-500 text-sm">Dashboard Proktor • Kelola ujian dan pantau statistik sekolah.</p>
      </div>

      <div class="flex items-center gap-3">
        <Button variant="secondary" size="md" class="font-semibold" on:click={loadStats}>
          Segarkan Data
        </Button>
        <Button variant="primary" size="md" class="bg-indigo-600 border-none hover:bg-indigo-700 font-semibold shadow-md shadow-indigo-100" on:click={downloadResultsCSV}>
          Ekspor Skor (CSV)
        </Button>
      </div>
    </div>

    <!-- Stats Grid Cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-6">
      <Card padding="md" class="border-slate-200/50 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div class="absolute left-0 top-0 h-full w-[4px] bg-indigo-500"></div>
        <div>
          <span class="text-xs text-slate-400 font-bold uppercase tracking-wider">Jumlah Siswa</span>
          <div class="text-4xl font-extrabold text-slate-800 mt-1">{stats.students}</div>
        </div>
      </Card>

      <Card padding="md" class="border-slate-200/50 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div class="absolute left-0 top-0 h-full w-[4px] bg-purple-500"></div>
        <div>
          <span class="text-xs text-slate-400 font-bold uppercase tracking-wider">Jumlah Kelas</span>
          <div class="text-4xl font-extrabold text-slate-800 mt-1">{stats.classes}</div>
        </div>
      </Card>

      <Card padding="md" class="border-slate-200/50 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div class="absolute left-0 top-0 h-full w-[4px] bg-teal-500"></div>
        <div>
          <span class="text-xs text-slate-400 font-bold uppercase tracking-wider">Jumlah Ruang</span>
          <div class="text-4xl font-extrabold text-slate-800 mt-1">{stats.rooms}</div>
        </div>
      </Card>

      <Card padding="md" class="border-slate-200/50 bg-white flex items-center justify-between shadow-sm relative overflow-hidden">
        <div class="absolute left-0 top-0 h-full w-[4px] bg-pink-500"></div>
        <div>
          <span class="text-xs text-slate-400 font-bold uppercase tracking-wider">Mata Pelajaran</span>
          <div class="text-4xl font-extrabold text-slate-800 mt-1">{stats.mapel}</div>
        </div>
      </Card>
    </div>

    <!-- Active token and quick actions -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">
      <!-- Quick actions & Info (2/3) -->
      <div class="lg:col-span-2 flex flex-col gap-6">
        <Card padding="lg" class="border-slate-200/60 bg-white shadow-sm">
          <h3 class="text-lg font-bold text-slate-800 mb-4 pb-2 border-b">Menu Pintasan Manajemen</h3>
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 text-center">
            <a href="/admin/students" class="p-4 bg-slate-50 border rounded-2xl hover:bg-indigo-50/40 hover:border-indigo-200 transition group flex flex-col items-center justify-center gap-2">
              <svg class="h-6 w-6 text-slate-400 group-hover:text-indigo-500 transition" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
              <span class="text-xs font-semibold text-slate-700">Data Siswa</span>
            </a>
            
            <a href="/admin/classes" class="p-4 bg-slate-50 border rounded-2xl hover:bg-indigo-50/40 hover:border-indigo-200 transition group flex flex-col items-center justify-center gap-2">
              <svg class="h-6 w-6 text-slate-400 group-hover:text-indigo-500 transition" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
              </svg>
              <span class="text-xs font-semibold text-slate-700">Data Kelas</span>
            </a>

            <a href="/admin/mapel" class="p-4 bg-slate-50 border rounded-2xl hover:bg-indigo-50/40 hover:border-indigo-200 transition group flex flex-col items-center justify-center gap-2">
              <svg class="h-6 w-6 text-slate-400 group-hover:text-indigo-500 transition" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
              </svg>
              <span class="text-xs font-semibold text-slate-700">Mata Pelajaran</span>
            </a>

            <a href="/admin/rooms" class="p-4 bg-slate-50 border rounded-2xl hover:bg-indigo-50/40 hover:border-indigo-200 transition group flex flex-col items-center justify-center gap-2">
              <svg class="h-6 w-6 text-slate-400 group-hover:text-indigo-500 transition" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
              </svg>
              <span class="text-xs font-semibold text-slate-700">Ruangan Ujian</span>
            </a>
          </div>
        </Card>

        <Card padding="lg" class="border-slate-200/60 bg-white shadow-sm">
          <h3 class="text-lg font-bold text-slate-800 mb-4 pb-2 border-b">Detail Konfigurasi Server</h3>
          <div class="space-y-3.5 text-sm">
            <div class="flex justify-between py-2 border-b border-slate-50">
              <span class="text-slate-400 font-medium">Judul Ujian Aktif:</span>
              <span class="font-bold text-slate-800">{activeExamTitle}</span>
            </div>
            <div class="flex justify-between py-2 border-b border-slate-50">
              <span class="text-slate-400 font-medium">Maksimum Siswa Simultan:</span>
              <span class="font-bold text-slate-800">500 Siswa / Tenant</span>
            </div>
            <div class="flex justify-between py-2 border-b border-slate-50">
              <span class="text-slate-400 font-medium">Driver Database:</span>
              <span class="font-bold text-slate-800 font-mono">SQLite 3 (WAL Mode)</span>
            </div>
            <div class="flex justify-between py-2">
              <span class="text-slate-400 font-medium">Status Mesin Autentikasi:</span>
              <span class="font-semibold text-emerald-600 flex items-center gap-1.5">
                <span class="h-2 w-2 bg-emerald-500 rounded-full"></span>
                JWT Token Aktif
              </span>
            </div>
          </div>
        </Card>
      </div>

      <!-- Live Token QR (1/3) -->
      <div class="lg:col-span-1 flex flex-col gap-6">
        <Card padding="md" class="border-slate-200/50 bg-white text-center shadow-sm">
          <div class="text-xs text-slate-400 font-bold uppercase tracking-wider mb-2">Token Ujian Aktif</div>
          <div class="text-2xl font-extrabold text-indigo-600 font-mono tracking-wider mb-4 bg-indigo-50/50 py-2.5 rounded-2xl border border-indigo-100">
            {activeToken}
          </div>

          <div class="text-xs text-slate-400 font-bold uppercase tracking-wider mb-3">QR Code Ujian Resmi</div>
          <div class="bg-slate-50 p-3 border rounded-3xl inline-block mx-auto mb-3">
            <img src="http://localhost:3000/api/qrcode?text={activeToken}" alt="QR Token" class="h-40 w-40 mx-auto" />
          </div>

          <p class="text-xs text-slate-500 leading-relaxed px-2">
            Gunakan QR Code di atas pada layar projektor kelas untuk mempermudah siswa melakukan login verifikasi token.
          </p>
        </Card>
      </div>
    </div>
  {/if}
</div>
