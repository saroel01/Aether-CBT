<script lang="ts">
  import { api } from '$lib/api';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import { toast } from '$lib/stores/toast';

  let noId = '';
  let password = '';
  let token = 'ujian2026';
  let error = '';
  let loading = false;

  onMount(() => {
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      const urlNoId = params.get('no_id');
      const urlPass = params.get('password');
      const urlToken = params.get('token');
      
      if (urlNoId && urlPass && urlToken) {
        noId = urlNoId;
        password = urlPass;
        token = urlToken;
        toast.info('Masuk otomatis dari QR Code terdeteksi. Memproses...');
        setTimeout(() => {
          login();
        }, 600);
      }
    }
  });

  async function login() {
    if (!noId || !password || !token) {
      toast.error('Semua kolom harus diisi!');
      return;
    }

    error = '';
    loading = true;
    try {
      const res = await api('/auth/student-login', {
        method: 'POST',
        body: JSON.stringify({ no_id: noId, password, token })
      });

      if (res.success) {
        localStorage.setItem('peserta_id', res.data.peserta_id);
        localStorage.setItem('peserta_no_id', noId);
        localStorage.setItem('exam_token', token);
        toast.success('Login berhasil! Silakan pilih mata pelajaran.');
        setTimeout(() => {
          window.location.href = '/student/select-subject';
        }, 800);
      }
    } catch (e: any) {
      error = e.message || 'Login gagal. Cek No. Peserta, password, dan token Anda.';
      toast.error(error);
    }
    loading = false;
  }
</script>

<svelte:head>
  <title>Login Peserta - Aether CBT</title>
  <meta name="description" content="Masuk ke halaman ujian Aether CBT" />
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-900 px-4">
  <div class="w-full max-w-md">
    <div class="text-center mb-8">
      <h1 class="text-4xl font-extrabold text-white tracking-tight mb-2">Aether CBT</h1>
      <p class="text-slate-400 text-sm">Computer-Based Testing Platform</p>
    </div>

    <Card padding="lg" class="border-slate-800/80 bg-slate-900/60 backdrop-blur-md text-white shadow-2xl relative overflow-hidden">
      <!-- Top aesthetic accent line -->
      <div class="absolute top-0 left-0 w-full h-[3px] bg-gradient-to-r from-indigo-500 via-purple-500 to-pink-500"></div>

      <h2 class="text-2xl font-bold text-center text-white mb-6">Login Peserta Ujian</h2>

      <div class="space-y-4">
        <Input 
          id="no-peserta"
          label="Nomor Peserta (No ID)" 
          placeholder="Contoh: 2024001" 
          bind:value={noId}
          disabled={loading}
          class="text-slate-800"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
            </svg>
          </span>
        </Input>

        <Input 
          id="password"
          label="Password" 
          type="password" 
          placeholder="Masukkan password Anda" 
          bind:value={password}
          disabled={loading}
          class="text-slate-800"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </span>
        </Input>

        <Input 
          id="exam-token"
          label="Token Ujian" 
          placeholder="Contoh: ujian2026" 
          bind:value={token}
          disabled={loading}
          class="text-slate-800"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M15 7a2 2 0 012 2m-2 4a2 2 0 012 2m-8-10a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2H9z" />
            </svg>
          </span>
        </Input>

        {#if error}
          <div class="p-3.5 bg-red-950/40 border border-red-900/50 rounded-xl text-red-400 text-xs font-medium text-center transition-all">
            {error}
          </div>
        {/if}

        <div class="pt-2">
          <Button 
            type="submit" 
            variant="primary" 
            size="lg" 
            class="w-full bg-gradient-to-r from-indigo-600 to-indigo-700 border-none font-semibold text-base py-3.5" 
            on:click={login}
            {loading}
          >
            {loading ? 'Memvalidasi...' : 'Masuk ke Sistem Ujian'}
          </Button>
        </div>
      </div>

      <div class="mt-6 text-center text-xs text-slate-500">
        Demo Login: <span class="font-mono text-indigo-400">2024001</span> / <span class="font-mono text-indigo-400">siswa123</span> • Token: <span class="font-mono text-indigo-400">ujian2026</span>
      </div>
    </Card>

    <div class="mt-8 text-center text-xs text-slate-600">
      Aether CBT v1.0 • Dikembangkan dengan Arsitektur Multi-Tenant Berkinerja Tinggi
    </div>
  </div>
</div>
