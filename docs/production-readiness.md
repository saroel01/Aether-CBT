# Production Readiness - Aether CBT

Last updated: 2026-05-26

## Status Singkat

Aplikasi sudah melewati verifikasi teknis utama untuk jalur kritis hasil ujian:

- `go test ./...` lulus.
- `npm run build` lulus.
- E2E queue/load skala 50, 100, 200, dan 500 lulus secara data integrity: semua submission tersimpan ke `hasil_tes`, 0 failed queue files.
- Filesystem queue sudah aktif, durable, recoverable, dan bisa dipantau lewat `/api/debug/queue` serta folder `data/queue/`.

Catatan penting: target HTTP P95 <100ms belum lulus untuk burst 200 dan 500. Hasil terakhir:

| Skala | Data Tersimpan | Failed | Drain | HTTP P95 |
|---:|---:|---:|---:|---:|
| 50 | 50/50 | 0 | 202ms | 45ms |
| 100 | 100/100 | 0 | 406ms | 90ms |
| 200 | 200/200 | 0 | 408ms | 380ms |
| 500 | 500/500 | 0 | 1.212s | 314ms |

## Keputusan Yang Perlu Diambil

1. Target pilot pertama:
   - Aman untuk pilot kecil sekitar 50-100 siswa serentak.
   - Untuk 200-500 siswa serentak, data integrity sudah baik, tetapi SLA penerimaan webhook perlu diterima sebagai ratusan ms atau dioptimasi lagi.

2. Target SLA:
   - Jika HTTP P95 <100ms wajib sampai 500 siswa, lanjutkan optimasi arsitektur.
   - Jika yang paling penting adalah tidak kehilangan hasil ujian, kondisi saat ini sudah jauh lebih dekat ke siap pilot.

3. Mode deployment:
   - Pilih Windows sekolah/lab lokal, VPS, atau container/Coolify.
   - Pilihan ini menentukan cara backup, auto-start service, reverse proxy, dan monitoring.

## Checklist Sebelum Dipakai Pilot

- [ ] Tentukan jumlah siswa serentak untuk pilot pertama.
- [ ] Putuskan apakah HTTP P95 <100ms wajib untuk semua skala atau hanya target ideal.
- [ ] Rotasi semua kredensial produksi: admin, ruang, siswa, global token.
- [ ] Pastikan `WEBHOOK_RATE_LIMIT_PER_MIN` cukup untuk jam ujian.
- [ ] Pastikan `QUEUE_DIR` berada di disk yang stabil dan ikut backup.
- [ ] Uji restore database dan folder `data/queue/` di mesin target.
- [ ] Siapkan cara menjalankan server otomatis setelah restart mesin.
- [ ] Pastikan `/api/health` dan `/api/debug/queue` bisa dicek admin/operator.
- [ ] Jalankan simulasi ujian kecil di mesin target, bukan hanya laptop dev.

## Rekomendasi Berikutnya

Urutan kerja yang paling masuk akal:

1. Finalisasi keputusan pilot: mulai dari 50-100 siswa serentak atau tetap mengejar 500 siswa dengan P95 <100ms.
2. Siapkan deployment target dan backup restore di mesin nyata.
3. Jalankan smoke test ujian end-to-end dengan data sekolah.
4. Baru setelah itu tutup checkpoint `.kiro/specs/filesystem-submission-queue/tasks.md` task 14.
