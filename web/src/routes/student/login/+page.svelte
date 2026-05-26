<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import { toast } from '$lib/stores/toast';

  let noId = '';
  let password = '';
  let token = '';
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
        localStorage.setItem('aether_token', res.data.token);
        localStorage.setItem('aether_user', JSON.stringify(res.data.user || {
          id: res.data.peserta_id,
          username: noId,
          role: 'student',
          tenant_id: 1
        }));
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

<div class="min-h-screen flex items-center justify-center bg-[oklch(0.12_0.012_250)] bg-grid-sovereign px-4 relative overflow-hidden select-none">
  <!-- Subtle organic glow -->
  <div class="absolute top-0 left-1/2 -translate-x-1/2 w-[700px] h-[350px] bg-indigo-500/5 rounded-full blur-[140px] pointer-events-none"></div>

  <div class="w-full max-w-md z-10">
    <!-- Sovereign Academic Emblem -->
    <div class="text-center mb-8">
      <div class="mb-5 flex justify-center">
        <div class="h-14 w-14 rounded-2xl bg-[oklch(0.16_0.014_250)] border border-[oklch(0.22_0.016_250)] flex items-center justify-center shadow-inner relative group">
          <div class="absolute inset-0 bg-indigo-500/5 rounded-2xl opacity-0 group-hover:opacity-100 transition-opacity"></div>
          <svg class="h-7 w-7 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
        </div>
      </div>
      <h1 class="text-3xl font-extrabold text-slate-100 tracking-tight font-display">Aether <span class="text-indigo-400 font-bold">CBT</span></h1>
      <p class="text-slate-500 text-[10px] font-bold uppercase tracking-widest mt-1.5 font-mono">Sovereign Testing Environment</p>
    </div>

    <!-- Elevated Slate Card (No glassmorphism slop) -->
    <Card theme="dark" padding="lg" class="shadow-2xl relative overflow-hidden">
      <!-- Minimalist elegant top border division -->
      <div class="absolute top-0 left-0 w-full h-[1px] bg-indigo-500/20"></div>

      <h2 class="text-xl font-bold text-center text-slate-200 mb-6 font-display">Login Peserta Ujian</h2>

      <div class="space-y-4">
        <!-- Student ID Input -->
        <Input 
          id="no-peserta"
          label="Nomor Peserta (No ID)" 
          placeholder="Contoh: 2024001" 
          bind:value={noId}
          disabled={loading}
          theme="dark"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
            </svg>
          </span>
        </Input>

        <!-- Password Input -->
        <Input 
          id="password"
          label="Password" 
          type="password" 
          placeholder="Masukkan password Anda" 
          bind:value={password}
          disabled={loading}
          theme="dark"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </span>
        </Input>

        <!-- Exam Token Input -->
        <Input 
          id="exam-token"
          label="Token Ujian" 
          placeholder="Contoh: ujian2026" 
          bind:value={token}
          disabled={loading}
          theme="dark"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M15 7a2 2 0 012 2m-2 4a2 2 0 012 2m-8-10a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2H9z" />
            </svg>
          </span>
        </Input>

        {#if error}
          <div class="p-3 bg-red-950/20 border border-red-900/30 rounded-2xl text-red-400 text-xs font-semibold text-center transition-all duration-300">
            {error}
          </div>
        {/if}

        <div class="pt-2">
          <Button 
            type="submit" 
            variant="primary" 
            size="lg" 
            class="w-full" 
            on:click={login}
            {loading}
          >
            {loading ? 'Memvalidasi...' : 'Masuk ke Sistem Ujian'}
          </Button>
        </div>
      </div>

      <div class="mt-6 text-center text-xs text-slate-500">
        Gunakan Nomor ID, kata sandi, dan token resmi yang dibagikan proktor.
      </div>
    </Card>

    <div class="mt-8 text-center text-xs text-slate-600 font-mono">
      Aether CBT v1.0 • Dikembangkan dengan Arsitektur Multi-Tenant SQLite-WAL
    </div>
  </div>
</div>
