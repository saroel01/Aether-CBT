<script lang="ts">
  import { onMount } from 'svelte';

  let studentLoginHref = '/student/login';

  onMount(() => {
    if (typeof window !== 'undefined' && window.location.search) {
      const search = window.location.search;
      const params = new URLSearchParams(search);
      if (params.has('token') || params.has('no_id')) {
        studentLoginHref = `/student/login${search}`;
        window.location.replace(`/student/login${search}`);
      }
    }
  });
</script>

<svelte:head>
  <title>Aether CBT</title>
  <meta name="description" content="Pilih portal untuk masuk ke Aether CBT." />
  <meta name="theme-color" content="#020617" />
</svelte:head>

<a href="#konten" class="sr-only focus:not-sr-only focus:absolute focus:top-3 focus:left-3 focus:z-50 focus:px-4 focus:py-2 focus:rounded-lg focus:bg-cobalt-600 focus:text-white focus:text-sm focus:font-semibold">
  Lewati ke konten utama
</a>

<div class="min-h-dvh flex flex-col bg-slate-950 text-slate-100 relative overflow-hidden bg-noise">
  <div class="absolute inset-0 bg-grid-sovereign" aria-hidden="true"></div>
  <div class="absolute -top-40 left-1/2 -translate-x-1/2 w-[720px] h-[420px] rounded-full bg-cobalt-600/20 blur-[120px] pointer-events-none" aria-hidden="true"></div>
  <div class="absolute top-1/3 -left-40 w-[420px] h-[420px] rounded-full bg-cobalt-900/40 blur-[100px] pointer-events-none" aria-hidden="true"></div>

  <header class="relative z-10 w-full max-w-6xl mx-auto px-6 pt-6 flex items-center justify-center">
    <span class="flex items-center gap-2.5" aria-label="Aether CBT">
      <span class="w-8 h-8 rounded-lg bg-cobalt-600 shadow-glow-cobalt flex items-center justify-center text-white font-extrabold font-display text-sm" aria-hidden="true">Æ</span>
      <span class="text-[17px] font-bold tracking-tight text-slate-100 font-display">AETHER <span class="text-cobalt-400">CBT</span></span>
    </span>
  </header>

  <main id="konten" class="relative z-10 flex-1 w-full max-w-6xl mx-auto px-6 flex items-center justify-center">
    <div class="w-full max-w-xl mx-auto text-center py-16">
      <h1 class="animate-rise animate-rise-1 text-5xl sm:text-6xl font-extrabold tracking-tight text-white font-display text-balance">
        Aether CBT
      </h1>
      <p class="animate-rise animate-rise-2 mt-4 text-base text-slate-400">
        Pilih portal untuk masuk.
      </p>

      <div class="animate-rise animate-rise-3 mt-10 flex flex-col gap-3 w-full max-w-sm mx-auto" role="group" aria-label="Pilih portal masuk">
        <a
          href={studentLoginHref}
          class="group flex items-center gap-4 px-5 py-4 bg-cobalt-600 hover:bg-cobalt-500 active:scale-[0.98] text-white rounded-2xl font-semibold shadow-glow-cobalt transition-premium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-400 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-950"
        >
          <span class="w-10 h-10 rounded-xl bg-white/15 border border-white/20 flex items-center justify-center shrink-0" aria-hidden="true">
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M12 14l9-5-9-5-9 5 9 5zm0 0v6" /></svg>
          </span>
          <span class="flex-1 text-left text-[15px]">Siswa</span>
          <svg class="w-4 h-4 opacity-70 transition-transform group-hover:translate-x-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 6l6 6-6 6" />
          </svg>
        </a>
        <a
          href="/supervisor/login"
          class="group flex items-center gap-4 px-5 py-4 bg-slate-900 hover:bg-slate-800 hover:border-slate-700 active:scale-[0.98] text-slate-100 rounded-2xl font-semibold border border-slate-800 transition-premium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-500 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-950"
        >
          <span class="w-10 h-10 rounded-xl bg-slate-800 border border-slate-700 flex items-center justify-center shrink-0" aria-hidden="true">
            <svg class="w-5 h-5 text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /><path stroke-linecap="round" stroke-linejoin="round" d="M2.05 12c1.5-3.5 5-6 9.95-6s8.45 2.5 9.95 6c-1.5 3.5-5 6-9.95 6s-8.45-2.5-9.95-6z" /></svg>
          </span>
          <span class="flex-1 text-left text-[15px]">Pengawas</span>
          <svg class="w-4 h-4 text-slate-500 transition-transform group-hover:translate-x-1 group-hover:text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 6l6 6-6 6" />
          </svg>
        </a>
        <a
          href="/admin"
          class="group flex items-center gap-4 px-5 py-4 bg-slate-900/60 hover:bg-slate-900 hover:border-slate-700 active:scale-[0.98] text-slate-200 rounded-2xl font-semibold border border-slate-800/80 transition-premium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-500 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-950"
        >
          <span class="w-10 h-10 rounded-xl bg-slate-800/60 border border-slate-700/60 flex items-center justify-center shrink-0" aria-hidden="true">
            <svg class="w-5 h-5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.6-4A11.9 11.9 0 0112 3a11.9 11.9 0 01-8.6 3.1A12 12 0 003 9c0 5.6 3.8 10.3 9 11.6 5.2-1.3 9-6 9-11.6 0-1-.1-2-.4-3z" /></svg>
          </span>
          <span class="flex-1 text-left text-[15px]">Admin</span>
          <svg class="w-4 h-4 text-slate-600 transition-transform group-hover:translate-x-1 group-hover:text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 6l6 6-6 6" />
          </svg>
        </a>
      </div>
    </div>
  </main>

  <footer class="relative z-10 w-full max-w-6xl mx-auto px-6 pb-8">
    <p class="pt-6 border-t border-slate-900 text-center text-xs text-slate-600">
      © 2026 Aether CBT
    </p>
  </footer>
</div>
