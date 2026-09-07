<script lang="ts">
  // Simple landing page for CBT portal
</script>

<svelte:head>
  <title>Portal Aether CBT - Sistem Ujian Berbasis Komputer</title>
  <meta name="description" content="Portal Computer-Based Testing Multi-Tenant Berkinerja Tinggi" />
</svelte:head>

<div class="min-h-dvh flex flex-col justify-between bg-slate-950 bg-grid-sovereign text-slate-100 relative overflow-hidden">
  <!-- Header -->
  <header class="max-w-6xl w-full mx-auto px-6 py-6 flex justify-between items-center z-10">
    <div class="flex items-center gap-2">
      <span class="text-lg font-bold tracking-tight text-slate-200 font-display">AETHER <span class="text-cobalt-400">CBT</span></span>
    </div>
    <div class="text-xs text-slate-400 font-mono tracking-wider">v1.0.0-hardened</div>
  </header>

  <!-- Hero Content -->
  <main class="flex-1 flex items-center justify-center px-6 z-10">
    <div class="text-center max-w-2xl">
      <!-- Badge -->
      <span class="inline-flex items-center px-3.5 py-1 rounded-full text-xs font-semibold bg-cobalt-950/50 text-cobalt-400 border border-cobalt-800/50 mb-8 tracking-wider uppercase font-sans">
        Sovereign Testing Environment
      </span>

      <!-- Title -->
      <h1 class="text-5xl md:text-7xl font-extrabold tracking-tight text-white mb-4 font-display text-balance">
        Aether CBT
      </h1>
      
      <!-- Subtitle -->
      <p class="text-lg md:text-xl text-slate-400 font-normal mb-12 max-w-xl mx-auto leading-relaxed text-pretty">
        Computer-Based Testing Platform dengan performa tinggi, keamanan berlapis, dan tata letak yang menenangkan.
      </p>

      <!-- Navigation links as custom precision buttons -->
      <div class="flex flex-col sm:flex-row gap-4 justify-center items-center">
        <a 
          href="/student/login" 
          class="w-full sm:w-auto px-7 py-3.5 bg-cobalt-600 hover:bg-cobalt-700 active:bg-cobalt-800 text-white rounded-xl font-semibold shadow-sm border border-cobalt-500/30 text-center transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-500 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-950"
        >
          Portal Login Siswa
        </a>
        
        <a 
          href="/supervisor/login" 
          class="w-full sm:w-auto px-7 py-3.5 bg-slate-900 hover:bg-slate-800 active:bg-slate-800 text-slate-200 rounded-xl font-semibold shadow-sm border border-slate-800 text-center transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-500 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-950"
        >
          Masuk Proktor / Pengawas
        </a>

        <a 
          href="/admin" 
          class="w-full sm:w-auto px-7 py-3.5 bg-slate-900/40 hover:bg-slate-900 active:bg-slate-800 text-slate-300 hover:text-white rounded-xl font-semibold border border-slate-800 text-center transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-500 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-950"
        >
          Admin Panel
        </a>
      </div>
    </div>
  </main>

  <!-- Footer -->
  <footer class="max-w-6xl w-full mx-auto px-6 py-8 flex flex-col sm:flex-row justify-between items-center gap-4 text-xs text-slate-400 border-t border-slate-900 z-10">
    <div>
      © 2026 Aether CBT. Hak Cipta Dilindungi.
    </div>
    <div class="flex items-center gap-3">
      <a href="https://github.com/saroel01/aether-cbt" class="hover:text-slate-300 transition-colors underline underline-offset-4">Dokumentasi</a>
      <span class="text-slate-700">•</span>
      <span>Multi-Tenant SQLite-WAL Infrastructure</span>
    </div>
  </footer>
</div>
