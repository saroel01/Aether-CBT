<script lang="ts">
  import { api } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import Table from '$lib/components/ui/Table.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import { toast } from '$lib/stores/toast';

  let items: any[] = [];
  let loading = true;
  let showAddModal = false;

  // New Tenant Form State
  let newName = '';
  let newSlug = '';
  let createLoading = false;

  onMount(async () => {
    await loadTenants();
  });

  async function loadTenants() {
    loading = true;
    try {
      const res = await api('/tenants');
      if (res.success) {
        items = res.data || [];
      } else {
        throw new Error(res.error || 'Terjadi kesalahan sistem');
      }
    } catch (e: any) {
      toast.error('Gagal mengambil data tenant: ' + e.message);
    }
    loading = false;
  }

  async function createTenant() {
    if (!newName || !newSlug) {
      toast.warning('Nama Sekolah dan Slug Domain wajib diisi!');
      return;
    }

    createLoading = true;
    try {
      const res = await api('/tenants', {
        method: 'POST',
        body: JSON.stringify({ name: newName, slug: newSlug.toLowerCase().trim() })
      });
      if (res.success) {
        toast.success(`Tenant sekolah "${newName}" berhasil terdaftar!`);
        newName = '';
        newSlug = '';
        showAddModal = false;
        await loadTenants();
      } else {
        throw new Error(res.error);
      }
    } catch (e: any) {
      toast.error('Gagal mendaftarkan sekolah: ' + e.message);
    }
    createLoading = false;
  }
</script>

<svelte:head>
  <title>Manajemen Tenant Sekolah - Superadmin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b pb-6">
    <div>
      <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Multi-Tenant Sekolah</h1>
      <p class="text-slate-500 text-sm">Kelola partisi pangkalan data, slug domain, dan status aktifasi sekolah/tenant.</p>
    </div>

    <Button 
      variant="primary" 
      size="md" 
      theme="light"
      on:click={() => showAddModal = true}
    >
      Daftarkan Sekolah Baru
    </Button>
  </div>

  {#if loading}
    <div class="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
      <svg class="animate-spin h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
      <p class="text-sm font-semibold">Memuat daftar sekolah...</p>
    </div>
  {:else}
    <Table>
      <thead>
        <tr>
          <th class="w-20">ID</th>
          <th>Nama Sekolah (Tenant)</th>
          <th>Slug Domain</th>
          <th>Status</th>
          <th>Terdaftar Pada</th>
        </tr>
      </thead>
      <tbody>
        {#each items as t}
          <tr>
            <td class="font-mono text-slate-400 font-bold">{t.id}</td>
            <td class="font-semibold text-slate-800">{t.name}</td>
            <td>
              <Badge theme="light" variant="info" class="font-mono">{t.slug}</Badge>
            </td>
            <td>
              {#if t.is_active}
                <Badge variant="success" theme="light" class="flex items-center gap-1.5 w-fit">
                  <span class="relative flex h-1.5 w-1.5 shrink-0">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500"></span>
                  </span>
                  Aktif
                </Badge>
              {:else}
                <Badge variant="neutral" theme="light" class="flex items-center gap-1.5 w-fit">
                  <span class="h-1.5 w-1.5 rounded-full bg-slate-400 shrink-0"></span>
                  Non-Aktif
                </Badge>
              {/if}
            </td>
            <td class="text-xs text-slate-400 font-mono">
              {new Date(t.created_at).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' })}
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="5" class="text-center py-16 text-slate-400 font-medium">
              Belum ada tenant sekolah terdaftar. Klik "Daftarkan Sekolah Baru" untuk menambahkan sekolah pertama Anda.
            </td>
          </tr>
        {/each}
      </tbody>
    </Table>
  {/if}

  <!-- Add Tenant Modal -->
  <Modal show={showAddModal} title="Daftarkan Sekolah Baru" size="md">
    <div class="space-y-4 text-slate-800">
      <p class="text-sm text-slate-500 leading-relaxed">
        Pendaftaran sekolah baru akan secara otomatis mengisolasi pangkalan data, proktor, siswa, ruangan, mata pelajaran, dan seluruh konfigurasi khusus miliknya.
      </p>

      <Input 
        id="tenant_name"
        label="Nama Resmi Sekolah *" 
        placeholder="Contoh: SMAN 1 Kluet Selatan" 
        bind:value={newName}
        disabled={createLoading}
        theme="light"
      />

      <Input 
        id="tenant_slug"
        label="Slug Domain (Subdomain/Path) *" 
        placeholder="Contoh: sman1kluet" 
        bind:value={newSlug}
        disabled={createLoading}
        theme="light"
      />
      <p class="text-[10px] text-slate-400 font-medium leading-none">
        * Hanya huruf kecil, angka, dan tanda hubung (-) tanpa spasi. E.g. "sman1kluet".
      </p>
    </div>

    <div slot="footer" class="flex gap-2">
      <Button variant="secondary" size="sm" theme="light" on:click={() => showAddModal = false} disabled={createLoading}>Batal</Button>
      <Button variant="primary" size="sm" theme="light" on:click={createTenant} loading={createLoading}>Simpan</Button>
    </div>
  </Modal>
</div>
