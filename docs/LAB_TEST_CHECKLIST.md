# Checklist Uji Coba Lab Aether CBT

## 1. Tujuan

Menguji kesiapan aplikasi Aether CBT sebelum pilot sekolah atau deployment nyata. Fokus utama:

- aplikasi bisa berjalan normal
- login dan akses sesuai role
- token/sesi ujian aman
- hasil ujian tersimpan dengan benar
- data tenant terisolasi
- performa dan recovery data cukup memadai

## 2. Persiapan Lab

- [ ] Server/lab siap dipakai
- [ ] Go 1.22+ terinstall
- [ ] Node.js 18+ terinstall
- [ ] Browser tersedia (Chrome/Edge)
- [ ] Variabel lingkungan siap:
  - `JWT_SECRET`
  - `CORS_ALLOWED_ORIGINS`
  - `PORT`
  - `DATABASE_URL`
- [ ] Aplikasi dijalankan dengan `npm run dev`
- [ ] Data seed siap jika dibutuhkan
- [ ] Akun test admin, siswa, supervisor dibuat

## 3. Checklist Pengujian

### A. Startup dan Kesiapan Aplikasi

- [ ] Server berhasil start tanpa error fatal
- [ ] Database berhasil dibuat/terbuka
- [ ] Migrasi otomatis berjalan
- [ ] Halaman login bisa diakses
- [ ] Tidak ada crash saat membuka halaman utama

### B. Login dan Otorisasi

- [ ] Login admin berhasil dengan akun valid
- [ ] Login admin gagal dengan password salah
- [ ] Login siswa berhasil dengan data valid
- [ ] Login siswa gagal dengan token salah
- [ ] Login supervisor berhasil
- [ ] Role tidak berhak ditolak
- [ ] Akses tanpa login ditolak

### C. Alur Ujian Siswa

- [ ] Siswa bisa memilih mata pelajaran/ujian
- [ ] Ujian dapat dimulai
- [ ] Soal tampil dengan benar
- [ ] Jawaban bisa tersimpan
- [ ] Submit ujian berhasil
- [ ] Siswa tidak bisa submit ulang tanpa hak

### D. Keamanan Sesi dan Token

- [ ] Attempt token atau sesi aktif terdeteksi
- [ ] Submit tanpa token ditolak
- [ ] Submit dengan token salah ditolak
- [ ] Submit setelah sesi berakhir ditolak
- [ ] Submit antar siswa tidak bisa menukar data

### E. iSpring Webhook dan Hasil Ujian

- [ ] Payload valid diterima
- [ ] Payload invalid ditolak
- [ ] XML yang rusak ditolak
- [ ] Data hasil tersimpan di tabel yang benar
- [ ] Tidak ada duplikasi data hasil ujian

### F. Tenant dan Akses Data

- [ ] Data tenant A tidak terlihat dari tenant B
- [ ] Admin hanya melihat data yang berhak
- [ ] Supervisor hanya melihat ruang yang relevan
- [ ] Siswa tidak mengakses halaman admin/supervisor

### G. Monitoring dan Admin

- [ ] Dashboard admin terbuka
- [ ] Data siswa, kelas, mapel, ruang tampil benar
- [ ] Export hasil ujian berjalan
- [ ] Monitoring supervisor menampilkan status ruang
- [ ] Reset/intervensi sesi bekerja bila diperlukan

### H. Load Test Ringkas

- [ ] 20 user login bersamaan berfungsi
- [ ] 50 user bersamaan berfungsi
- [ ] 100 user bersamaan masih aman (jika lab memungkinkan)
- [ ] Tidak ada data hilang saat submit bersamaan
- [ ] Waktu respon masih masuk akal

### I. Backup dan Restore

- [ ] Backup database berhasil dibuat
- [ ] Backup data penting berhasil dibuat
- [ ] Restore berjalan dengan sukses
- [ ] Data pulih lengkap dan konsisten
- [ ] Tidak ada data yang hilang setelah restore

### J. Keamanan Dasar

- [ ] Endpoint API terlindungi
- [ ] Akses tanpa auth ditolak
- [ ] Request dengan token invalid ditolak
- [ ] Payload besar ke webhook dibatasi
- [ ] Origin yang tidak diizinkan ditolak
- [ ] Rate limiting atau pembatasan akses berfungsi

## 4. Kriteria Kelulusan

Aplikasi dinyatakan siap untuk pilot terbatas jika semua item berikut terpenuhi:

- [ ] Startup aplikasi stabil
- [ ] Login berhasil untuk admin, siswa, dan supervisor
- [ ] Token/sesi ujian aman
- [ ] Hasil ujian tersimpan valid dan konsisten
- [ ] Tenant terisolasi dengan baik
- [ ] Akses role sesuai ketentuan
- [ ] Webhook iSpring berjalan aman
- [ ] Load test kecil tidak mengakibatkan kehilangan data
- [ ] Backup dan restore berhasil

## 5. Rekap Hasil

| No | Uji Coba | Hasil | Catatan |
|---|---|---|---|
| 1 | Startup | Pass / Fail / Blocked | |
| 2 | Login Admin | Pass / Fail / Blocked | |
| 3 | Login Siswa | Pass / Fail / Blocked | |
| 4 | Login Supervisor | Pass / Fail / Blocked | |
| 5 | Alur Ujian | Pass / Fail / Blocked | |
| 6 | Token & Sesi | Pass / Fail / Blocked | |
| 7 | Webhook iSpring | Pass / Fail / Blocked | |
| 8 | Tenant Isolation | Pass / Fail / Blocked | |
| 9 | Load Test | Pass / Fail / Blocked | |
| 10 | Backup/Restore | Pass / Fail / Blocked | |

## 6. Catatan Akhir

- Jika ada item gagal pada keamanan, token, atau integritas hasil ujian, aplikasi belum siap untuk pilot resmi.
- Prioritaskan perbaikan berdasarkan: keamanan, integritas data, lalu performa.
- Setelah semua item lolos, aplikasi dapat dipertimbangkan untuk uji coba terbatas di lab.
