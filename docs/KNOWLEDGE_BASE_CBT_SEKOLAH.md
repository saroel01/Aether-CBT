# KNOWLEDGE BASE: CBT SEKOLAH (Computer Based Test untuk Sekolah)
## Sistem Ujian Berbasis Web dengan Soal iSpring HTML5 Publish

**Versi Aplikasi**: 7.0.0 (berdasarkan footer di login)
**Teknologi**: PHP + MySQL + JavaScript + iSpring QuizMaker (HTML5 output)
**Tujuan**: Platform ujian online/offline sekolah yang menggunakan file soal hasil publish iSpring QuizMaker.

---

## 1. RINGKASAN EKSEKUTIF

Aplikasi **CBT Sekolah** adalah sistem ujian berbasis komputer (CBT) tradisional yang dirancang khusus untuk sekolah. Karakteristik utama:

- Menggunakan **soal HTML5 hasil publish dari iSpring QuizMaker** (bukan soal native).
- Hasil ujian dikirim otomatis ke server melalui fitur **"Send results to server"** iSpring.
- Mendukung **multi-ruangan** dengan pengawas terpisah per ruang.
- Fitur lengkap untuk manajemen data peserta, kelas, mapel, token, live monitoring, analisis hasil, backup/restore.
- Arsitektur **monolitik PHP** (bukan framework modern seperti Laravel).

Aplikasi ini cocok sebagai referensi untuk rebuild versi modern (React + Node.js / Laravel + Vue, dll) dengan arsitektur yang lebih scalable, secure, dan maintainable.

---

## 2. ARSITEKTUR & CARA KERJA

### 2.1 Teknologi Stack
- **Backend**: PHP native + MySQL (mysqli)
- **Frontend**: HTML + CSS custom + jQuery + FontAwesome + Chart.js
- **Library Khusus**:
  - `phpqrcode` → Generate QR code token
  - `phpspreadsheet` → Import/Export Excel
  - Webcam.js + jQuery → Capture foto peserta
- **Integrasi Soal**: iSpring QuizMaker HTML5 output (folder `data_soal/` berisi `index.html`, `ismplayer.html`, assets)

### 2.2 Alur Kerja Utama (Flow)

```
1. Admin Setup
   - Buat database (buat_db.php)
   - Import data peserta/kelas/mata pelajaran/ruang via Excel
   - Upload/publish soal iSpring ke folder data_soal/
   - Atur mapel per kelas + aktifkan ujian
   - Generate/atur token ujian

2. Peserta Login
   - Buka halaman utama (index.php)
   - Input: No ID + Password
   - Validasi token (dari tabel login)
   - Pilih mapel (jika ada pilihan)

3. Mulai Ujian
   - Load file soal iSpring (tampil_ujian.php / proses_ujian.php)
   - iSpring quiz berjalan di iframe atau full page
   - Peserta mengerjakan soal HTML5

4. Pengiriman Hasil
   - iSpring "Send results to server" → memanggil script PHP (storeImage.php / proses khusus)
   - Data hasil (skor, jawaban detail, waktu) disimpan ke tabel `hasil_tes`
   - Validasi: `no_id + nama_mapel`

5. Monitoring & Pengawasan
   - Admin & Pengawas melihat live status (sedang mengerjakan / selesai)
   - Pengawas per ruang dapat reset peserta tertentu
   - Foto peserta + webcam capture

6. Hasil & Analisis
   - Export Excel/PDF
   - Analisis jawaban per soal
   - Rekap skor per kelas/ruang
```

### 2.3 Integrasi iSpring (Kunci Utama) - Berdasarkan Repo Resmi iSpring QuizResults

Aplikasi **tidak membuat soal sendiri**. Ia hanya **host** file hasil publish iSpring QuizMaker.

#### Cara Kerja Pengiriman Hasil (Sesuai Standar iSpring)

1. **Konfigurasi di iSpring QuizMaker**:
   - Buka **Quiz Properties → Reporting**
   - Centang **"Send quiz result to server"**
   - Masukkan URL script PHP yang akan menerima data (contoh: `https://server.com/admin/data_snap/storeImage.php`)
   - Publish quiz ke format HTML5

2. **Variabel POST yang dikirim iSpring** (dari repo resmi):
   | Variabel | Deskripsi | Keterangan |
   |----------|-----------|------------|
   | `v` | QuizMaker version | - |
   | `dr` | **Detailed results** dalam format XML | **Paling penting** - berisi semua jawaban detail |
   | `sp` | Earned points (skor yang diperoleh) | - |
   | `tp` | Gained score | - |
   | `ps` / `psp` | Passing score (poin atau persen) | - |
   | `qt` | Quiz title (nama ujian) | - |
   | `sn` / `USER_NAME` | Username peserta | - |
   | `se` / `USER_EMAIL` | Email peserta | - |
   | `ut` / `fut` | Used time / Time spent | Durasi pengerjaan |
   | `sid` | User ID | Bisa digunakan untuk no_id |

