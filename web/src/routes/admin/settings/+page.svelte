<script lang="ts">
  import { onMount } from 'svelte';
  import { api, qrCodeUrl } from '$lib/api';
  import Button from '$lib/components/ui/Button.svelte';
  import Card from '$lib/components/ui/Card.svelte';
  import Input from '$lib/components/ui/Input.svelte';
  import PasswordGenerator from '$lib/components/PasswordGenerator.svelte';
  import Modal from '$lib/components/ui/Modal.svelte';
  import { authStore } from '$lib/stores/auth';
  import { goto } from '$app/navigation';
  import { toast } from '$lib/stores/toast';

  let examTitle = '';
  let proctorName = '';
  let footerText = '';
  let activeToken = '';
  let isExamActive = false;
  let loading = true;
  let saveLoading = false;

  // My Profile (self update)
  let currentPassword = '';
  let newUsername = '';
  let newPassword = '';
  let confirmNewPassword = '';
  let profileLoading = false;

  // Confirmation modal
  let showConfirmModal = false;

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

  function prepareUpdateProfile() {
    if (!currentPassword) {
      toast.warning('Masukkan password saat ini untuk verifikasi');
      return;
    }

    if (newPassword && newPassword !== confirmNewPassword) {
      toast.warning('Password baru dan konfirmasi tidak sama');
      return;
    }

    if (newPassword && newPassword.length < 6) {
      toast.warning('Password baru minimal 6 karakter');
      return;
    }

    showConfirmModal = true;
  }

  async function confirmAndUpdateProfile() {
    showConfirmModal = false;
    profileLoading = true;

    try {
      const payload: any = {
        current_password: currentPassword
      };

      if (newUsername.trim()) payload.new_username = newUsername.trim();
      if (newPassword) payload.new_password = newPassword;

      const isChangingPassword = !!newPassword;

      await api('/me', {
        method: 'PUT',
        body: JSON.stringify(payload)
      });

      if (isChangingPassword) {
        toast.success('Password berhasil diubah. Anda akan di-logout otomatis...');
        
        // Logout otomatis + redirect ke halaman login admin
        setTimeout(() => {
          authStore.logout();
          goto('/admin');
        }, 1200);
      } else {
        toast.success('Username berhasil diperbarui!');
        // Reset form jika hanya ganti username
        currentPassword = '';
        newUsername = '';
      }
    } catch (e: any) {
      toast.error('Gagal memperbarui profil: ' + e.message);
    }

    profileLoading = false;
  }
</script>

<svelte:head>
  <title>Konfigurasi Ujian - Admin</title>
</svelte:head>

