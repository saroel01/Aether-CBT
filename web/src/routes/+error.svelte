<script lang="ts">
  import { page } from '$app/stores';
  import { browser } from '$app/environment';
  import Button from '$lib/components/ui/Button.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';

  $: status = $page.status || 500;
  $: errorMessage = $page.error?.message || '';

  // Non-technical error categorization & comforting institutional guidance
  $: errorMeta = getErrorMeta(status);

  function getErrorMeta(code: number) {
    if (code === 404) {
      return {
        badgeVariant: 'amber' as const,
        badgeText: 'GALAT 404 • TIDAK DITEMUKAN',
        title: 'Halaman Tidak Ditemukan',
        description: 'Tautan atau halaman yang Anda tuju tidak tersedia, telah dipindahkan, atau alamat URL yang dimasukkan kurang tepat. Pastikan alamat URL sudah benar.',
        iconColor: 'bg-amber-50 text-amber-600 border-amber-200',
        type: '404'
      };
    } else if (code === 403) {
      return {
        badgeVariant: 'ruby' as const,
        badgeText: 'GALAT 403 • AKSES DIBATASI',
        title: 'Akses Tidak Diizinkan',
        description: 'Anda tidak memiliki hak akses atau sesi login yang memadai untuk membuka direktori ini. Silakan periksa akun dan peran pengguna Anda.',
        iconColor: 'bg-ruby-50 text-ruby-600 border-ruby-200',
        type: '403'
      };
    } else if (code >= 500) {
      return {
        badgeVariant: 'ruby' as const,
        badgeText: `GALAT ${code} • KENDALA SISTEM`,
        title: 'Kendala Server Sementara',
        description: 'Sistem Aether CBT sedang mengalami kendala pemrosesan di server. Data jawaban ujian dan sesi Anda tetap tersimpan dengan aman di database.',
        iconColor: 'bg-ruby-50 text-ruby-600 border-ruby-200',
        type: '500'
      };
    } else {
      return {
        badgeVariant: 'slate' as const,
        badgeText: `GALAT ${code}`,
        title: 'Terjadi Kendala Teknis',
        description: 'Aplikasi mengalami kendala yang tidak terduga. Silakan gunakan tombol pemulihan di bawah ini untuk kembali ke halaman sebelumnya.',
        iconColor: 'bg-slate-100 text-slate-600 border-slate-200',
        type: 'generic'
      };
    }
  }

  function handleGoBack() {
    if (browser) {
      if (window.history.length > 1) {
        window.history.back();
      } else {
        window.location.href = '/';
      }
    }
  }
</script>

<svelte:head>
  <title>Galat {status}: {errorMeta.title} - Aether CBT</title>
  <meta name="robots" content="noindex, nofollow" />
</svelte:head>

