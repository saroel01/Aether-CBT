<script lang="ts">
  import { api } from '$lib/api';
  import { authStore } from '$lib/stores/auth';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import { toast } from '$lib/stores/toast';

  let username = import.meta.env.DEV ? 'ruang_a' : '';
  let password = import.meta.env.DEV ? 'ruang123' : '';
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
        body: JSON.stringify({ username, password }),
        raw401: true
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
  <meta name="description" content="Masuk ke portal pengawas ruangan Aether CBT" />
</svelte:head>

<div class="min-h-dvh flex items-center justify-center bg-slate-950 bg-grid-sovereign px-4 py-8 relative overflow-hidden select-none">
  <div class="w-full max-w-md z-10">
    <!-- Academic Emblem -->
    <div class="text-center mb-8">
      <div class="mb-5 flex justify-center">
        <div class="h-14 w-14 rounded-2xl bg-slate-900 border border-slate-800 flex items-center justify-center shadow-sm">
          <svg class="h-7 w-7 text-cobalt-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            <path stroke-linecap="round" stroke-linejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
          </svg>
        </div>
      </div>
      <h1 class="text-3xl font-extrabold text-slate-100 tracking-tight font-display">Aether <span class="text-cobalt-400 font-bold">CBT</span></h1>
      <p class="text-slate-400 text-xs font-semibold uppercase tracking-wider mt-1.5 font-mono">Supervisor Portal</p>
    </div>

    <!-- Elevated Slate Card -->
    <Card theme="dark" padding="lg" class="shadow-sm border-slate-800 bg-slate-900/90 rounded-2xl">
      <div class="mb-6 text-center">
        <h2 class="text-xl font-bold text-slate-100 font-display">Masuk Pengawas</h2>
        <p class="text-xs text-slate-400 mt-1">Gunakan username dan sandi ruangan Anda</p>
      </div>

      <form on:submit|preventDefault={login} class="space-y-4">
        <!-- Username Input -->
        <Input 
          id="username"
          label="Username Ruang" 
          placeholder="Contoh: ruang_a" 
          bind:value={username}
          disabled={loading}
          theme="dark"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
            </svg>
          </span>
        </Input>

        <!-- Password Input -->
        <Input 
          id="password"
          label="Password Ruang" 
          type="password" 
          placeholder="Masukkan password ruangan" 
          bind:value={password}
          disabled={loading}
          theme="dark"
        >
          <span slot="iconLeft">
            <svg class="h-5 w-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </span>
        </Input>

        {#if error}
          <div class="p-3 bg-ruby-950/40 border border-ruby-800/60 rounded-xl text-ruby-300 text-xs font-semibold text-center transition-colors duration-150">
            {error}
          </div>
        {/if}

        <div class="pt-2">
          <Button 
            type="submit" 
            variant="primary" 
            size="lg" 
            class="w-full font-semibold" 
            {loading}
          >
            {loading ? 'Mengotentikasi...' : 'Masuk sebagai Pengawas'}
          </Button>
        </div>
      </form>

      {#if import.meta.env.DEV}
        <div class="mt-6 text-center text-xs text-slate-400">
          Dev only: kolom kredensial sudah terisi otomatis — langsung klik Masuk.
        </div>
      {/if}
    </Card>

    <div class="mt-8 text-center text-xs text-slate-400 font-mono">
      Aether CBT v1.0 • Panel Pengawasan Ruangan Terisolasi Mandiri
    </div>
  </div>
</div>