<div class="p-8 flex flex-col gap-6 max-w-7xl mx-auto select-none">
  <!-- Section Title -->
  <div class="border-b border-slate-200/60 pb-6">
    <h1 class="text-3xl font-extrabold text-slate-900 tracking-tight font-display">Pengaturan Ujian</h1>
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
      <!-- General configuration (2/3 Grid) -->
      <div class="lg:col-span-2 flex flex-col gap-6">
        <Card padding="lg" class="border-slate-200/60 bg-white shadow-sm relative overflow-hidden">
          <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-100"></div>

          <h3 class="text-sm font-bold text-slate-800 uppercase tracking-widest font-mono mb-6 pb-2 border-b border-slate-100">Konfigurasi Umum</h3>
          
          <div class="space-y-5">
            <Input 
              id="exam_title_input"
              label="Judul Pelaksanaan Ujian *" 
              placeholder="Contoh: Penilaian Akhir Semester 2025/2026" 
              bind:value={examTitle}
              disabled={saveLoading}
              theme="light"
            />

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
              <Input 
                id="proctor_name_input"
                label="Nama Proktor / Kepala Ruangan" 
                placeholder="Contoh: Drs. H. Sumardi" 
                bind:value={proctorName}
                disabled={saveLoading}
                theme="light"
              />

              <Input 
                id="footer_text_input"
                label="Teks Kaki Halaman (Footer)" 
                placeholder="Contoh: Aether CBT • Hak Cipta Sekolah" 
                bind:value={footerText}
                disabled={saveLoading}
                theme="light"
              />
            </div>

            <!-- Global toggle activation -->
            <div class="flex items-center justify-between py-5 border-t border-slate-100 mt-6">
              <div>
                <h4 class="text-sm font-bold text-slate-800">Status Server Ujian</h4>
                <p class="text-xs text-slate-400">Apabila dinonaktifkan, siswa tidak dapat masuk ke sistem lembar ujian.</p>
              </div>
              <button 
                type="button" 
                aria-label={isExamActive ? 'Nonaktifkan server ujian' : 'Aktifkan server ujian'}
                class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-300 ease-[cubic-bezier(0.16,1,0.3,1)] focus:outline-none
                  {isExamActive ? 'bg-indigo-600' : 'bg-slate-200'}"
                on:click={() => isExamActive = !isExamActive}
              >
                <span class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]
                  {isExamActive ? 'translate-x-5' : 'translate-x-0'}"
                ></span>
              </button>
            </div>
          </div>
        </Card>

        <Button 
          variant="primary" 
          size="lg" 
          theme="light"
          class="w-full font-semibold mt-2"
          on:click={saveSettings}
          loading={saveLoading}
        >
          Simpan Seluruh Perubahan
        </Button>
      </div>

      <!-- Token Manager & QR (1/3 Grid) -->
      <div class="lg:col-span-1 flex flex-col gap-6">
        <Card padding="md" class="border-slate-200/50 bg-white text-center shadow-sm relative overflow-hidden">
          <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-100"></div>

          <div class="text-[10px] text-slate-400 font-bold uppercase tracking-wider mb-3 font-mono">Token Ujian Aktif</div>
          
          <div class="flex gap-2 mb-4">
            <input 
              id="token_input_box"
              type="text" 
              bind:value={activeToken} 
              disabled={saveLoading} 
              class="w-full text-center text-xl font-extrabold text-indigo-600 font-mono border border-slate-200 rounded-2xl outline-none focus:ring-4 focus:ring-indigo-600/10 focus:border-indigo-600 bg-slate-50/50 uppercase tracking-widest transition-all duration-300"
            />
            <Button 
              variant="secondary" 
              size="sm" 
              theme="light"
              class="px-3 rounded-2xl" 
              on:click={rotateToken}
              title="Acak Token"
              disabled={saveLoading}
            >
              <svg class="h-5 w-5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 1121.21 8H18.2" />
              </svg>
            </Button>
          </div>

          <div class="text-[10px] text-slate-400 font-bold uppercase tracking-wider mb-3 font-mono">Live QR Code Token</div>
          {#if activeToken}
            <div class="bg-slate-50 p-4 border border-slate-100 rounded-3xl inline-block mx-auto mb-3 hover:scale-[1.01] transition-transform duration-300">
              <img src={qrCodeUrl(activeToken)} alt="QR Token" class="h-40 w-40 mx-auto" />
            </div>
          {/if}

          <p class="text-[11px] text-slate-400 leading-relaxed px-2 font-medium">
            * Jika Anda mengubah token, pastikan untuk mengklik tombol **"Simpan Seluruh Perubahan"** di samping agar perubahan token tersebut aktif untuk siswa.
          </p>
        </Card>
      </div>
    </div>

    <!-- === AKUN SAYA (Self Profile Update) === -->
    <div class="mt-8">
      <Card padding="lg" class="border-slate-200 bg-white shadow-sm max-w-2xl relative overflow-hidden">
        <div class="absolute top-0 left-0 w-full h-[1px] bg-slate-100"></div>

        <h3 class="text-sm font-bold text-slate-800 uppercase tracking-widest font-mono mb-1">Akun Saya</h3>
        <p class="text-xs text-slate-400 mb-6">Ubah username atau password Anda sendiri. Masukkan password saat ini untuk verifikasi keamanan.</p>

        <div class="space-y-5">
          <Input 
            label="Password Saat Ini *" 
            type="password" 
            bind:value={currentPassword} 
            placeholder="Masukkan password Anda saat ini"
            disabled={profileLoading}
            theme="light"
          />

          <Input 
            label="Username Baru (opsional)" 
            bind:value={newUsername} 
            placeholder="Kosongkan jika tidak ingin mengubah"
            disabled={profileLoading}
            theme="light"
          />

          <PasswordGenerator 
            bind:value={newPassword} 
            length={12}
            label="Password Baru (opsional)"
            placeholder="Klik Generate atau ketik manual"
            theme="light"
          />

          {#if newPassword}
            <Input 
              label="Konfirmasi Password Baru" 
              type="password" 
              bind:value={confirmNewPassword} 
              placeholder="Ulangi password baru"
              disabled={profileLoading}
              theme="light"
            />
          {/if}

          <div class="pt-2">
            <Button 
              variant="primary" 
              size="md" 
              theme="light"
              class="w-full font-semibold shadow-md shadow-indigo-600/10" 
              on:click={prepareUpdateProfile}
              loading={profileLoading}
              disabled={!currentPassword || profileLoading}
            >
              Simpan Perubahan Akun
            </Button>
          </div>

          <p class="text-[11px] text-amber-600 font-semibold leading-relaxed">
            ⚠️ Jika Anda mengubah password, Anda akan diminta login ulang pada sesi berikutnya.
          </p>
        </div>
      </Card>
    </div>
  {/if}

  <!-- Confirmation Modal -->
  <Modal show={showConfirmModal} title="Konfirmasi Perubahan Akun" size="sm">
    <div class="space-y-4 text-slate-700 p-2">
      <p class="text-base font-semibold text-slate-900">Apakah Anda yakin ingin menyimpan perubahan pada akun Anda?</p>
      
      {#if newPassword}
        <div class="bg-red-50 border border-red-150 rounded-2xl p-4 text-xs text-red-700 font-semibold leading-relaxed">
          ⚠️ <strong>Peringatan:</strong> Anda akan mengubah password. Setelah berhasil, Anda akan <strong>otomatis di-logout</strong> dan harus login kembali.
        </div>
      {/if}
    </div>

    <div slot="footer" class="flex gap-3 justify-end">
      <Button 
        variant="secondary" 
        size="sm" 
        theme="light"
        on:click={() => showConfirmModal = false}
        disabled={profileLoading}
      >
        Batal
      </Button>
      <Button 
        variant="primary" 
        size="sm" 
        theme="light"
        class="font-semibold shadow-md"
        on:click={confirmAndUpdateProfile}
        loading={profileLoading}
      >
        Ya, Simpan Perubahan
      </Button>
    </div>
  </Modal>
</div>