<div class="min-h-dvh flex flex-col justify-between bg-slate-50 text-slate-900 select-none">
  <!-- Minimal Institutional Header -->
  <header class="bg-white border-b border-slate-200 px-6 py-4 shadow-sm">
    <div class="max-w-6xl mx-auto flex items-center justify-between">
      <div class="flex items-center gap-2.5">
        <a href="/" class="flex items-center gap-2 text-slate-900 hover:text-cobalt-600 transition-colors">
          <span class="text-lg font-extrabold tracking-tight font-display">AETHER <span class="text-cobalt-600">CBT</span></span>
        </a>
        <span class="text-[10px] px-2 py-0.5 bg-slate-100 text-slate-600 border border-slate-200 rounded font-bold uppercase tracking-wider font-mono">
          Status Galat
        </span>
      </div>

      <div class="text-xs text-slate-400 font-mono tracking-wider tabular-nums">
        HTTP {status}
      </div>
    </div>
  </header>

  <!-- Centered Error Card -->
  <main class="flex-1 flex items-center justify-center p-4 sm:p-6 my-auto">
    <div class="w-full max-w-lg bg-white border border-slate-200/80 rounded-2xl shadow-sm p-6 sm:p-10 text-center">
      <!-- Status Badge -->
      <div class="mb-5">
        <Badge variant={errorMeta.badgeVariant} theme="light" class="text-xs font-mono font-bold tracking-wider px-3 py-1">
          {errorMeta.badgeText}
        </Badge>
      </div>

      <!-- Icon Container -->
      <div class="w-16 h-16 mx-auto rounded-2xl flex items-center justify-center border mb-6 shadow-sm {errorMeta.iconColor}">
        {#if errorMeta.type === '404'}
          <svg class="h-8 w-8 stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        {:else if errorMeta.type === '403'}
          <svg class="h-8 w-8 stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
        {:else}
          <svg class="h-8 w-8 stroke-current" fill="none" viewBox="0 0 24 24" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        {/if}
      </div>

      <!-- Error Title -->
      <h1 class="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-display mb-3 text-balance">
        {errorMeta.title}
      </h1>

      <!-- Non-technical Description -->
      <p class="text-sm text-slate-500 leading-relaxed max-w-md mx-auto mb-6 text-pretty">
        {errorMeta.description}
      </p>

      <!-- Institutional Reassurance Banner for CBT Students -->
      <div class="bg-cobalt-50/70 border border-cobalt-100 rounded-xl p-3.5 mb-6 text-left flex items-start gap-3">
        <svg class="h-5 w-5 text-cobalt-600 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <div class="text-xs text-slate-700 leading-relaxed text-pretty">
          <strong class="text-cobalt-900 font-semibold block mb-0.5">Panduan Peserta Ujian:</strong>
          Jika halaman ini muncul saat ujian berlangsung, jangan panik dan jangan menutup jendela peramban. Jawaban Anda tersimpan. Hubungi pengawas ruang untuk petunjuk lanjutan.
        </div>
      </div>

      <!-- Technical Details Fold (Hidden by default for clean presentation) -->
      {#if errorMessage && errorMessage !== errorMeta.title}
        <details class="text-left mb-6 border border-slate-200 rounded-xl bg-slate-50/60 p-3 text-xs">
          <summary class="font-semibold text-slate-600 cursor-pointer select-none outline-none focus:text-cobalt-600">
            Detail Teknis Sistem
          </summary>
          <div class="mt-2.5 pt-2.5 border-t border-slate-200/80 font-mono text-[11px] text-slate-700 break-all bg-white p-2.5 rounded-lg border border-slate-100 shadow-inner">
            {errorMessage}
          </div>
        </details>
      {/if}

      <!-- Recovery Buttons -->
      <div class="flex flex-col sm:flex-row gap-3 justify-center items-center">
        <Button 
          variant="primary" 
          size="md" 
          theme="light" 
          class="w-full sm:w-auto font-semibold shadow-sm" 
          on:click={handleGoBack}
        >
          Kembali ke Halaman Sebelumnya
        </Button>
        <a href="/" class="w-full sm:w-auto">
          <Button 
            variant="secondary" 
            size="md" 
            theme="light" 
            class="w-full font-semibold shadow-sm"
          >
            Ke Beranda Ujian
          </Button>
        </a>
      </div>

      <!-- Quick Portal Jump Links -->
      <div class="mt-8 pt-6 border-t border-slate-100 flex flex-wrap items-center justify-center gap-4 text-xs font-medium text-slate-500">
        <span>Tautan Langsung:</span>
        <a href="/student/login" class="text-cobalt-600 hover:underline hover:text-cobalt-700">Portal Siswa</a>
        <span class="text-slate-300">•</span>
        <a href="/supervisor/login" class="text-cobalt-600 hover:underline hover:text-cobalt-700">Portal Pengawas</a>
        <span class="text-slate-300">•</span>
        <a href="/admin" class="text-cobalt-600 hover:underline hover:text-cobalt-700">Panel Admin</a>
      </div>
    </div>
  </main>

  <!-- Institutional Footer -->
  <footer class="bg-white border-t border-slate-200 py-4 px-6 text-center text-xs text-slate-400 font-mono">
    Aether CBT • Sovereign Computer-Based Testing System
  </footer>
</div>
