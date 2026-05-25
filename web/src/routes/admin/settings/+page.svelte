<script lang="ts">
  import { api, qrCodeUrl } from '$lib/api';
  import { onMount } from 'svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import { toast } from '$lib/stores/toast';

  let examTitle = '';
  let proctorName = '';
  let footerText = '';
  let activeToken = '';
  let isExamActive = false;
  let loading = true;
  let saveLoading = false;

  onMount(async () => {
    await loadSettings();
  });

  async function loadSettings() {
    loading = true;
    try {
      const res = await api('/admin/settings');
      if (res.success && res.data) {
        examTitle = res.data.exam_title;
        proctorName = res.data.proctor_name;
        footerText = res.data.footer_text;
        activeToken = res.data.token;
        isExamActive = res.data.is_exam_active;
      }
    } catch {
      toast.error('Gagal memuat pengaturan server.');
    }
    loading = false;
  }

  async function saveSettings() {
    if (!examTitle || !activeToken) {
      toast.warning('Judul Ujian dan Token harus diisi!');
      return;
    }

    saveLoading = true;
    try {
      await api('/admin/settings', {
        method: 'POST',
        body: JSON.stringify({
          exam_title: examTitle,
          proctor_name: proctorName,
          footer_text: footerText,
          token: activeToken,
          is_exam_active: isExamActive
        })
      });
      toast.success('Pengaturan berhasil diperbarui!');
    } catch (e: any) {
      toast.error('Gagal memperbarui pengaturan: ' + e.message);
    }
    saveLoading = false;
  }

  // Helper to rotate token by generating a random 6-character string
  function rotateToken() {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
    let code = '';
    for (let i = 0; i < 6; i++) {
      code += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    activeToken = code;
    toast.info(`Token diubah menjadi: "${code}". Simpan perubahan untuk mengaktifkannya.`);
  }
</script>

<svelte:head>
  <title>Konfigurasi Ujian - Admin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto">
  <!-- Section Title -->
  <div class="border-b pb-6">
    <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight">Pengaturan Ujian</h1>
    <p class="text-slate-500 text-sm">Kelola spesifikasi ujian sekolah, kredensial proktor, dan rotasi token.</p>
  </div>

  {#if loading}
    <div class="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
      <svg class="animate-spin h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
      <p class="text-sm font-semibold">Memuat data pengaturan...</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-8 items-start">
      <!-- General configuration (2/3) -->
      <div class="lg:col-span-2 flex flex-col gap-6">
        <Card padding="lg" class="border-slate-200 bg-white shadow-sm">
          <h3 class="text-lg font-bold text-slate-800 mb-4 pb-2 border-b">Konfigurasi Umum</h3>
          
          <div class="space-y-4">
            <Input 
              id="exam_title_input"
              label="Judul Pelaksanaan Ujian *" 
              placeholder="Contoh: Penilaian Akhir Semester 2025/2026" 
              bind:value={examTitle}
              disabled={saveLoading}
            />

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Input 
                id="proctor_name_input"
                label="Nama Proktor / Kepala Ruangan" 
                placeholder="Contoh: Drs. H. Sumardi" 
                bind:value={proctorName}
                disabled={saveLoading}
              />

              <Input 
                id="footer_text_input"
                label="Teks Kaki Halaman (Footer)" 
                placeholder="Contoh: Aether CBT • Hak Cipta Sekolah" 
                bind:value={footerText}
                disabled={saveLoading}
              />
            </div>

            <!-- Global toggle activation -->
            <div class="flex items-center justify-between py-4 border-t border-slate-100 mt-6">
              <div>
                <h4 class="text-sm font-bold text-slate-800">Status Server Ujian</h4>
                <p class="text-xs text-slate-500">Apabila dinonaktifkan, siswa tidak dapat masuk ke sistem lembar ujian.</p>
              </div>
              <button 
                type="button" 
                aria-label={isExamActive ? 'Nonaktifkan server ujian' : 'Aktifkan server ujian'}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none
                  {isExamActive ? 'bg-indigo-600' : 'bg-slate-200'}"
                on:click={() => isExamActive = !isExamActive}
              >
                <span class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out
                  {isExamActive ? 'translate-x-5' : 'translate-x-0'}"
                ></span>
              </button>
            </div>
          </div>
        </Card>

        <Button 
          variant="primary" 
          size="lg" 
          class="bg-indigo-600 hover:bg-indigo-700 text-white font-semibold shadow-md py-3.5 border-none w-full"
          on:click={saveSettings}
          loading={saveLoading}
        >
          Simpan Seluruh Perubahan
        </Button>
      </div>

      <!-- Token Manager & QR (1/3) -->
      <div class="lg:col-span-1 flex flex-col gap-6">
        <Card padding="md" class="border-slate-200/50 bg-white text-center shadow-sm">
          <div class="text-xs text-slate-400 font-bold uppercase tracking-wider mb-2">Token Ujian Aktif</div>
          
          <div class="flex gap-2 mb-4">
            <input 
              id="token_input_box"
              type="text" 
              bind:value={activeToken} 
              disabled={saveLoading} 
              class="w-full text-center text-xl font-extrabold text-indigo-600 font-mono border rounded-xl outline-none focus:ring-2 focus:ring-indigo-500 bg-slate-50/50 uppercase tracking-widest"
            />
            <Button 
              variant="secondary" 
              size="sm" 
              class="px-3" 
              on:click={rotateToken}
              title="Acak Token"
              disabled={saveLoading}
            >
              <svg class="h-5 w-5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 1121.21 8H18.2" />
              </svg>
            </Button>
          </div>

          <div class="text-xs text-slate-400 font-bold uppercase tracking-wider mb-3">Live QR Code Token</div>
          {#if activeToken}
            <div class="bg-slate-50 p-3 border rounded-3xl inline-block mx-auto mb-3">
              <img src={qrCodeUrl(activeToken)} alt="QR Token" class="h-40 w-40 mx-auto" />
            </div>
          {/if}

          <p class="text-[11px] text-slate-500 leading-relaxed px-2">
            * Jika Anda mengubah token, pastikan untuk mengklik tombol **"Simpan Seluruh Perubahan"** di samping agar perubahan token tersebut aktif untuk siswa.
          </p>
        </Card>
      </div>
    </div>
  {/if}
</div>
