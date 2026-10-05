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
2.  Tambahkan variabel berikut:
    *   **`PORT`**: `3000`.
    *   **`DATABASE_URL`**: `data/cbt_aether.db`.
    *   **`ENV`**: `production` (sudah default di image).
    *   **`JWT_SECRET`**: **WAJIB**. Hasil `openssl rand -hex 32` (minimal 32 karakter; nilai lemah yang dikenal ditolak).
    *   **`CORS_ALLOWED_ORIGINS`**: **WAJIB** di produksi; tanpa ini container berhenti saat start. Contoh: `https://cbt.sekolah.sch.id`
    *   **`SETUP_ADMIN_PASSWORD`**: password admin pertama (minimal 8 karakter, bukan password default). Dipakai hanya bila belum ada admin; hapus setelah login pertama dan ganti password lewat menu Pengaturan.
    *   **`TRUSTED_PROXIES`**: rentang IP proxy Coolify/Traefik (mis. `10.0.0.0/8`), agar rate limit login (`AUTH_RATE_LIMIT_PER_MIN`, `AUTH_IP_RATE_LIMIT_PER_MIN`) memakai IP klien asli.
    *   **`DEFAULT_TENANT_ID`**: isi `1` bila hanya satu sekolah memakai domain ini.
3.  Klik tombol **Save** di bagian bawah kolom variabel.

### Langkah 5: Konfigurasi Volume Persisten (SANGAT PENTING!)
Platform Aether CBT menggunakan database **SQLite 3** yang menyimpan seluruh data dalam satu berkas di folder `data/`. Karena container Docker bersifat sementara (*stateless*), Anda **wajib** membuat volume penyimpanan eksternal persisten di disk fisik VPS agar database tidak terhapus saat aplikasi diperbarui atau di-restart.

1.  Buka tab **Storage** pada dasbor resource Coolify Anda.
2.  Klik tombol **+ Add Volume**.
3.  Isi kolom konfigurasi volume persisten dengan nilai berikut secara tepat:
    *   **Volume Name / Source**: `aether-cbt-database-storage`
    *   **Mount Path / Destination (di dalam Container)**: `/app/data`
4.  Klik **Save**. Volume ini menjamin berkas database `cbt_aether.db` tersimpan secara permanen pada disk fisik server VPS Anda.
5.  Image berjalan sebagai user non-root `app` (uid 10001) dan mendeklarasikan `VOLUME /app/data`. Bila memakai bind mount ke folder host, pastikan folder itu dapat ditulis uid 10001 (`chown -R 10001:10001 <folder>`). Image juga punya `HEALTHCHECK` ke `/api/health`.
6.  **Upgrade dari image lama (root) — wajib sekali jalan.** Volume yang sudah ada berisi file milik root (`cbt_aether.db`, `-wal`/`-shm`, `queue/`, `soal/`); Docker tidak mengubah kepemilikan volume yang sudah terisi, sehingga uid 10001 gagal menulis DB/WAL/antrean. Sebelum deploy image baru, hentikan aplikasi di Coolify, lalu di VPS jalankan:
    ```bash
    docker volume ls | grep aether-cbt-database-storage   # Coolify bisa menambah prefiks pada nama volume
    docker run --rm -v <nama-volume>:/data alpine:3.22 chown -R 10001:10001 /data
    ```
    Lalu deploy. Untuk bind mount, jalankan `sudo chown -R 10001:10001 <folder>` di host.
7.  Batas waktu tulis respons server adalah 10 menit per respons (`WriteTimeout`). Transfer yang lebih lama (media paket sangat besar di jaringan lab yang padat) akan terputus; kecilkan media paket atau perbaiki bandwidth lab.

### Langkah 6: Jalankan Kompilasi dan Deployment
1.  Setelah seluruh konfigurasi di atas disimpan, klik tombol **Deploy** di pojok kanan atas.
2.  Buka tab **Deployments** untuk melihat jalannya proses kompilasi Docker secara *real-time*:
    *   *Stage 1*: Node.js mengompilasi SvelteKit menjadi aset statis di `web/build`.
    *   *Stage 2*: Golang mengompilasi kode server backend menjadi biner mandiri yang teroptimasi.
    *   *Stage 3*: Mengemas container runner berbasis Alpine Linux yang sangat aman dan ringan.
3.  Setelah proses build selesai (sekitar 1-2 menit), status aplikasi akan berubah menjadi **Running (Aktif)** berwarna hijau.

---

## 🔗 PERUTEAN MULTI-TENANT SUBDOMAIN WILDCARD DI CLOUD
Dengan domain wildcard (misalnya `https://*.domainanda.com`) semua subdomain diteruskan ke container yang sama. Server menentukan tenant per request dengan urutan berikut (`internal/api/middleware/tenant.go`):
1.  Setelah login: klaim `tenant_id` di JWT (selalu menang).
2.  Sebelum login: header/parameter `X-Tenant-ID`, lalu `X-Tenant-Slug`, lalu subdomain pertama host (`sekolaha.domainanda.com` → slug `sekolaha`, kecuali `www`/`api`).
3.  Bila tidak ada satu pun: `DEFAULT_TENANT_ID`; tanpa itu request ditolak 400 di produksi.

Slug subdomain harus sama dengan `tenants.slug`. Tenant baru dibuat oleh superadmin lewat `POST /api/tenants` (slug huruf kecil/angka/tanda hubung); tidak ada UI untuk ini. `CORS_ALLOWED_ORIGINS` harus memuat setiap origin subdomain yang dipakai.
