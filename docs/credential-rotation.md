# Prosedur Rotasi Kredensial (Credential Rotation Workflow)

**Versi:** 1.0  
**Tanggal:** 25 Mei 2026  
**Status:** Wajib dilakukan sebelum setiap pilot dan deployment

---

## 1. Tujuan

Dokumen ini menetapkan **prosedur wajib** untuk merotasi semua kredensial penting sebelum ujian pilot atau deployment ke lingkungan produksi. Tujuannya adalah mencegah penggunaan password default yang mudah ditebak.

---

## 2. Kredensial yang WAJIB Dirotasi

| No | Jenis Kredensial            | Lokasi Tabel / File          | Risiko jika Tidak Dirotasi | Prioritas |
|----|-----------------------------|------------------------------|----------------------------|---------|
| 1  | Admin (Super Admin)         | `users` (role = admin)       | Akses penuh ke seluruh sistem | **Sangat Tinggi** |
| 2  | Pengawas Ruang              | `ruang` (username + password_hash) | Bisa mengawasi dan memanipulasi ujian | **Tinggi** |
| 3  | Siswa / Peserta             | `peserta` (password)         | Siswa bisa login dengan password default | **Tinggi** |
| 4  | Global Token Ujian (`token`) | `settings.token`            | Bisa bypass proteksi ujian | **Sangat Tinggi** |
| 5  | JWT_SECRET                  | Environment Variable         | Bisa memalsukan token siapa saja | **Kritis** |

> **Catatan:** Semua password harus disimpan dalam bentuk **bcrypt hash**. Jangan pernah menyimpan plaintext kecuali untuk keperluan migrasi legacy.

---

## 3. Kapan Harus Melakukan Rotasi

Rotasi kredensial **WAJIB** dilakukan pada situasi berikut:

| Situasi                                      | Kapan Dilakukan          | Siapa yang Bertanggung Jawab |
|----------------------------------------------|--------------------------|------------------------------|
| Sebelum Pilot Ujian Pertama                  | Minimal 1–2 hari sebelum | Admin Sekolah + Developer    |
| Sebelum Deployment ke Produksi               | Setiap kali deploy       | Developer + Admin            |
| Setelah ada pergantian admin/pengawas        | Segera setelah pergantian| Admin Sekolah                |
| Setelah insiden keamanan atau kebocoran      | Segera                   | Developer + Admin            |
| Setiap 3–6 bulan (jadwal rutin)              | Terjadwal                | Admin Sekolah                |

---

## 4. Prosedur Rotasi (Langkah demi Langkah)

### 4.1 Persiapan

1. Buat backup database terlebih dahulu (lihat `docs/backup-restore.md`).
2. Siapkan environment production dengan `JWT_SECRET` yang kuat (minimal 32 karakter acak).

### 4.2 Rotasi Admin

- Gunakan tool `cmd/createadmin` atau buat admin baru via API.
- **Jangan** pernah menggunakan password `admin123` di produksi.
- Hapus atau nonaktifkan admin lama jika perlu.

### 4.3 Rotasi Pengawas Ruang

- Buat ulang data ruang dengan password yang berbeda untuk setiap ruang.
- Hindari pola yang mudah ditebak (contoh: `ruang_a`, `ruang123`).

### 4.4 Rotasi Password Siswa

- Saat impor CSV, **selalu** isi kolom password dengan nilai yang unik atau random.
- Hindari menggunakan password yang sama untuk semua siswa (`siswa123`).
- Jika memungkinkan, berikan password berbeda per siswa.

### 4.5 Rotasi Global Token

- Ganti nilai `settings.token` melalui menu Settings di admin panel.
- Token ini digunakan untuk validasi sesi ujian.

### 4.6 Rotasi JWT_SECRET

- Ini adalah rahasia paling penting.
- Ubah di file `.env` atau environment variable server.
- Setelah diubah, **semua token yang sudah dikeluarkan akan invalid**.

---

## 5. Rekomendasi Password yang Baik

Gunakan password dengan karakteristik berikut:

- Panjang minimum: **12 karakter**
- Kombinasi: Huruf besar + huruf kecil + angka + simbol
- Hindari kata yang ada di kamus atau nama sekolah
- Lebih baik gunakan **passphrase** (contoh: `KudaLautBiru!2026@CBT`)

Contoh password yang direkomendasikan:
- `P@nd4w4n4!2026`
- `UjianK3m1a2026#RPL`
- `RuangAkses!Xii-04`

---

## 6. Tools Pendukung

### Untuk Developer (yang punya source code)
- `scripts/generate-password.ps1`
- `scripts/generate-password.go`
- `cmd/createadmin/main.go`
- `cmd/seed/main.go`

### Untuk Produksi / Admin Sekolah (hanya punya hasil build)
- `aether-password-generator.exe` (dapat di-build dari `cmd/password-generator`)
- `docs/PANDUAN_ROTASI_KREDENSIAL_PRODUKSI.txt` (versi ringkas yang disertakan di rilis)

**Saran untuk setiap rilis:**
Selalu sertakan dua file berikut dalam paket distribusi:
1. `aether-password-generator.exe`
2. `PANDUAN_ROTASI_KREDENSIAL_PRODUKSI.txt`

---

## 7. Checklist Rotasi (Wajib Diisi)

Sebelum pilot/deployment, pastikan checklist berikut sudah selesai:

- [ ] Database sudah di-backup
- [ ] Password Admin baru (bukan `admin123`)
- [ ] Password semua ruang sudah diganti
- [ ] Password siswa tidak menggunakan default `siswa123`
- [ ] Global Token di settings sudah diganti
- [ ] `JWT_SECRET` di environment sudah diubah (minimal 32 karakter)
- [ ] Semua perubahan sudah didokumentasikan di log deployment
- [ ] Tim sudah diberitahu password baru melalui channel aman (bukan chat publik)

---

## 8. Penanggung Jawab

| Peran             | Tanggung Jawab |
|-------------------|----------------|
| Developer         | Rotasi JWT_SECRET, memastikan tidak ada hardcoded password di code |
| Admin Sekolah     | Rotasi password admin, ruang, siswa, dan global token |
| Ketua Pelaksana   | Memastikan checklist rotasi sudah diisi sebelum ujian |

---

**Dokumen ini dibuat sebagai bagian dari Task 1.3 - Phase 1 Pre-Pilot Readiness Aether CBT.**
