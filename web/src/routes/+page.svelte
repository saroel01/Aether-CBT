<script lang="ts">
  // Simple landing page for CBT portal
</script>

<svelte:head>
  <title>Portal Aether CBT - Sistem Ujian Berbasis Komputer</title>
  <meta name="description" content="Portal Computer-Based Testing Multi-Tenant Berkinerja Tinggi" />
</svelte:head>

<div class="min-h-screen flex flex-col justify-between bg-slate-950 bg-grid-sovereign text-slate-100 relative overflow-hidden select-none">
  <!-- Subtle top glow -->
  <div class="absolute top-0 left-1/2 -translate-x-1/2 w-[500px] h-[350px] bg-indigo-500/10 rounded-full blur-[120px] pointer-events-none"></div>

  <!-- Header -->
  <header class="max-w-6xl w-full mx-auto px-6 py-6 flex justify-between items-center z-10">
    <div class="flex items-center gap-2">
      <span class="text-lg font-bold tracking-tight text-slate-200 font-display">AETHER <span class="text-indigo-500">CBT</span></span>
    </div>
    <div class="text-xs text-slate-500 font-mono">v1.0.0-hardened</div>
  </header>

  <!-- Hero Content -->
  <main class="flex-1 flex items-center justify-center px-6 z-10">
    <div class="text-center max-w-2xl">
      <!-- Badge -->
      <span class="inline-flex items-center px-3.5 py-1 rounded-full text-xs font-semibold bg-indigo-950/40 text-indigo-400 border border-indigo-900/30 mb-8 tracking-wider uppercase font-display">
        Sovereign Testing Environment
      </span>

      <!-- Title -->
      <h1 class="text-5xl md:text-7xl font-extrabold tracking-tight text-white mb-4 font-display">
        Aether CBT
      </h1>
      
      <!-- Subtitle -->
      <p class="text-lg md:text-xl text-slate-400 font-medium mb-12 max-w-lg mx-auto leading-relaxed">
        Computer-Based Testing Platform dengan performa tinggi, keamanan berlapis, dan tata letak yang menenangkan.
      </p>

      <!-- Navigation links as custom premium buttons -->
      <div class="flex flex-col sm:flex-row gap-4 justify-center items-center">
        <a 
          href="/student/login" 
          class="w-full sm:w-auto px-8 py-3.5 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl font-semibold shadow-lg shadow-indigo-600/10 border border-indigo-600/30 text-center transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] hover:scale-[1.02] active:scale-[0.98]"
        >
          Portal Login Siswa
        </a>
        
        <a 
          href="/supervisor/login" 
          class="w-full sm:w-auto px-8 py-3.5 bg-slate-900 hover:bg-slate-900/80 text-slate-300 rounded-2xl font-semibold border border-slate-800 text-center transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] hover:scale-[1.02] active:scale-[0.98]"
        >
          Masuk Proktor / Pengawas
        </a>

        <a 
          href="/admin" 
          class="w-full sm:w-auto px-8 py-3.5 bg-transparent hover:bg-slate-900/50 text-slate-400 hover:text-slate-200 rounded-2xl font-semibold border border-transparent hover:border-slate-800 text-center transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] hover:scale-[1.02] active:scale-[0.98]"
        >
          Admin Panel
        </a>
      </div>
    </div>
  </main>

  <!-- Footer -->
  <footer class="max-w-6xl w-full mx-auto px-6 py-8 flex flex-col sm:flex-row justify-between items-center gap-4 text-xs text-slate-600 border-t border-slate-900/50 z-10">
    <div>
      © 2026 Aether CBT. Hak Cipta Dilindungi.
    </div>
    <div class="flex gap-4">
      <a href="https://github.com/saroel01/aether-cbt" class="hover:text-slate-400 transition-colors underline">Dokumentasi</a>
      <span>•</span>
      <span>Multi-Tenant SQLite-WAL Infrastructure</span>
    </div>
  </footer>
</div>