3. **Format Detailed Results (`dr`)**:
   - Dikirim dalam bentuk **XML string**
   - Mengikuti skema: `QuizReport.xsd` / `QuizReport_8.xsd`
   - Berisi per pertanyaan: `questionId`, `status` (correct/incorrect), `awardedPoints`, `maxPoints`, `userResponse`, `correctAnswer`, dll.

4. **Implementasi di CBT Sekolah**:
   - Script penerima: `admin/data_snap/storeImage.php`
   - Data hasil disimpan ke tabel `hasil_tes`:
     - `skor` ← `sp` atau `tp`
     - `detail` ← XML mentah dari `dr` (atau diparse)
     - `durasi_kerja` ← `ut` / `fut`
     - `validasi` ← `no_id + nama_mapel` (unik)

5. **Catatan Penting**:
   - iSpring mengirim data via **HTTP POST** setelah peserta menyelesaikan quiz
   - Script penerima harus bisa parse XML dari variabel `dr`
   - Untuk user identification, biasanya menggunakan **User Info form** di iSpring (USER_NAME, USER_EMAIL, atau custom variable seperti `no_id`)

Lihat folder `troubleshooting/SETTING PUBLISH ISPRING/` untuk screenshot konfigurasi dan repo resmi: https://github.com/ispringsolutions/QuizResults

---

## 3. STRUKTUR DATABASE (Relasi Tabel)

### Tabel Utama

| Tabel          | Deskripsi                                      | Kolom Penting                                      | Relasi |
|----------------|------------------------------------------------|----------------------------------------------------|--------|
| **login**      | Konfigurasi global + kredensial admin + token | `token`, `nama_sekolah`, `nama_tes`, `file_soal`  | Pusat konfigurasi |
| **data_peserta** | Data peserta ujian                           | `no_id`, `password`, `nama_peserta`, `nama_kelas`, `ruang_ujian`, `foto` | Utama |
| **kelas**      | Mapping kelas ↔ mata pelajaran                 | `nama_kelas`, `nama_mapel`, `kode_mapel`          | 1 kelas bisa punya beberapa mapel |
| **mapel**      | Daftar mata pelajaran                          | `nama_mapel`                                       | Referensi |
| **ruang**      | Ruang ujian + kredensial pengawas              | `ruang_ujian`, `username_ruang`, `password_ruang` | Pengawas login per ruang |
| **hasil_tes**  | Hasil ujian peserta                            | `no_id`, `cek_mapel`, `skor`, `validasi`, `detail` (XML mentah), `durasi_kerja` | Hasil akhir dari iSpring |
| **cek_login**  | Tracking peserta yang sedang login             | `id_login`, `waktu`                                | Monitoring real-time |

### Relasi Penting

- `data_peserta.nama_kelas` → `kelas.nama_kelas`
- `data_peserta.ruang_ujian` → `ruang.ruang_ujian`
- `kelas.nama_mapel` → `mapel.nama_mapel`
- `hasil_tes.validasi` = `no_id + nama_mapel` (unik per peserta per mapel)
- `hasil_tes.no_id` → `data_peserta.no_id`

---

## 4. FITUR LENGKAP

### 4.1 Modul Admin
- **Manajemen Data**:
  - Peserta (tambah, ubah, hapus, import/export Excel)
  - Kelas, Mapel, Ruang (CRUD + import Excel)
  - Ubah No ID peserta
- **Pengaturan Ujian**:
  - Aktifkan soal per mapel/kelas
  - Atur token (acak, ganti, reset)
  - Upload soal iSpring
- **Hasil & Analisis**:
  - Lihat hasil per peserta/kelas/ruang
  - Export Excel (detail + rekap)
  - Analisis jawaban per soal
  - PDF report
- **Monitoring**:
  - Live status peserta (sedang mengerjakan / selesai)
  - Jumlah login, skor tertinggi/rata-rata
- **Sistem**:
  - Backup & Restore database/file
  - Hapus data ujian
  - Capture foto peserta (webcam)

### 4.2 Modul Pengawas (Ruang)
- Login dengan username/password ruang
- Monitoring peserta di ruang tersebut saja
- Reset peserta tertentu (hapus dari cek_login)
- Lihat token live
- Rekap jumlah status & skor per ruang

### 4.3 Modul Peserta
- Login dengan No ID + Password
- Token validasi global
- Pilih mapel (jika multiple)
- Kerjakan soal iSpring HTML5
- Hasil otomatis terkirim

### 4.4 Fitur Khusus
- **Token-based access** (1 token untuk seluruh ujian)
- **Multi-room supervision** dengan pengawas terpisah
- **Photo verification** (foto peserta tersimpan)
- **Detailed result storage** (`detail` TEXT berisi jawaban lengkap dari iSpring)
- **QR Code** untuk token
- **Excel import/export** massal
- **Live dashboard** dengan AJAX polling

---

## 5. STRUKTUR FOLDER & FILE PENTING

