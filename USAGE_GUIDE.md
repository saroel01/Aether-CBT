# PANDUAN PENGGUNAAN AETHER CBT

Panduan ini disusun secara terstruktur untuk membantu Pengembang, Admin Sekolah, dan Guru/Korektor dalam memahami, mengonfigurasi, serta menggunakan seluruh fitur utama pada platform **Aether CBT (Multi-Tenant Computer-Based Testing)**.

---

## 1. PENYIAPAN & PERSIAPAN CEPAT (QUICKSTART)

Aether CBT dapat dijalankan menggunakan dua metode utama tergantung kebutuhan Anda: **Mode Sekolah (Pengguna Awam)** yang sangat praktis dan terpadu, atau **Mode Pengembang (Developer)** untuk memodifikasi source code.

### A. MODE SEKOLAH / PENGGUNA AWAM (Sangat Praktis - Satu Port Terpadu)
Dalam mode ini, seluruh aplikasi frontend dan backend disajikan secara bersamaan oleh file executable tunggal pada satu port (`3000`).
*   **Persyaratan**: Tidak ada (tidak memerlukan instalasi Go, Node.js, ataupun perintah terminal rumit).
*   **Langkah Menjalankan**:
    1.  Ekstrak paket rilis ZIP Aether CBT di komputer server.
    2.  Klik dua kali berkas executable `aether-cbt.exe` (di Windows) atau jalankan `./aether-cbt` (di Linux/macOS).
    3.  Akses aplikasi secara langsung melalui browser pada alamat:
        `http://localhost:3000` (atau IP lokal server, misal `http://192.168.1.15:3000`).
    *Catatan: Seluruh antarmuka admin, proktor, dan ujian siswa berjalan terintegrasi di port 3000.*

### B. MODE PENGEMBANGAN (DEVELOPER MODE)
Gunakan mode ini jika Anda ingin berkontribusi pada pengembangan aplikasi atau melakukan modifikasi kode sumber secara real-time.
*   **Persyaratan**: Go versi 1.22+ dan Node.js versi 18+.
*   **Langkah Menjalankan**:
    1.  Buka terminal di direktori root proyek.
    2.  Jalankan perintah berikut untuk mengaktifkan hot-reload frontend (port 5173) dan backend (port 3000) bersamaan:
        ```bash
        npm run dev
        ```
    3.  Untuk mengisi database simulasi awal dengan data contoh:
        ```bash
        npm run seed
        ```

### Akun Uji Coba Default:
*   **Super Admin / Admin**: Username: `admin` | Password: `admin123`
*   **Pengawas Ruang**: Username: `ruang_a` | Password: `ruang123`
*   **Siswa/Peserta**: Username: `2024001` | Password: `siswa123` (Token Ujian: `ujian2026`)

**PERINGATAN PENTING:**
Semua akun di atas menggunakan password default yang sangat lemah. **WAJIB** diganti sebelum digunakan untuk ujian nyata.

### Untuk Admin Sekolah (Hanya Pakai Hasil Build)
Jika Anda hanya menerima file aplikasi (bukan source code), gunakan:

- `aether-password-generator.exe` → untuk membuat password kuat
- `PANDUAN_ROTASI_KREDENSIAL_PRODUKSI.txt` → panduan singkat dalam bahasa Indonesia

File-file ini sebaiknya disertakan dalam setiap paket rilis oleh tim pengembang.

Lihat prosedur lengkap di:
→ `docs/credential-rotation.md`

### Untuk Developer
Gunakan tool berikut untuk membuat password yang kuat:
- `scripts/generate-password.ps1` (PowerShell)
- `scripts/generate-password.go` (Go)

---

## 1.1. VARIABEL LINGKUNGAN PENTING (ENVIRONMENT VARIABLES)

Untuk menjalankan Aether CBT di lingkungan produksi, beberapa variabel berikut **sangat dianjurkan** bahkan **wajib**:

| Nama Variabel            | Status          | Penjelasan |
|--------------------------|------------------|----------|
| `JWT_SECRET`             | **Wajib**        | Rahasia penandatanganan JWT. Aplikasi **akan crash** jika tidak diisi. Gunakan string panjang minimal 32 karakter acak. |
| `CORS_ALLOWED_ORIGINS`   | **Wajib di produksi** | Daftar domain yang diizinkan mengakses API (dipisah koma). Contoh: `https://cbt.sekolah.sch.id,https://admin.sekolah.sch.id` |
| `PORT`                   | Opsional         | Port aplikasi (default: 3000) |
| `DATABASE_URL`           | Opsional         | Lokasi file SQLite (default: `data/cbt_aether.db`) |
| `ENV`                    | Opsional         | `development` atau `production`. Mempengaruhi perilaku default tenant dan error message. |

