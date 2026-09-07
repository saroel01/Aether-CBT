<script lang="ts">
  import { authStore } from '$lib/stores/auth';
  import { api, apiUrl, authHeaders, qrCodeUrl, auth as apiAuth } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import { toast } from '$lib/stores/toast';

  let username = import.meta.env.DEV ? 'admin' : '';
  let password = import.meta.env.DEV ? 'admin123' : '';
  let error = '';
  let loading = false;
  let loggedIn = false;

  let stats = {
    students: 0,
    classes: 0,
    rooms: 0,
    mapel: 0
  };

  let activeToken = '';
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
        api('/admin/settings').catch(() => ({ data: {} }))
      ]);

      stats.students = studentsRes.data?.length || 0;
      stats.classes = classesRes.data?.length || 0;
      stats.rooms = roomsRes.data?.length || 0;
      stats.mapel = mapelRes.data?.length || 0;
      
      if (activeInfo.success && activeInfo.data) {
        activeExamTitle = activeInfo.data.exam_title;
        activeToken = activeInfo.data.token || activeToken;
      }
    } catch (e) {
      console.warn('Could not load stats', e);
    }
  }

  // Triggers downloading the CSV sheet containing exam scores directly from backend Go endpoint!
  function downloadResultsCSV() {
    try {
      loading = true;
      toast.info('Menyiapkan berkas ekspor hasil...');
      
      fetch(apiUrl('/admin/results/export-csv'), {
        method: 'GET',
        headers: authHeaders()
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
    <!-- Login Cockpit (Precision Light Form) -->
    <div class="min-h-[80vh] flex items-center justify-center relative overflow-hidden">
      <div class="w-full max-w-md z-10">
        <Card padding="lg" class="border-slate-200 bg-white shadow-xl relative overflow-hidden rounded-2xl">
          <div class="absolute top-0 left-0 w-full h-[2px] bg-cobalt-600"></div>
          
          <div class="text-center mb-8">
            <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight font-display">Aether <span class="text-cobalt-600">CBT</span></h1>
            <p class="text-slate-500 text-xs font-semibold uppercase tracking-widest mt-1.5 font-mono">Panel Proktor Utama</p>
          </div>

          <form on:submit|preventDefault={login} class="space-y-4">
            <!-- Admin Username input using theme="light" -->
            <Input 
              id="username"
              label="Username Admin" 
              placeholder="Contoh: admin" 
              bind:value={username}
              disabled={loading}
              theme="light"
            >
              <span slot="iconLeft">
                <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
              </span>
            </Input>

            <!-- Admin Password input using theme="light" -->
            <Input 
              id="password"
              label="Password Admin" 
              type="password" 
              placeholder="Masukkan password admin" 
              bind:value={password}
              disabled={loading}
              theme="light"
            >
              <span slot="iconLeft">
                <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </span>
            </Input>

            {#if error}
              <div class="p-3 bg-ruby-50 border border-ruby-200 rounded-xl text-ruby-600 text-xs font-semibold text-center">
                {error}
              </div>
            {/if}

            <Button 
              type="submit" 
              variant="primary" 
              size="lg" 
              theme="light"
              class="w-full font-semibold mt-6 shadow-sm" 
              {loading}
            >
              {loading ? 'Masuk...' : 'Sign In'}
            </Button>
          </form>

          {#if import.meta.env.DEV}
            <p class="text-xs text-center text-slate-400 mt-6 leading-relaxed">
              Dev only: kolom kredensial sudah terisi otomatis — langsung klik Sign In.
            </p>
          {/if}
        </Card>
      </div>
    </div>
  {:else}
    <!-- Dashboard Overview Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-slate-200 pb-6">
      <div>
        <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight font-display">Ikhtisar CBT</h1>
        <p class="text-slate-500 text-sm mt-1">Dashboard Proktor • Kelola konfigurasi ujian dan rekapitulasi data.</p>
      </div>

      <div class="flex items-center gap-3">
        <Button variant="secondary" size="sm" theme="light" class="font-semibold shadow-sm" on:click={loadStats}>
          Segarkan Data
        </Button>
        <Button variant="primary" size="sm" theme="light" class="font-semibold shadow-sm" on:click={downloadResultsCSV}>
          Ekspor Skor (CSV)
        </Button>
      </div>
    </div>

    <!-- Pusat Kontrol Operasional -->
    <Card padding="lg" class="border-slate-200 bg-white shadow-sm relative overflow-hidden rounded-2xl">
      <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-100"></div>
      
      <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 mb-6 pb-4 border-b border-slate-100">
        <div>
          <h3 class="text-sm font-bold text-slate-800 uppercase tracking-widest font-mono">Pusat Kontrol Operasional</h3>
          <p class="text-xs text-slate-500 mt-1">Status dan rekapitulasi data entitas akademik aktif di dalam pangkalan data.</p>
        </div>
        <div class="inline-flex items-center gap-2 px-3 py-1 bg-emerald-50 border border-emerald-200 rounded-full text-xs font-semibold text-emerald-700 font-mono">
          <span class="relative flex h-2 w-2">
            <span class="inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
          </span>
          Database WAL Active
        </div>
      </div>

      <div class="grid grid-cols-2 lg:grid-cols-4 gap-6">
        <div class="flex flex-col gap-1 p-4 bg-slate-50 border border-slate-200/80 rounded-xl">
          <span class="text-[10px] text-slate-500 font-bold uppercase tracking-wider font-mono">Siswa Terdaftar</span>
          <div class="text-3xl font-extrabold text-slate-900 font-display mt-0.5 tabular-nums font-mono">{stats.students}</div>
          <span class="text-[11px] text-slate-500 font-medium leading-relaxed mt-1">• Siswa aktif terverifikasi</span>
        </div>

        <div class="flex flex-col gap-1 p-4 bg-slate-50 border border-slate-200/80 rounded-xl">
          <span class="text-[10px] text-slate-500 font-bold uppercase tracking-wider font-mono">Tingkatan Kelas</span>
          <div class="text-3xl font-extrabold text-slate-900 font-display mt-0.5 tabular-nums font-mono">{stats.classes}</div>
          <span class="text-[11px] text-slate-500 font-medium leading-relaxed mt-1">• Kelas kurikulum terpetakan</span>
        </div>

        <div class="flex flex-col gap-1 p-4 bg-slate-50 border border-slate-200/80 rounded-xl">
          <span class="text-[10px] text-slate-500 font-bold uppercase tracking-wider font-mono">Ruangan Fisik</span>
          <div class="text-3xl font-extrabold text-slate-900 font-display mt-0.5 tabular-nums font-mono">{stats.rooms}</div>
          <span class="text-[11px] text-slate-500 font-medium leading-relaxed mt-1">• Ruang pemantauan pengawas</span>
        </div>

        <div class="flex flex-col gap-1 p-4 bg-slate-50 border border-slate-200/80 rounded-xl">
          <span class="text-[10px] text-slate-500 font-bold uppercase tracking-wider font-mono">Mata Pelajaran</span>
          <div class="text-3xl font-extrabold text-slate-900 font-display mt-0.5 tabular-nums font-mono">{stats.mapel}</div>
          <span class="text-[11px] text-slate-500 font-medium leading-relaxed mt-1">• Silabus aktif terdaftar</span>
        </div>
      </div>
    </Card>

    <!-- Active token and quick actions -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">
      <!-- Quick actions & Info (2/3) -->
      <div class="lg:col-span-2 flex flex-col gap-6">
        <Card padding="lg" class="border-slate-200 bg-white shadow-sm relative overflow-hidden rounded-2xl">
          <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-100"></div>

          <h3 class="text-sm font-bold text-slate-800 uppercase tracking-widest font-mono mb-4 pb-2 border-b border-slate-100">Pintasan Manajemen</h3>
          
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 text-center">
            <a href="/admin/students" class="p-4 bg-slate-50 border border-slate-200/80 rounded-xl hover:bg-cobalt-50/40 hover:border-cobalt-200 transition-colors duration-150 group flex flex-col items-center justify-center gap-2">
              <svg class="h-6 w-6 text-slate-500 group-hover:text-cobalt-600 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
              <span class="text-xs font-semibold text-slate-700">Data Siswa</span>
            </a>
            
            <a href="/admin/classes" class="p-4 bg-slate-50 border border-slate-200/80 rounded-xl hover:bg-cobalt-50/40 hover:border-cobalt-200 transition-colors duration-150 group flex flex-col items-center justify-center gap-2">
              <svg class="h-6 w-6 text-slate-500 group-hover:text-cobalt-600 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
              </svg>
              <span class="text-xs font-semibold text-slate-700">Data Kelas</span>
            </a>

            <a href="/admin/mapel" class="p-4 bg-slate-50 border border-slate-200/80 rounded-xl hover:bg-cobalt-50/40 hover:border-cobalt-200 transition-colors duration-150 group flex flex-col items-center justify-center gap-2">
              <svg class="h-6 w-6 text-slate-500 group-hover:text-cobalt-600 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
              </svg>
              <span class="text-xs font-semibold text-slate-700">Mata Pelajaran</span>
            </a>

            <a href="/admin/rooms" class="p-4 bg-slate-50 border border-slate-200/80 rounded-xl hover:bg-cobalt-50/40 hover:border-cobalt-200 transition-colors duration-150 group flex flex-col items-center justify-center gap-2">
              <svg class="h-6 w-6 text-slate-500 group-hover:text-cobalt-600 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
              </svg>
              <span class="text-xs font-semibold text-slate-700">Ruangan Ujian</span>
            </a>
          </div>
        </Card>

        <Card padding="lg" class="border-slate-200 bg-white shadow-sm relative overflow-hidden rounded-2xl">
          <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-100"></div>

          <h3 class="text-sm font-bold text-slate-800 uppercase tracking-widest font-mono mb-4 pb-2 border-b border-slate-100">Informasi Teknis Server</h3>
          
          <div class="space-y-3.5 text-sm">
            <div class="flex justify-between py-2 border-b border-slate-100">
              <span class="text-slate-500 font-medium">Judul Ujian Aktif:</span>
              <span class="font-bold text-slate-800 font-display">{activeExamTitle}</span>
            </div>
            <div class="flex justify-between py-2 border-b border-slate-100">
              <span class="text-slate-500 font-medium">Maksimum Siswa Simultan:</span>
              <span class="font-semibold text-slate-800 tabular-nums">500 Siswa / Tenant</span>
            </div>
            <div class="flex justify-between py-2 border-b border-slate-100">
              <span class="text-slate-500 font-medium">Driver Database:</span>
              <span class="font-semibold text-slate-800 font-mono">SQLite 3 (WAL Mode)</span>
            </div>
            <div class="flex justify-between py-2">
              <span class="text-slate-500 font-medium">Status Mesin Autentikasi:</span>
              <span class="font-bold text-emerald-600 flex items-center gap-2">
                <span class="h-2 w-2 bg-emerald-500 rounded-full"></span>
                JWT Token Enforced
              </span>
            </div>
          </div>
        </Card>
      </div>

      <!-- Live Token QR (1/3) -->
      <div class="lg:col-span-1 flex flex-col gap-6">
        <Card padding="md" class="border-slate-200 bg-white text-center shadow-sm relative overflow-hidden rounded-2xl">
          <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-100"></div>

          <div class="text-[10px] text-slate-500 font-bold uppercase tracking-wider mb-2 font-mono">Token Ujian Aktif</div>
          
          <div class="text-2xl font-extrabold text-cobalt-700 font-mono tracking-wider mb-4 bg-cobalt-50 py-2.5 rounded-xl border border-cobalt-200 tabular-nums">
            {activeToken}
          </div>

          <div class="text-[10px] text-slate-500 font-bold uppercase tracking-wider mb-3 font-mono">QR Code Ujian Resmi</div>
          
          <div class="bg-white p-3 border border-slate-200 rounded-2xl inline-block mx-auto mb-3 shadow-sm">
            <img src={qrCodeUrl(activeToken)} alt="QR Token" class="h-40 w-40 mx-auto" />
          </div>

          <p class="text-xs text-slate-500 leading-relaxed px-2">
            Gunakan QR Code di atas pada layar proyektor kelas untuk mempermudah siswa melakukan login verifikasi token.
          </p>
        </Card>
      </div>
    </div>
  {/if}
</div>
