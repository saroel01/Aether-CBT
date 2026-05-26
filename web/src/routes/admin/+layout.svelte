<script lang="ts">
  import { authStore } from '$lib/stores/auth';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import Button from '$lib/components/ui/Button.svelte';

  let loading = true;
  let activeRoute = '';

  // Subscribe to page stores to track active menu items
  $: activeRoute = $page.url.pathname;

  onMount(() => {
    // Check authentication
    const unsub = authStore.subscribe((state) => {
      if (!state.isAuthenticated) {
        const storedToken = localStorage.getItem('aether_token');
        const storedUser = localStorage.getItem('aether_user');
        
        if (storedToken && storedUser) {
          const u = JSON.parse(storedUser);
          if (u.role === 'admin') {
            authStore.login(storedToken, u);
          } else {
            goto('/admin');
          }
        } else {
          // If we are not on the main /admin login page, redirect to login
          if (window.location.pathname !== '/admin') {
            goto('/admin');
          }
        }
      }
      loading = false;
    });
    return unsub;
  });

  const menus = [
    { label: 'Dashboard', path: '/admin', icon: 'M4 6a2 2 0 012-2h2a2 2 0 012 2v4a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v4a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v4a2 2 0 01-2 2H6a2 2 0 01-2-2v-4zM14 16a2 2 0 012-2h2a2 2 0 012 2v4a2 2 0 01-2 2h-2a2 2 0 01-2-2v-4z' },
    { label: 'Peserta Ujian', path: '/admin/students', icon: 'M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z' },
    { label: 'Kelas', path: '/admin/classes', icon: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10' },
    { label: 'Mata Pelajaran', path: '/admin/mapel', icon: 'M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253' },
    { label: 'Ruang Ujian', path: '/admin/rooms', icon: 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4' },
    { label: 'Pengaturan Ujian', path: '/admin/settings', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z' },
    { label: 'Analisis Soal', path: '/admin/results/analysis', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z' },
    { label: 'Manajemen Tenant', path: '/admin/tenants', icon: 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4' }
  ];
</script>

{#if loading}
  <div class="min-h-screen bg-slate-50 flex items-center justify-center text-slate-400 gap-3">
    <svg class="animate-spin h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
    <p class="text-sm font-semibold">Memuat data panel admin...</p>
  </div>
{:else}
  <div class="min-h-screen bg-slate-100/50 flex">
    <!-- Left Sidebar (Collapsible/Fixed) -->
    {#if $authStore.isAuthenticated}
      <aside class="w-64 bg-slate-950 text-white flex flex-col justify-between shadow-xl shrink-0 z-10 sticky top-0 h-screen border-r border-slate-900">
        <div>
          <!-- Sidebar Brand Header -->
          <div class="h-16 flex items-center px-6 border-b border-slate-900 bg-slate-950/20">
            <div class="flex items-center gap-3">
              <span class="text-lg font-bold tracking-tight text-slate-200 font-display">AETHER CBT</span>
              <span class="text-[9px] px-2 py-0.5 bg-indigo-950 text-indigo-400 border border-indigo-900/60 rounded font-bold uppercase tracking-wider font-mono">PROKTOR</span>
            </div>
          </div>

          <!-- Navigation Links -->
          <nav class="p-4 space-y-1.5">
            {#each menus as m}
              {@const isActive = activeRoute === m.path}
              <a 
                href={m.path}
                class="flex items-center gap-3 px-4 py-3 rounded-2xl font-semibold text-sm transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]
                  {isActive ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-950/50 scale-[1.02]' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/50'}"
              >
                <svg class="h-5 w-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d={m.icon} />
                </svg>
                <span>{m.label}</span>
              </a>
            {/each}
          </nav>
        </div>

        <!-- Sidebar footer profiles -->
        <div class="p-4 border-t border-slate-900 bg-slate-950/40">
          <div class="flex items-center justify-between">
            <div class="flex flex-col text-left">
              <span class="text-[9px] font-bold text-slate-500 uppercase tracking-widest font-mono">Pengguna</span>
              <span class="text-sm font-bold text-slate-200 truncate max-w-[130px]">
                {$authStore.user?.full_name || $authStore.user?.username || 'Proktor'}
              </span>
            </div>
            
            <button 
              type="button"
              class="text-red-400 hover:text-red-300 transition-colors p-2 hover:bg-slate-900/50 rounded-xl"
              on:click={() => { authStore.logout(); goto('/'); }}
              title="Logout"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
              </svg>
            </button>
          </div>
        </div>
      </aside>
    {/if}

    <!-- Main Content Container -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Top header bar -->
      {#if $authStore.isAuthenticated}
        <header class="h-16 bg-white border-b border-slate-100 flex items-center justify-between px-8 sticky top-0 z-10 shadow-sm shadow-slate-100/50 shrink-0">
          <div class="flex items-center gap-2">
            <span class="text-[10px] px-2.5 py-0.5 bg-indigo-50 text-indigo-700 font-bold uppercase tracking-wider rounded-full border border-indigo-100 font-mono">
              TENANT-ID: {$authStore.user?.tenant_id || 1}
            </span>
          </div>

          <div class="text-xs font-bold text-slate-400 uppercase tracking-widest flex items-center gap-3 font-mono">
            <span>Server CBT Aktif</span>
            <span class="relative flex h-2 w-2">
              <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
            </span>
          </div>
        </header>
      {/if}

      <div class="flex-1 overflow-y-auto">
        <slot />
      </div>
    </div>
  </div>
{/if}