**Contoh .env di produksi:**
```bash
JWT_SECRET=KunciSangatPanjangDanAcak2026Min32Karakter
CORS_ALLOWED_ORIGINS=https://cbt.sekolah.sch.id
PORT=3000
ENV=production
```

---

## 2. PANDUAN ADMINISTRASI OPERASIONAL (ADMIN FLOW)

Admin dapat mengelola seluruh entitas sekolah menggunakan portal administrasi. Di bawah ini adalah alur operasional utama:

### A. Impor Data Siswa via CSV
Untuk memasukkan ratusan siswa secara sekaligus ke dalam kelas dan ruangan:
1.  Siapkan file CSV dengan format kolom sebagai berikut (tanpa spasi setelah koma):
    `no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin,password`
    *Contoh baris data:*
    `2026001,Syahrul Hamdi,10,1,L,siswa123`
2.  Unggah berkas melalui API `POST /api/admin/students/import-csv` (atau menu Impor Siswa di dashboard admin).
3.  Sistem secara otomatis mengisolasi siswa baru ke dalam database Tenant bersangkutan.

### B. Memulai Sesi Ujian Aktif
Sebelum siswa dapat menempuh ujian:
1.  Admin/Pengawas harus memastikan mata pelajaran (mapel) telah ditautkan dengan kelas yang diuji.
2.  Siswa melakukan login di portal. Sesi aktif akan tercatat secara *real-time* di monitor pengawas (`cek_login`). Sesi ini berfungsi sebagai tiket resmi masuk ke kuis iSpring.
3.  Saat sesi dimulai, server menerbitkan `attempt_token` per siswa/mapel. Token ini harus ikut terkirim saat hasil iSpring dikirim ke webhook.

---

## 3. PANDUAN INTEGRASI ISPRING QUIZMAKER

Aether CBT mendukung penuh visualisasi dan perekaman butir soal iSpring QuizMaker secara aman dan dinamis. Ikuti langkah di bawah ini untuk menghubungkan kuis iSpring Anda:

### A. Pengaturan Formulir di iSpring QuizMaker (Pengisian Otomatis)
Guna menghindari kesalahan ketik (*typo*) nomor ujian oleh siswa, kita menggunakan metode peluncuran dinamis:
1.  Buka kuis Anda di **iSpring QuizMaker**.
2.  Pilih **Introduction** -> **User Info** pada toolbar.
3.  Tambahkan satu kolom kustom berlabel **Nomor Ujian**.
4.  **SANGAT PENTING**: Pada kolom kustom tersebut, ubah nama variabel (*Variable Name*) menjadi **`sid`**. Kolom ini harus diset sebagai **Mandatory** (Wajib).
5.  *Tip Praktis*: Anda dapat menyembunyikan form pop-up ini di browser siswa. Aether CBT akan secara otomatis mengisi parameter ini di balik layar menggunakan parameter GET pada URL iframe peluncuran kuis:
    `http://[ALAMAT_CBT]/soal/math/index.html?sid=2026001&USER_NAME=Syahrul`

### B. Konfigurasi Endpoint Webhook di iSpring QuizMaker
iSpring akan mengirimkan data hasil secara dinamis ke server Aether CBT saat siswa mengklik tombol "Selesai/Submit":
1.  Buka **Properties** -> **Reporting** di iSpring QuizMaker.
2.  Centang pilihan **Send quiz result to server**.
3.  Masukkan URL Webhook kustom sesuai identitas sekolah (tenant) Anda:
    *   Menggunakan Slug Sekolah:
        `http://[IP_CBT]:3000/api/ispring/webhook?tenant_slug=sman1kluet`
    *   Menggunakan ID Tenant:
        `http://[IP_CBT]:3000/api/ispring/webhook?tenant_id=1`
4.  Simpan, lalu publikasikan (*Publish*) kuis iSpring Anda ke format **HTML5**.

### C. Validasi Attempt Token
Aether CBT menolak hasil yang tidak berasal dari sesi aktif. Pastikan hasil iSpring membawa salah satu field berikut:

* `attempt_token`
* `AETHER_ATTEMPT_TOKEN`

Nilainya diterbitkan oleh Aether CBT saat siswa memulai mata pelajaran. Simulator bawaan frontend sudah mengirim field ini otomatis. Untuk paket iSpring asli, tambahkan field/variabel user info tersembunyi yang nilainya diisi dari parameter launch URL.

---

## 4. FITUR EKSPOR JAWABAN ESAI MULTI-FORMAT (GURU & KOREKTOR)

