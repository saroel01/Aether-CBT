# Panduan Pengujian Lab Aether CBT

## 1. Tujuan

Dokumen ini berfungsi sebagai panduan uji coba laboratorium untuk mengevaluasi kesiapan aplikasi Aether CBT sebelum digunakan dalam pilot sekolah atau deployment nyata. Pengujian dilakukan untuk menilai:

- stabilitas startup aplikasi;
- kelayakan alur autentikasi dan otorisasi;
- kekuatan keamanan sesi, token, dan tenant;
- integritas data hasil ujian;
- performa pada skenario multi-user;
- kesiapan backup dan restore;
- keberhasilan operasi admin, supervisor, dan siswa.

## 2. Ruang Lingkup

Pengujian ini mencakup:

- login admin, siswa, dan supervisor;
- alur pelaksanaan ujian siswa;
- validasi token dan sesi ujian aktif;
- webhook hasil iSpring;
- monitoring ruang pengawas;
- pengelolaan data admin;
- keamanan akses dan tenant isolation;
- load test terbatas di lingkungan lab;
- backup dan restore data penting.

## 3. Persyaratan Lingkungan Lab

### 3.1 Infrastruktur

- 1 mesin server atau VM untuk menjalankan aplikasi
- 1 workstation untuk pengujian browser
- Jaringan lokal yang memungkinkan akses aplikasi dari browser
- Akses ke database SQLite local atau folder data aplikasi

### 3.2 Perangkat Lunak

- Go 1.22+
- Node.js 18+
- Git
- Browser: Chrome atau Edge terbaru
- Terminal/PowerShell

### 3.3 Variabel Lingkungan

Pastikan variabel berikut disiapkan sebelum menjalankan aplikasi:

```bash
JWT_SECRET=your_secure_secret_here
CORS_ALLOWED_ORIGINS=http://localhost:5173
PORT=3000
DATABASE_URL=data/cbt_aether.db
```

Catatan:
- `JWT_SECRET` wajib diisi agar aplikasi dapat berjalan dengan aman.
- `CORS_ALLOWED_ORIGINS` harus dibatasi pada origin yang benar-benar diizinkan.
- Jangan menggunakan secret default untuk lingkungan nyata.

## 4. Persiapan Pelaksanaan Uji Coba

### 4.1 Langkah Persiapan

1. Clone repository ke mesin lab.
2. Pastikan dependency telah terinstall.
3. Jalankan aplikasi dengan perintah berikut:

```bash
npm run dev
```

4. Jika diperlukan, jalankan seeding data awal:

```bash
npm run seed
```

5. Siapkan akun uji coba:
   - admin test
   - siswa test
   - supervisor test
   - tenant test

### 4.2 Data Uji Coba

Gunakan data dummy dan non-produktif. Hindari penggunaan data sekolah asli, validasi siswa nyata, atau payload ujian produksi.

### 4.3 Dokumentasi Hasil

Setiap pengujian harus dicatat dengan format:

- ID pengujian
- Nama skenario
- Waktu pelaksanaan
- Pelaku pengujian
- Hasil: Pass / Fail / Blocked
- Catatan detail
- Tindakan perbaikan jika diperlukan

## 5. Checklist Persiapan Sebelum Pengujian

- [ ] Aplikasi dapat dijalankan tanpa error startup.
- [ ] Database berhasil dibuat.
- [ ] Migrasi otomatis berjalan.
- [ ] Login page dapat diakses.
- [ ] Data seed tersedia jika diperlukan.
- [ ] Akun uji coba siap dibuat.
- [ ] Browser test siap digunakan.
- [ ] Log aplikasi dipantau selama pengujian.
- [ ] Folder backup dan data ujian siap diawasi.

## 6. Skenario Pengujian

### TC-01: Startup Aplikasi

**Tujuan**
Menguji apakah aplikasi dapat berjalan dengan benar setelah startup.

**Langkah**
1. Jalankan aplikasi dari root project.
2. Tunggu proses startup selesai.
3. Buka halaman utama aplikasi.
4. Amati log console dan server log.

**Kriteria Diterima**
- Aplikasi berjalan tanpa crash.
- Database dapat dibuat atau dibuka.
- Tidak terdapat error fatal di log.
- Halaman login atau halaman awal dapat diakses.

**Status**
- Pass / Fail / Blocked

---

### TC-02: Login Admin

**Tujuan**
Menguji validasi autentikasi admin.

**Langkah**
1. Buka halaman login admin.
2. Masukkan username admin yang valid.
3. Masukkan password valid.
4. Klik Login.
5. Ulangi dengan password salah.
6. Ulangi dengan username tidak terdaftar.

**Kriteria Diterima**
- Login berhasil untuk akun valid.
- Login gagal untuk kredensial salah.
- Sistem tidak menampilkan detail sensitif.
- Tidak terjadi crash atau loop login.

**Status**
- Pass / Fail / Blocked

---

