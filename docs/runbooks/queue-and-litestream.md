# Runbook: Filesystem Queue & Litestream Backup

**Untuk:** Admin sekolah  
**Terakhir diperbarui:** 26 Mei 2026

## Gambaran Umum

Aether CBT memakai filesystem queue untuk memproses hasil ujian iSpring. Setiap submission disimpan sebagai file JSON di `data/queue/`, lalu worker memproses file tersebut di belakang layar.

Alurnya:

1. iSpring mengirim hasil ujian ke server.
2. Server memvalidasi sesi dan token, lalu menyimpan submission ke `pending/`.
3. Worker memindahkan file ke `processing/` dan menyimpan hasil ke database.
4. Jika sukses, file pindah ke `done/`.
5. Jika gagal, file dicoba ulang hingga batas retry, lalu pindah ke `failed/`.

Litestream tetap digunakan untuk membackup database SQLite. Folder queue perlu ikut dibackup oleh mekanisme backup server biasa jika sekolah ingin menyimpan jejak submission mentah.

## Cara Cek Status Antrean

Buka browser dan akses setelah login sebagai admin/supervisor:

```text
GET /api/debug/queue
```

Contoh respons:

```json
{
  "success": true,
  "data": {
    "pending_count": 5,
    "processing_count": 1,
    "failed_count": 0,
    "done_count": 120
  },
  "message": "Queue status retrieved"
}
```

Arti field:

- `pending_count`: file yang menunggu diproses.
- `processing_count`: file yang sedang diproses worker.
- `failed_count`: file yang gagal permanen setelah retry habis.
- `done_count`: file yang sudah berhasil diproses dan disimpan sebagai arsip sementara.

Jika `pending_count` dan `processing_count` sama-sama `0`, antrean aktif sudah habis.

## Memantau Queue via File Explorer

Default lokasi queue:

```text
data/queue/
```

Isi folder:

- `pending/`: submission yang menunggu diproses.
- `processing/`: submission yang sedang dipegang worker.
- `done/`: submission yang sudah berhasil diproses.
- `failed/`: submission yang gagal permanen. Buka file `.json` untuk melihat `no_id`, `validasi`, `retry_count`, dan `last_error`.
- `tmp/`: area tulis sementara. File sisa di sini akan dihapus otomatis saat startup.

Nama file mengikuti pola:

```text
<unix_nano>-<tenant_id>-<no_id>-<8hex>.json
```

## Jika Ada Job di Failed

Langkah investigasi:

1. Cek log server untuk pesan `[WORKER] batch process error` atau `[WORKER] PANIC recovered`.
2. Buka file `.json` di `data/queue/failed/`.
3. Lihat `last_error`, `no_id`, `validasi`, dan `retry_count`.
4. Perbaiki penyebabnya, misalnya data siswa/mapel hilang atau sesi sudah kedaluwarsa.

Jika perlu reprocess manual, hentikan server dulu, pindahkan file `.json` dari `failed/` ke `pending/`, lalu turunkan `retry_count` atau kosongkan `last_error` jika diperlukan. Nyalakan server kembali agar worker mengambil file tersebut.

## Konfigurasi Env Var Queue

| Env var | Default | Fungsi |
|---|---:|---|
| `QUEUE_DIR` | `data/queue` | Root direktori queue. `tmp/` harus berada di volume yang sama dengan folder lain. |
| `QUEUE_MAX_RETRIES` | `5` | Jumlah retry sebelum file masuk `failed/`. |
| `QUEUE_BATCH_SIZE` | `5` | Jumlah maksimal job dalam satu transaksi worker. |
| `QUEUE_BATCH_TIMEOUT_MS` | `100` | Waktu tunggu maksimal untuk mengisi batch. |
| `QUEUE_STUCK_THRESHOLD_MIN` | `5` | Batas umur file di `processing/` sebelum dianggap stuck saat recovery normal. |
| `QUEUE_DONE_RETENTION_DAYS` | `7` | Lama penyimpanan file di `done/` sebelum dibersihkan otomatis. |

## Migrasi dari Versi Lama

Tabel `submission_queue` lama tetap ada di database untuk kompatibilitas upgrade. Saat startup, server akan:

1. Membuat folder `pending/`, `processing/`, `done/`, `failed/`, dan `tmp/`.
2. Membersihkan file sisa di `tmp/`.
3. Memindahkan file stuck dari `processing/` ke `pending/`.
4. Mengonversi baris `submission_queue` lama yang berstatus `pending` atau `processing` menjadi file JSON di `pending/`.
5. Menghapus hanya baris legacy yang berhasil dimigrasi.

Setelah upgrade, pemantauan operasional dilakukan lewat folder queue dan `/api/debug/queue`, bukan lagi lewat SQL `submission_queue`.

## Menjalankan Litestream

### Docker

```bash
docker run -d \
  --name litestream \
  --restart unless-stopped \
  -v ./data:/data \
  -v ./backups:/backups \
  -v ./docs/litestream-config.example.yml:/etc/litestream.yml \
  litestream/litestream replicate -config /etc/litestream.yml
```

### Systemd

Buat file `/etc/systemd/system/litestream.service`:

```ini
[Unit]
Description=Litestream DB Replication
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/litestream replicate -config /path/to/litestream.yml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable litestream
sudo systemctl start litestream
```

### Windows Manual

```powershell
.\litestream.exe replicate -config docs\litestream-config.example.yml
```

## Cara Restore Database

1. Stop server Aether CBT.
2. Restore database dari backup.
3. Pastikan folder `data/queue/` juga dikembalikan jika tersedia.
4. Start server.

Contoh restore:

```bash
litestream restore -config litestream.yml -o data/cbt_aether.db
```

Verifikasi:

1. Jalankan server.
2. Cek `/api/health`.
3. Cek `/api/debug/queue`.
4. Jika ada file di `processing/`, restart server. Startup recovery akan memindahkannya ke `pending/`.

## Troubleshooting

### Worker tidak memproses job

- Cek log untuk pesan `[WORKER]`.
- Pastikan server berjalan, karena worker hidup di proses server yang sama.
- Cek `/api/debug/queue` untuk melihat apakah file tertahan di `pending/` atau `processing/`.
- Restart server jika perlu.

### Job stuck di processing setelah restart

- Ini normal jika server mati di tengah pemrosesan.
- Restart server. Startup recovery akan memindahkan file dari `processing/` ke `pending/`.
- Worker akan mengambil kembali file tersebut setelah server hidup.

### Database locked / busy timeout

- Filesystem queue menghindari kontensi handler terhadap tabel queue lama.
- Jika masih sering terjadi, cek ukuran batch lewat `QUEUE_BATCH_SIZE`.
- Default batch `5` biasanya cukup untuk ujian sekolah.

### Backup Litestream tidak jalan

- Cek path database di konfigurasi Litestream.
- Pastikan folder backup writable.
- Cek log Litestream untuk error detail.

## Checklist Sebelum Ujian

- [ ] Server berjalan dan `/api/health` OK.
- [ ] Litestream aktif dan backup berjalan.
- [ ] Queue aktif bersih: `pending_count = 0`, `processing_count = 0`, `failed_count = 0`.
- [ ] Rate limiter webhook cukup longgar untuk jam ujian.
- [ ] Log monitoring aktif.

## Checklist Setelah Ujian

- [ ] Tunggu `pending_count = 0` dan `processing_count = 0`.
- [ ] Cek `failed_count`; jika ada, investigasi file di `data/queue/failed/`.
- [ ] Verifikasi semua hasil ujian tersimpan.
- [ ] Backup database manual sebagai snapshot.