```
cbt_sekolah/
├── config.php                  # Koneksi DB
├── index.php                   # Login peserta
├── proses_login.php
├── proses_ujian.php
├── tampil_ujian.php            # Loader soal iSpring
├── exam-view.php               # Dashboard monitoring
├── data_soal/                  # Folder soal iSpring (index.html + assets)
├── admin/                      # Panel admin (semua manajemen)
├── pengawas/                   # Panel pengawas ruang
├── aset_gambar/foto_peserta/   # Foto peserta
├── admin/data_snap/            # Script capture foto + storeImage.php
├── admin/tombol_*/             # Setiap tombol aksi (CRUD)
├── admin/proses_import/        # Phpspreadsheet
├── phpqrcode/                  # Library QR
├── css/ & js/                  # Styling & script
├── troubleshooting/            # Panduan + screenshot konfigurasi iSpring
└── db_cbtsekolah.sql           # Schema database
```

---

## 6. KELEMAHAN & PELUANG PERBAIKAN (untuk Rebuild Modern)

### Kelemahan Saat Ini
1. **Tidak scalable** — Semua logic di satu codebase PHP native
2. **Security rendah** — SQL injection risk (raw mysqli query), password MD5
3. **Tidak ada API** — Sulit integrasi dengan sistem lain
4. **Hardcoded path** — `file_soal` di login table berisi path absolut Windows
5. **Monitoring pakai polling** — Bukan WebSocket/real-time
6. **Tidak ada role-based access control** yang proper
7. **iSpring integration rapuh** — Bergantung pada konfigurasi manual di iSpring + parsing XML manual

### 8.4 Detail Teknis Integrasi iSpring (Update dari Repo Resmi)

#### Struktur XML Detailed Results (`dr`)
Setiap kali peserta menyelesaikan quiz, iSpring mengirim XML yang berisi:

```xml
<report>
  <quiz title="..." score="..." maxScore="..." passingScore="...">
    <question id="Q1" type="choice" status="correct" 
              awardedPoints="10" maxPoints="10" usedAttempts="1">
      <body>Teks pertanyaan...</body>
      <userAnswer>China and Nepal</userAnswer>
      <correctAnswer>China and Nepal</correctAnswer>
    </question>
    ...
  </quiz>
</report>
```

#### Rekomendasi Parsing di Sistem Modern
- Gunakan library XML parser (SimpleXML / DOMDocument di PHP, atau `xml2js` / `fast-xml-parser` di Node.js)
- Simpan **XML mentah** di kolom `detail` (untuk backup)
- Parse dan simpan data terstruktur di tabel terpisah:
  - `hasil_tes_detail` (per pertanyaan)
  - Kolom: `no_id`, `mapel`, `question_id`, `status`, `awarded_points`, `user_answer`, `correct_answer`

#### Best Practice untuk Rebuild
- Buat **webhook endpoint** khusus: `/api/ispring/webhook`
- Validasi request hanya dari iSpring (IP whitelist atau secret token)
- Log semua incoming POST untuk debugging
- Gunakan queue (Redis/BullMQ) untuk proses parsing XML yang berat
- Simpan file XML mentah di storage (untuk audit)

### Rekomendasi untuk Versi Modern
- **Backend**: Laravel / Node.js (Express/Fastify) dengan REST API + JWT
- **Frontend**: React / Vue / Next.js (SPA atau SSR)
- **Database**: MySQL/PostgreSQL + migration
- **Autentikasi**: Laravel Sanctum / JWT / OAuth
- **Soal**: 
  - Opsi 1: Tetap gunakan iSpring (embed iframe + webhook hasil)
  - Opsi 2: Buat soal native (multiple choice, essay, dll) dengan editor modern
- **Real-time**: Gunakan Laravel Echo + Pusher / Socket.io / Supabase Realtime
- **Arsitektur**:
  - Microservices (opsional): User Service, Exam Service, Result Service
  - Atau Modular Monolith
- **Fitur Tambahan**:
  - Anti-cheat (tab switch detection, fullscreen lock)
  - Auto-save jawaban
  - Randomisasi soal & pilihan
  - Dashboard analytics yang lebih kaya
  - Mobile responsive + PWA

---

## 7. KESIMPULAN

Aplikasi **CBT Sekolah** adalah solusi CBT sederhana namun lengkap yang mengandalkan **iSpring QuizMaker** sebagai sumber soal. Kekuatannya terletak pada kemudahan penggunaan untuk sekolah dan fitur pengawasan ruangan yang baik. Kelemahannya adalah arsitektur lama dan ketergantungan pada teknologi eksternal (iSpring).

Dokumen ini menjadi dasar yang solid untuk membangun ulang aplikasi yang lebih modern, aman, scalable, dan fleksibel — baik dengan tetap menggunakan iSpring maupun dengan sistem soal native.

---

**Disusun untuk keperluan rebuild aplikasi CBT modern**  
**Tanggal analisis**: 23 Mei 2026