### TC-03: Login Siswa

**Tujuan**
Menguji autentikasi dan validasi token siswa.

**Langkah**
1. Masuk ke halaman login siswa.
2. Gunakan nomor induk dan password valid.
3. Masukkan token ujian yang benar.
4. Uji dengan token salah.
5. Uji dengan akun siswa tidak aktif.

**Kriteria Diterima**
- Siswa dapat login hanya dengan kombinasi data yang valid.
- Token ujian wajib valid.
- Akses ditolak untuk akun atau token tidak valid.
- Informasi error aman dan jelas.

**Status**
- Pass / Fail / Blocked

---

### TC-04: Login Supervisor / Ruang

**Tujuan**
Menguji autentikasi bagian monitoring supervisor.

**Langkah**
1. Buka halaman supervisor.
2. Login dengan akun supervisor valid.
3. Login dengan akun yang tidak berhak.
4. Coba akses halaman monitor ruang tanpa login.

**Kriteria Diterima**
- Supervisor dapat masuk ke area yang sesuai.
- Akses ditolak untuk role yang salah.
- Tidak ada bypass otorisasi.

**Status**
- Pass / Fail / Blocked

---

### TC-05: Alur Mulai Ujian

**Tujuan**
Menguji kemampuan siswa memulai ujian dengan benar.

**Langkah**
1. Login sebagai siswa.
2. Pilih mata pelajaran atau subjek ujian.
3. Klik tombol mulai ujian.
4. Periksa tampilan halaman ujian.
5. Cek apakah soal tampil lengkap.

**Kriteria Diterima**
- Ujian dapat dimulai tanpa error.
- Soal ditampilkan sesuai mapel yang dipilih.
- System mencatat sesi aktif dan token ujian.
- Siswa tidak bisa memulai lebih dari satu sesi aktif yang tidak sah.

**Status**
- Pass / Fail / Blocked

---

### TC-06: Penyimpanan Jawaban dan Submit Ujian

**Tujuan**
Menguji integritas jawaban siswa selama ujian.

**Langkah**
1. Mulai ujian baru.
2. Jawab beberapa soal.
3. Simpan atau lanjutkan ke soal berikutnya.
4. Submit ujian.
5. Cek apakah status submit berubah menjadi sukses.

**Kriteria Diterima**
- Jawaban disimpan dengan benar.
- Submit akhir berhasil.
- Data tidak hilang saat berpindah soal.
- Siswa tidak bisa submit tanpa sesi yang valid.

**Status**
- Pass / Fail / Blocked

---

### TC-07: Validasi Token dan Sesi Ujian

**Tujuan**
Menguji keamanan sesi dan integritas submit.

**Langkah**
1. Login siswa dan mulai ujian.
2. Catat `attempt_token` atau token sesi aktif.
3. Coba submit hasil tanpa token.
4. Coba submit dengan token salah.
5. Coba submit setelah sesi berakhir.
6. Coba submit dari akun lain.

**Kriteria Diterima**
- Submit ditolak untuk token tidak valid.
- Submit ditolak di luar sesi aktif.
- Data tidak dapat tertukar antar siswa.
- Aplikasi menolak akses tidak sah.

**Status**
- Pass / Fail / Blocked

---

### TC-08: Pengujian Webhook iSpring

**Tujuan**
Menguji penerimaan hasil ujian yang dikirim dari iSpring.

**Langkah**
1. Siapkan payload iSpring valid.
2. Kirim ke endpoint webhook.
3. Uji payload dengan XML tidak valid.
4. Uji payload dengan field yang hilang.
5. Uji payload dengan token atau sesi tidak valid.
6. Cek database hasil tes.

**Kriteria Diterima**
- Payload valid diterima dan disimpan.
- Payload invalid ditolak dengan error yang jelas.
- Data hasil tersimpan ke tabel yang benar.
- Tidak terjadi duplikasi data.
- Tidak ada hasil ujian yang masuk tanpa validasi yang tepat.

**Status**
- Pass / Fail / Blocked

---

### TC-09: Validasi Tenant Isolation

**Tujuan**
Menguji bahwa data tenant tidak bercampur antar sekolah atau ruang.

**Langkah**
1. Login ke tenant A.
2. Akses data siswa, mapel, atau ujian tenant A.
3. Ubah header tenant atau akses data tenant B.
4. Coba akses route yang seharusnya dibatasi.

**Kriteria Diterima**
- Pengguna hanya melihat data tenant yang berhak.
- Data tenant lain tidak terlihat atau tidak bisa diakses.
- Middleware tenant bekerja sesuai konteks.

**Status**
- Pass / Fail / Blocked

---

### TC-10: Validasi Akses Admin dan Supervisor

**Tujuan**
Menguji batasan peran pengguna.

**Langkah**
1. Login sebagai admin.
2. Akses halaman yang hanya supervisor yang boleh buka.
3. Login sebagai supervisor.
4. Akses halaman admin.
5. Login sebagai siswa.
6. Akses route admin atau supervisor.

