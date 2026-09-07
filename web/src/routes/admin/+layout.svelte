<script lang="ts">
  import { authStore } from '$lib/stores/auth';
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { browser } from '$app/environment';
  import { fade, fly } from 'svelte/transition';
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte';

  let loading = true;
  let activeRoute = '';
  let isDrawerOpen = false;
  let showLogoutModal = false;
  let innerWidth = 0;

  // Track active menu route
  $: activeRoute = $page.url.pathname;

  // Auto-close mobile drawer when route changes
  $: if ($page.url.pathname) {
    isDrawerOpen = false;
  }

  // Auto-close mobile drawer when viewport expands to desktop breakpoint (>= 1024px)
  $: if (innerWidth >= 1024 && isDrawerOpen) {
    isDrawerOpen = false;
  }

  // Manage body scroll-lock when mobile drawer is open
  $: if (browser) {
    if (isDrawerOpen) {
      document.body.style.overflow = 'hidden';
    } else if (!showLogoutModal) {
      document.body.style.overflow = '';
    }
  }

  onDestroy(() => {
    if (browser) {
      document.body.style.overflow = '';
    }
  });

  function toggleDrawer() {
    isDrawerOpen = !isDrawerOpen;
  }

  function closeDrawer() {
    isDrawerOpen = false;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && isDrawerOpen) {
      event.preventDefault();
      closeDrawer();
    }
  }

  function promptLogout() {
    isDrawerOpen = false;
    showLogoutModal = true;
  }

  function handleConfirmLogout() {
    showLogoutModal = false;
    authStore.logout();
    goto('/');
  }

  onMount(() => {
    // Check authentication. A non-admin (or unauthenticated) user is sent to "/" via a full
    // reload so /me can re-route by role — goto('/admin') looped because /admin is itself under
    // this layout (review finding #8, Task 22).
    const unsub = authStore.subscribe((state) => {
      if (!state.isAuthenticated) {
        const storedToken = localStorage.getItem('aether_token');
        const storedUser = localStorage.getItem('aether_user');

        if (storedToken && storedUser) {
          try {
            const u = JSON.parse(storedUser);
            if (u.role === 'admin' || u.role === 'superadmin') {
              authStore.login(storedToken, u);
            } else {
              // A logged-in non-admin has no business under /admin — send them home.
              window.location.href = '/';
            }
          } catch {
            window.location.href = '/admin';
          }
        } else if (window.location.pathname !== '/admin') {
          // Not logged in and not on the login page: go to the admin login form.
          window.location.href = '/admin';
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
    { label: 'Paket Soal', path: '/admin/soal-packages', icon: 'M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4' },
    { label: 'Definisi Ujian', path: '/admin/exams', icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4' },
    { label: 'Sesi Ujian', path: '/admin/exam-sessions', icon: 'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z' },
    { label: 'Pengaturan Ujian', path: '/admin/settings', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z' },
    { label: 'Analisis Soal', path: '/admin/results/analysis', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z' },
    { label: 'Manajemen Tenant', path: '/admin/tenants', icon: 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4' }
  ];
</script>

<svelte:window on:keydown={handleKeydown} bind:innerWidth />

{#if loading}
  <div class="min-h-dvh bg-slate-50 flex items-center justify-center text-slate-400 gap-3">
    <svg class="animate-spin h-8 w-8 text-cobalt-600" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
    <p class="text-sm font-semibold text-slate-600">Memuat data panel admin...</p>
  </div>
{:else}
  <div class="min-h-dvh bg-slate-50 flex">
    <!-- Desktop Persistent Sidebar (>= lg) -->
    {#if $authStore.isAuthenticated}
      <aside class="hidden lg:flex w-64 bg-slate-900 text-white flex-col justify-between shadow-xl shrink-0 z-20 sticky top-0 h-dvh border-r border-slate-800 print:hidden">
        <div class="flex-1 overflow-y-auto">
          <!-- Sidebar Brand Header -->
          <div class="h-16 flex items-center px-6 border-b border-slate-800 bg-slate-900/50">
            <div class="flex items-center gap-3">
              <span class="text-lg font-bold tracking-tight text-white font-display">AETHER CBT</span>
              <span class="text-[9px] px-2 py-0.5 bg-cobalt-950 text-cobalt-300 border border-cobalt-800/60 rounded font-bold uppercase tracking-wider font-mono">PROKTOR</span>
            </div>
          </div>

          <!-- Navigation Links -->
          <nav class="p-3 space-y-1">
            {#each menus as m}
              {@const isActive = activeRoute === m.path}
              <a 
                href={m.path}
                class="flex items-center gap-3 px-3.5 py-2.5 rounded-xl font-medium text-sm transition-colors duration-150
                  {isActive ? 'bg-cobalt-600 text-white shadow-sm font-semibold' : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60'}"
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
        <div class="p-4 border-t border-slate-800 bg-slate-900/60 shrink-0">
          <div class="flex items-center justify-between">
            <div class="flex flex-col text-left">
              <span class="text-[9px] font-bold text-slate-400 uppercase tracking-widest font-mono">Pengguna</span>
              <span class="text-sm font-semibold text-slate-200 truncate max-w-[130px]">
                {$authStore.user?.full_name || $authStore.user?.username || 'Proktor'}
              </span>
            </div>
            
            <button 
              type="button"
              class="text-ruby-400 hover:text-ruby-300 transition-colors p-2 hover:bg-slate-800/60 rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-ruby-500"
              on:click={promptLogout}
              title="Keluar dari Panel Admin"
              aria-label="Keluar dari Panel Admin"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
              </svg>
            </button>
          </div>
        </div>
      </aside>

      <!-- Mobile / Tablet Responsive Drawer & Backdrop (< lg) -->
      {#if isDrawerOpen}
        <!-- Backdrop overlay with smooth fade -->
        <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
        <div 
          class="fixed inset-0 bg-slate-950/70 backdrop-blur-sm z-40 lg:hidden transition-opacity print:hidden"
          on:click={closeDrawer}
          transition:fade={{ duration: 200 }}
          aria-hidden="true"
        ></div>

        <!-- Slide-out Drawer Panel -->
        <aside
          class="fixed inset-y-0 left-0 z-50 w-72 max-w-[85vw] bg-slate-900 text-white flex flex-col justify-between shadow-2xl border-r border-slate-800 lg:hidden h-dvh print:hidden"
          transition:fly={{ x: -288, duration: 250 }}
          aria-label="Menu Navigasi Admin"
        >
          <div class="flex-1 overflow-y-auto">
            <!-- Drawer Header with Brand and Close Button -->
            <div class="h-16 flex items-center justify-between px-5 border-b border-slate-800 bg-slate-900/60">
              <div class="flex items-center gap-2.5">
                <span class="text-lg font-bold tracking-tight text-white font-display">AETHER CBT</span>
                <span class="text-[9px] px-2 py-0.5 bg-cobalt-950 text-cobalt-300 border border-cobalt-800/60 rounded font-bold uppercase tracking-wider font-mono">PROKTOR</span>
              </div>
              <button
                type="button"
                class="p-2 -mr-1 rounded-xl text-slate-400 hover:text-white hover:bg-slate-800/80 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-500"
                on:click={closeDrawer}
                aria-label="Tutup navigasi"
              >
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <!-- Drawer Navigation Links -->
            <nav class="p-3 space-y-1">
              {#each menus as m}
                {@const isActive = activeRoute === m.path}
                <a 
                  href={m.path}
                  class="flex items-center gap-3 px-3.5 py-2.5 rounded-xl font-medium text-sm transition-colors duration-150
                    {isActive ? 'bg-cobalt-600 text-white shadow-sm font-semibold' : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60'}"
                  on:click={closeDrawer}
                >
                  <svg class="h-5 w-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d={m.icon} />
                  </svg>
                  <span>{m.label}</span>
                </a>
              {/each}
            </nav>
          </div>

          <!-- Drawer Footer with User and Logout -->
          <div class="p-4 border-t border-slate-800 bg-slate-900/80 shrink-0">
            <div class="flex items-center justify-between">
              <div class="flex flex-col text-left">
                <span class="text-[9px] font-bold text-slate-400 uppercase tracking-widest font-mono">Pengguna</span>
                <span class="text-sm font-semibold text-slate-200 truncate max-w-[150px]">
                  {$authStore.user?.full_name || $authStore.user?.username || 'Proktor'}
                </span>
              </div>
              
              <button 
                type="button"
                class="text-ruby-400 hover:text-ruby-300 transition-colors p-2 hover:bg-slate-800/60 rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-ruby-500"
                on:click={promptLogout}
                title="Keluar dari Panel Admin"
                aria-label="Keluar dari Panel Admin"
              >
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                </svg>
              </button>
            </div>
          </div>
        </aside>
      {/if}
    {/if}

    <!-- Main Content Container -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Top header bar -->
      {#if $authStore.isAuthenticated}
        <header class="h-16 bg-white border-b border-slate-200 flex items-center justify-between px-4 sm:px-6 lg:px-8 sticky top-0 z-10 shadow-sm shrink-0 print:hidden">
          <div class="flex items-center gap-3">
            <!-- Mobile / Tablet Hamburger Button (< lg) -->
            <button
              type="button"
              class="lg:hidden p-2 -ml-1 rounded-xl text-slate-600 hover:text-slate-900 hover:bg-slate-100 active:bg-slate-200 border border-slate-200 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-cobalt-600"
              on:click={toggleDrawer}
              aria-label="Buka menu navigasi"
              aria-expanded={isDrawerOpen}
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
              </svg>
            </button>

            <span class="text-[11px] px-2.5 py-1 bg-slate-100 text-slate-700 font-bold uppercase tracking-wider rounded-lg border border-slate-200 font-mono">
              TENANT-ID: {$authStore.user?.tenant_id || 1}
            </span>
          </div>

          <div class="text-xs font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-2.5 font-mono">
            <span class="hidden sm:inline">Server CBT Aktif</span>
            <span class="sm:hidden">Aktif</span>
            <span class="relative flex h-2.5 w-2.5">
              <span class="inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500"></span>
            </span>
          </div>
        </header>
      {/if}

      <div class="flex-1 overflow-y-auto">
        <slot />
      </div>
    </div>
  </div>

  <!-- Logout Confirmation Modal Protection -->
  <ConfirmModal
    bind:isOpen={showLogoutModal}
    title="Konfirmasi Keluar Sesi Admin"
    message="Apakah Anda yakin ingin keluar dari panel admin? Sesi aktif Anda akan ditutup dan Anda harus login kembali untuk mengelola ujian."
    confirmLabel="Keluar Sistem"
    cancelLabel="Batal"
    variant="danger"
    theme="light"
    on:confirm={handleConfirmLogout}
    on:cancel={() => { showLogoutModal = false; }}
  />
{/if}
