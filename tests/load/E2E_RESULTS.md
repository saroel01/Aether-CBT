# Aether CBT - End-to-End Queue Verification Report

**Generated:** 2026-05-26 19:14:35
**Methodology:** One-shot burst N concurrent webhook POSTs -> wait for queue drain -> count `hasil_tes` rows and failed queue files for that scale's prefix.

This measures true end-to-end success: the job is accepted by HTTP handler, drained by worker, and persisted into `hasil_tes`.

## Results Matrix

| N | HTTP 200 % | HTTP P95 | Drain Time | hasil_tes Saved | E2E Success % | Failed | Notes |
|---:|---:|---:|---:|---:|---:|---:|---|
| 50 | 100.0% | 45ms | 202ms | 50/50 | **100.0%** | 0 | all good |
| 100 | 100.0% | 90ms | 406ms | 100/100 | **100.0%** | 0 | all good |
| 200 | 100.0% | 380ms | 408ms | 200/200 | **100.0%** | 0 | all good |
| 500 | 100.0% | 314ms | 1.212s | 500/500 | **100.0%** | 0 | all good |

## Per-Scale Detail

### Scale: 50 students

- **HTTP burst window**: 49ms
- **HTTP latency**: avg 27ms / p95 45ms / max 47ms
- **HTTP 200 acceptance**: 50 / 50 (100.0%)
- **Worker drain duration**: 202ms
- **hasil_tes rows persisted**: 50 / 50 (**100.0% end-to-end success**)
- **hasil_tes_detail rows**: 100
- **failed queue files**: 0
- **DB / WAL after run**: 2.3MB / -

### Scale: 100 students

- **HTTP burst window**: 96ms
- **HTTP latency**: avg 37ms / p95 90ms / max 92ms
- **HTTP 200 acceptance**: 100 / 100 (100.0%)
- **Worker drain duration**: 406ms
- **hasil_tes rows persisted**: 100 / 100 (**100.0% end-to-end success**)
- **hasil_tes_detail rows**: 200
- **failed queue files**: 0
- **DB / WAL after run**: 2.3MB / -

### Scale: 200 students

- **HTTP burst window**: 411ms
- **HTTP latency**: avg 188ms / p95 380ms / max 409ms
- **HTTP 200 acceptance**: 200 / 200 (100.0%)
- **Worker drain duration**: 408ms
- **hasil_tes rows persisted**: 200 / 200 (**100.0% end-to-end success**)
- **hasil_tes_detail rows**: 400
- **failed queue files**: 0
- **DB / WAL after run**: 2.3MB / -

### Scale: 500 students

- **HTTP burst window**: 350ms
- **HTTP latency**: avg 173ms / p95 314ms / max 343ms
- **HTTP 200 acceptance**: 500 / 500 (100.0%)
- **Worker drain duration**: 1.212s
- **hasil_tes rows persisted**: 500 / 500 (**100.0% end-to-end success**)
- **hasil_tes_detail rows**: 1000
- **failed queue files**: 0
- **DB / WAL after run**: 2.5MB / -

## Interpretasi Cepat

- **HTTP 200 %** = berapa persen submission diterima oleh handler.
- **E2E Success %** = berapa persen submission yang benar-benar tersimpan ke `hasil_tes` setelah worker selesai.
- Selisih HTTP 200 dan E2E = job yang gagal di worker atau masuk ke `data/queue/failed/`.
- **Drain time** = berapa lama worker menyelesaikan semua job. Ini menentukan kapan admin bisa melihat hasil lengkap setelah ujian.
- **Failed** > 0 = submission perlu intervensi manual.

## Production Readiness Note

- Jalur data hasil ujian sudah lulus functional E2E sampai burst 500: 500/500 tersimpan, 0 failed, drain 1.212s.
- Budget HTTP P95 <100ms baru terpenuhi pada skala 50 dan 100. Pada burst 200/500, P95 masih 380ms/314ms.
- Untuk pilot kecil sampai sekitar 100 siswa serentak, hasil ini sudah masuk akal.
- Untuk target 200-500 siswa serentak dengan SLA P95 <100ms, masih perlu keputusan: longgarkan SLA penerimaan webhook, lanjut optimasi, atau pindahkan queue/DB ke arsitektur yang lebih kuat.