**Kriteria Diterima**
- Role yang tidak berhak ditolak.
- Akses sesuai hak peran.
- Tidak ada route yang bisa diakses tanpa otorisasi.

**Status**
- Pass / Fail / Blocked

---

### TC-11: Load Test Sederhana pada Lab

**Tujuan**
Menilai ketahanan aplikasi pada skenario multi-user terbatas.

**Langkah**
1. Buat skenario login bersamaan untuk 20 user.
2. Lanjutkan dengan 50 user dan 100 user jika kapasitas lab mencukupi.
3. Jalankan submit ujian dalam jumlah bersamaan.
4. Amati waktu respon, log, dan hasil database.

**Kriteria Diterima**
- Aplikasi tetap responsif selama skenario load kecil.
- Tidak ada data hilang.
- Tidak ada hasil submit yang gagal tanpa alasan jelas.
- Tidak terjadi deadlock atau crash server.

**Status**
- Pass / Fail / Blocked

---

### TC-12: Backup, Restore, dan Pemulihan Data

**Tujuan**
Menguji kesiapan operasi saat data perlu dipulihkan.

**Langkah**
1. Lakukan backup database dan file data penting.
2. Ubah data atau tambahkan data uji coba.
3. Restore data ke lingkungan test.
4. Bandingkan hasil sebelum dan sesudah restore.

**Kriteria Diterima**
- Backup berhasil dibuat.
- Restore berhasil dijalankan.
- Data kembali konsisten.
- Tidak ada data penting yang hilang.

**Status**
- Pass / Fail / Blocked

---

### TC-13: Keamanan Dasar Akses API

**Tujuan**
Menguji pengamanan endpoint aplikasi.

**Langkah**
1. Akses endpoint API tanpa autentikasi.
2. Gunakan token valid untuk endpoint tidak sah.
3. Kirim payload dengan ukuran besar ke webhook.
4. Coba akses origin yang tidak diizinkan.
5. Lakukan rate-limit test dengan permintaan berulang.

**Kriteria Diterima**
- Endpoint terlindungi.
- Akses tidak sah ditolak.
- Webhook merespons sesuai batasan keamanan.
- Request berbahaya tidak diterima.

**Status**
- Pass / Fail / Blocked

## 7. Kriteria Kelulusan Lab

Aplikasi dinyatakan layak untuk pilot terbatas jika semua kondisi berikut terpenuhi:

- startup aplikasi stabil;
- login admin, siswa, dan supervisor berhasil diuji;
- token dan sesi ujian aktif valid;
- hasil ujian tersimpan dengan benar;
- data tidak bercampur antar tenant;
- role dan akses ditolak sesuai ketentuan;
- webhook iSpring menerima dan menolak payload yang benar-benar invalid;
- pengujian load kecil berjalan tanpa kehilangan data;
- backup dan restore berhasil dipraktikkan.

Jika ada satu atau lebih item gagal, maka aplikasi belum siap untuk pilot resmi dan harus diperbaiki sebelum ujicoba lanjutan.

## 8. Formulir Rekap Hasil Uji Coba

| ID Skenario | Nama Skenario | Hasil | Catatan | Tindakan |
|---|---|---|---|---|
| TC-01 | Startup Aplikasi | Pass / Fail / Blocked | | |
| TC-02 | Login Admin | Pass / Fail / Blocked | | |
| TC-03 | Login Siswa | Pass / Fail / Blocked | | |
| TC-04 | Login Supervisor | Pass / Fail / Blocked | | |
| TC-05 | Mulai Ujian | Pass / Fail / Blocked | | |
| TC-06 | Submit Ujian | Pass / Fail / Blocked | | |
| TC-07 | Token & Sesi | Pass / Fail / Blocked | | |
| TC-08 | Webhook iSpring | Pass / Fail / Blocked | | |
| TC-09 | Tenant Isolation | Pass / Fail / Blocked | | |
| TC-10 | Akses Role | Pass / Fail / Blocked | | |
| TC-11 | Load Test | Pass / Fail / Blocked | | |
| TC-12 | Backup/Restore | Pass / Fail / Blocked | | |
| TC-13 | Keamanan API | Pass / Fail / Blocked | | |

## 9. Kesimpulan

Dokumen ini dirancang untuk memberi panduan yang jelas, terukur, dan mudah diikuti oleh tim lab atau operator sekolah sebelum penggunaan aplikasi Aether CBT pada skala lebih besar. Fokus utama pengujian bukan hanya pada fitur, tetapi pada integritas data, keamanan, dan keandalan operasional.

Jika semua skenario di atas lolos, maka aplikasi masuk dalam kategori siap untuk pilot terbatas. Jika ada kegagalan pada aspek keamanan, validasi sesi, atau integritas hasil ujian, maka aplikasi harus dikembalikan ke tahapan perbaikan sebelum digunakan dalam event nyata.