Karena soal esai memerlukan penilaian manual oleh guru, Aether CBT menyediakan fitur ekspor dinamis berkualitas premium yang memisahkan seluruh butir esai siswa agar guru dapat menilainya dengan mudah.

### Cara Mengunduh Laporan
Akses endpoint berikut (terproteksi hak akses Admin / Pengawas):
*   **Format CSV**: `GET /api/admin/results/export-essay/csv`
*   **Format Excel (XLSX)**: `GET /api/admin/results/export-essay/xlsx`
*   **Format Cetak PDF**: `GET /api/admin/results/export-essay/pdf`

### Detil Desain Laporan:

#### 📊 Rekap Excel (XLSX)
Spreadsheet didesain rapi dan bersih:
*   **Header Elegan**: Memiliki baris header berwarna *Steel Blue* (`4682B4`) dengan teks tebal putih berukuran proporsional.
*   **Penyajian Luang**: Kolom jawaban esai dilebarkan secara khusus dan mengaktifkan fitur *Auto Wrap Text* agar paragraf jawaban panjang siswa terlipat rapi tanpa terpotong.
*   **Border Grid**: Seluruh data dibingkai oleh garis grid tipis berwarna abu-abu terang (`D3D3D3`).

#### 📄 Dokumen Cetak PDF
File PDF dirancang indah dengan struktur layout formal siap cetak:
*   **Kop Surat Tenant**: Menampilkan Nama Sekolah (Tenant) yang tebal di bagian atas, bergaris pemisah ganda estetis warna Steel Blue.
*   **Kartu Soal & Jawaban**: Setiap siswa dikelompokkan dalam kartu panel khusus.
    *   *Pertanyaan*: Teks soal dibungkus di dalam kotak berlatar abu-abu terang lembut (`#F5F5F5`) sehingga nyaman dibaca.
    *   *Jawaban*: Teks esai siswa dicetak tebal miring berwarna biru tua.
*   **Kolom Korektor Fisik**: Setiap soal memiliki panel bawah berwarna kuning gading lembut yang menyertakan skor kuis sementara beserta kotak kosong untuk diisi nilai manual dan tanda tangan korektor: `Nilai Akhir Guru:  ________  (Paraf: ____)`.
*   **Penomoran Halaman**: Footer dinamis bertuliskan `"Halaman X dari Y | Aether CBT"`.

---

## 5. KONFIGURASI JARINGAN LOKAL (OFFLINE / INTRANET)

Dalam skenario implementasi di sekolah, server Aether CBT biasanya dioperasikan pada satu komputer server lokal, kemudian diakses oleh perangkat siswa (gawai/laptop) melalui jaringan Wi-Fi lokal tanpa memerlukan koneksi internet aktif.

Berikut adalah langkah-langkah konfigurasi jaringan lokal secara terstruktur:

### Langkah 1: Penyiapan Jaringan Wi-Fi
1.  Hubungkan komputer server Aether CBT dan seluruh perangkat siswa ke titik akses (Access Point) Wi-Fi yang sama.
2.  Pastikan Access Point aktif. Koneksi internet luar (WAN) tidak diperlukan karena proses transfer data berjalan dalam jaringan lokal (intranet).

### Langkah 2: Mengidentifikasi IP Address Lokal Server
1.  Buka aplikasi **Command Prompt (CMD)** pada sistem operasi Windows server.
2.  Jalankan perintah berikut:
    ```cmd
    ipconfig
    ```
3.  Cari antarmuka jaringan yang aktif (misalnya *Wireless LAN adapter Wi-Fi* atau *Ethernet adapter*).
4.  Catat angka pada baris **IPv4 Address** (misalnya `192.168.1.15`).

### Langkah 3: Penyesuaian Endpoint Webhook di iSpring
Saat melakukan publikasi kuis di iSpring QuizMaker, ganti host tujuan pengiriman nilai agar merujuk ke IP Address server lokal yang telah dicatat, bukan `localhost`:
*   Format URL Webhook:
    `http://[IP_ADDRESS_SERVER]:3000/api/ispring/webhook?tenant_slug=[SLUG_TENANT]`
*   *Contoh:*
    `http://192.168.1.15:3000/api/ispring/webhook?tenant_slug=default`

### Langkah 4: Akses Siswa Melalui Perangkat Klien
1.  Minta siswa menghubungkan perangkat ke jaringan Wi-Fi sekolah yang sama.
2.  Buka browser web (disarankan Google Chrome atau Safari) di perangkat siswa.
3.  Akses alamat IP Address server terpadu dengan port (`3000`):
    `http://[IP_ADDRESS_SERVER]:3000`
    *Contoh:*
    `http://192.168.1.15:3000`
4.  Halaman login siswa akan terbuka secara otomatis dan siap digunakan.
