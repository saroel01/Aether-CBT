# Prosedur Backup & Restore Database Aether CBT

**Versi:** 1.0  
**Tanggal:** 25 Mei 2026  
**Status:** Siap digunakan

---

## 1. Latar Belakang

Aether CBT menggunakan **SQLite dalam mode WAL (Write-Ahead Logging)** untuk performa dan konkurensi yang lebih baik. Karena sifat WAL, backup database tidak boleh dilakukan hanya dengan menyalin file `.db` secara langsung saat aplikasi sedang berjalan.

Dokumen ini menjelaskan cara backup dan restore yang **aman dan direkomendasikan**.

---

## 2. Alat yang Tersedia

| Alat                    | Lokasi                        | Keterangan                              |
|-------------------------|-------------------------------|-----------------------------------------|
| Backup Tool (Go)        | `scripts/backup.go`           | Melakukan backup atomik menggunakan `VACUUM INTO` |
| Backup Wrapper (PS)     | `scripts/backup.ps1`          | Wrapper PowerShell yang mudah digunakan |
| Restore Tool (PS)       | `scripts/restore.ps1`         | Prosedur restore dengan konfirmasi & safety |
| Dokumentasi             | `docs/backup-restore.md`      | File ini                              |

---

## 3. Cara Melakukan Backup

### Metode yang Direkomendasikan (Paling Aman)

Gunakan script PowerShell:

```powershell
.\scripts\backup.ps1
```

Atau dengan parameter:

```powershell
.\scripts\backup.ps1 -Database "data/cbt_aether.db" -Output "backups"
```

**Yang dilakukan script:**
1. Membuka database dengan mode WAL yang benar.
2. Menjalankan `VACUUM INTO` (backup atomik).
3. Melakukan `integrity_check` pada file backup.
4. Menyimpan hasil di folder `backups/` dengan nama:
   - `cbt_aether_YYYYMMDD_HHmmss.db`

### Hasil Backup

Setiap backup yang berhasil akan berisi:
- File database lengkap dan konsisten.
- Ukuran yang hampir sama dengan database asli.
- Status integrity = `ok`.

---

## 4. Cara Melakukan Restore

### Peringatan Penting

> **HENTIKAN** aplikasi server sebelum melakukan restore!

### Langkah-langkah

1. Jalankan script restore:

   ```powershell
   .\scripts\restore.ps1 -Backup "backups\cbt_aether_20260525_115405.db"
   ```

2. Script akan:
   - Memberi peringatan keras.
   - Meminta konfirmasi dengan mengetik `YA`.
   - Membuat cadangan database lama (`.before-restore-...`).
   - Mengganti file database.
   - Membersihkan file WAL dan SHM lama.

3. Setelah restore selesai, jalankan kembali aplikasi.

---

## 5. Rekomendasi Operasional

| Situasi                          | Rekomendasi Backup                          |
|----------------------------------|---------------------------------------------|
| Sebelum deployment / update      | Wajib                                       |
| Sebelum ujian pilot              | Wajib                                       |
| Setiap hari (production)         | Disarankan (bisa dijadwalkan via Task Scheduler) |
| Setelah ujian selesai            | Sangat disarankan                           |
| Sebelum rotasi credential        | Wajib                                       |

**Lokasi penyimpanan backup yang baik:**
- Folder `backups/` di dalam proyek (untuk development)
- Drive terpisah atau cloud storage (untuk production)

---

## 6. Catatan Teknis

- Driver yang digunakan: `modernc.org/sqlite` (pure Go).
- `VACUUM INTO` adalah metode paling aman dan direkomendasikan untuk WAL mode.
- Script backup **bisa dijalankan** saat aplikasi sedang berjalan (online backup).
- Script restore **harus** dilakukan saat aplikasi berhenti.

---

## 7. Troubleshooting

| Masalah                          | Solusi |
|----------------------------------|--------|
| `go: command not found`          | Pastikan Go sudah terinstall dan ada di PATH |
| Backup gagal integrity check     | Jangan gunakan file tersebut. Coba backup ulang |
| Restore gagal karena file terkunci | Pastikan aplikasi benar-benar sudah dimatikan |
| File WAL/SHM masih tersisa       | Hapus manual file `.db-wal` dan `.db-shm` |

---

## 8. Pengembangan Selanjutnya (Opsional)

- Membuat versi Go untuk restore juga.
- Menambahkan kompresi backup (zip).
- Integrasi dengan scheduled task / cron.
- Notifikasi ke Telegram/Discord setelah backup berhasil.

---

**Dokumentasi ini dibuat sebagai bagian dari Task 1.2 - Phase 1 Pre-Pilot Readiness.**
