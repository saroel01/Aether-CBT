<script lang="ts">
  import { api } from '$lib/api';
  import { authStore } from '$lib/stores/auth';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import { toast } from '$lib/stores/toast';

  let username = 'ruang_a';
  let password = 'ruang123';
  let error = '';
  let loading = false;

  async function login() {
    if (!username || !password) {
      toast.error('Username dan password harus diisi!');
      return;
    }

    error = '';
    loading = true;
    try {
      const res = await api('/auth/supervisor-login', {
        method: 'POST',
        body: JSON.stringify({ username, password })
      });

      if (res.success && res.data?.token) {
        // Save using authStore so unified API client grabs it!
        authStore.login(res.data.token, {
          id: res.data.ruang_id,
          username: username,
          role: 'supervisor',
          full_name: res.data.room_name,
          tenant_id: 1 // default
        });

        toast.success(`Selamat datang, Pengawas ${res.data.room_name}!`);
        setTimeout(() => {
          window.location.href = '/supervisor';
        }, 800);
      }
    } catch (e: any) {
      error = e.message || 'Login gagal. Silakan periksa kembali username dan password.';
      toast.error(error);
    }
    loading = false;
  }
</script>

<svelte:head>
  <title>Login Pengawas Ruang - Aether CBT</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-900 via-slate-950 to-indigo-950 px-4">
  <div class="w-full max-w-md">
    <div class="text-center mb-8">
      <h1 class="text-4xl font-extrabold text-white tracking-tight mb-2">Aether CBT</h1>
      <p class="text-slate-400 text-sm">Panel Pengawas Ruangan Ujian</p>
    </div>

    <Card padding="lg" class="border-slate-800 bg-slate-900/60 backdrop-blur-md text-white shadow-2xl relative overflow-hidden">
      <!-- Top aesthetic accent line -->
      <div class="absolute top-0 left-0 w-full h-[3px] bg-gradient-to-r from-teal-500 to-indigo-500"></div>

      <div class="mb-6">
        <h2 class="text-2xl font-bold text-white text-center">Masuk Pengawas</h2>
        <p class="text-center text-xs text-slate-400 mt-1">Gunakan username dan sandi ruangan Anda</p>
      </div>

      <div class="space-y-4">
        <Input 
          id="username"
          label="Username Ruang" 
          placeholder="Contoh: ruang_a" 
          bind:value={username}
          disabled={loading}
          class="text-slate-800"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
            </svg>
          </span>
        </Input>

        <Input 
          id="password"
          label="Password Ruang" 
          type="password" 
          placeholder="Masukkan password ruangan" 
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
            class="w-full bg-gradient-to-r from-teal-600 to-indigo-600 border-none font-semibold text-base py-3.5 hover:scale-[1.02]" 
            on:click={login}
            {loading}
          >
            {loading ? 'Mengotentikasi...' : 'Masuk sebagai Pengawas'}
          </Button>
        </div>
      </div>

      <div class="mt-6 text-center text-xs text-slate-500">
        Default: <span class="font-mono text-teal-400">ruang_a</span> / <span class="font-mono text-teal-400">ruang123</span>
      </div>
    </Card>

    <div class="mt-8 text-center text-xs text-slate-600">
      Aether CBT v1.0 • Panel Pengawasan Ruangan Terisolasi Mandiri
    </div>
  </div>
</div>
