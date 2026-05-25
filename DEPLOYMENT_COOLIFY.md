# PANDUAN DEPLOYMENT COOLIFY (INTRUKSI LANGKAH-DEMI-LANGKAH)
## Aether CBT — Platform Computer-Based Testing Modern

**Coolify** adalah dasbor PaaS (*Platform-as-a-Service*) mandiri yang sangat andal dan mudah dikonfigurasi untuk menghosting aplikasi Docker. 

Karena berkas **`Dockerfile`** multi-stage yang tangguh telah disediakan di direktori root Aether CBT, Anda dapat menyebarkan platform ini di VPS menggunakan Coolify secara instan dalam beberapa langkah instruksional berikut.

---

## 🚀 PANDUAN OPERASIONAL DEPLOYMENT DI DASBOR COOLIFY

Ikuti langkah-langkah di bawah ini secara runut untuk melakukan deployment Aether CBT di server VPS Anda:

### Langkah 1: Hubungkan Repositori Git Anda ke Coolify
1.  Buka dasbor **Coolify** Anda di browser.
2.  Buka menu **Sources** pada bilah sisi kiri, lalu pastikan integrasi akun Git Anda (GitHub atau GitLab) sudah terhubung secara aktif.
3.  Kembali ke menu **Projects**, pilih proyek yang ada (atau klik **Create New Project** di pojok kanan atas).
4.  Pilih **Environment** (biasanya bernama *production*).
5.  Klik tombol **+ Add New Resource** di pojok kanan atas, lalu pilih opsi **Public Repository** atau **Private Repository**.
6.  Pilih repositori `Aether-CBT` Anda dari daftar proyek yang muncul.

### Langkah 2: Konfigurasikan Tipe Builder (Docker)
1.  Setelah repositori terpilih, Coolify akan memindai isi file proyek secara otomatis.
2.  Pada kolom **Build Pack**, pilih opsi **Docker** (Coolify akan secara cerdas mengenali file `Dockerfile` multi-stage yang berada di root proyek untuk proses kompilasi otomatis).
3.  Pada kolom **Destination / Server**, pilih node VPS/Docker host tempat aplikasi ingin Anda jalankan.
4.  Klik tombol **Save** atau **Configure**.

### Langkah 3: Pengaturan Domain & Enkripsi HTTPS Otomatis
Coolify akan menangani konfigurasi *reverse proxy* dan sertifikat SSL secara otomatis tanpa perlu melakukan setting manual.
1.  Cari bagian input **Domain** pada tab **General Settings** resource Anda.
2.  Masukkan domain atau subdomain yang ingin Anda gunakan:
    *   *Satu Domain Utama*: `https://ujiancbt.id`
    *   *Wildcard Subdomain dinamis (Sangat Direkomendasikan untuk Multi-Tenant)*:
        `https://*.ujiancbt.id`
        *(Ganti `ujiancbt.id` dengan domain resmi yang Anda miliki).*
3.  Simpan konfigurasi. Coolify akan secara otomatis menerbitkan sertifikat SSL gratis dari Let's Encrypt dan mengaktifkan protokol HTTPS.

### Langkah 4: Konfigurasi Variabel Lingkungan (Environment Variables)
1.  Buka tab **Environment Variables** di dasbor resource Coolify Anda.
2.  Tambahkan tiga variabel penting berikut:
    *   **`PORT`**: Isi dengan nilai `3000`.
    *   **`DATABASE_URL`**: Isi dengan nilai `data/cbt_aether.db`.
    *   **`JWT_SECRET`**: **WAJIB**. Isi dengan kunci token rahasia acak yang sangat kuat (minimal 32 karakter).
    *   **`CORS_ALLOWED_ORIGINS`**: **Sangat direkomendasikan di produksi**. Contoh: `https://cbt.sekolah.sch.id,https://admin.sekolah.sch.id`
3.  Klik tombol **Save** di bagian bawah kolom variabel.

### Langkah 5: Konfigurasi Volume Persisten (SANGAT PENTING!)
Platform Aether CBT menggunakan database **SQLite 3** yang menyimpan seluruh data dalam satu berkas di folder `data/`. Karena container Docker bersifat sementara (*stateless*), Anda **wajib** membuat volume penyimpanan eksternal persisten di disk fisik VPS agar database tidak terhapus saat aplikasi diperbarui atau di-restart.

1.  Buka tab **Storage** pada dasbor resource Coolify Anda.
2.  Klik tombol **+ Add Volume**.
3.  Isi kolom konfigurasi volume persisten dengan nilai berikut secara tepat:
    *   **Volume Name / Source**: `aether-cbt-database-storage`
    *   **Mount Path / Destination (di dalam Container)**: `/app/data`
4.  Klik **Save**. Volume ini menjamin berkas database `cbt_aether.db` tersimpan secara permanen pada disk fisik server VPS Anda.

### Langkah 6: Jalankan Kompilasi dan Deployment
1.  Setelah seluruh konfigurasi di atas disimpan, klik tombol **Deploy** di pojok kanan atas.
2.  Buka tab **Deployments** untuk melihat jalannya proses kompilasi Docker secara *real-time*:
    *   *Stage 1*: Node.js mengompilasi SvelteKit menjadi aset statis di `web/build`.
    *   *Stage 2*: Golang mengompilasi kode server backend menjadi biner mandiri yang teroptimasi.
    *   *Stage 3*: Mengemas container runner berbasis Alpine Linux yang sangat aman dan ringan.
3.  Setelah proses build selesai (sekitar 1-2 menit), status aplikasi akan berubah menjadi **Running (Aktif)** berwarna hijau.

---

## 🔗 PERUTEAN MULTI-TENANT SUBDOMAIN WILDCARD DI CLOUD
Jika Anda memasang domain wildcard (misalnya `https://*.domainanda.com`), fitur isolasi multi-tenant otomatis di Aether CBT akan langsung beroperasi secara penuh:
*   Browser siswa yang memanggil `sekolaha.domainanda.com` secara instan menyajikan halaman kuis milik **Sekolah A**.
*   Browser siswa yang memanggil `sekolahb.domainanda.com` secara instan menyajikan halaman kuis milik **Sekolah B**.
*   Dasbor Coolify secara cerdas bertindak sebagai *wildcard router* yang meneruskan dan mengamankan seluruh lalu lintas data HTTPS subdomain tersebut langsung ke port internal container `3000` secara mulus.
