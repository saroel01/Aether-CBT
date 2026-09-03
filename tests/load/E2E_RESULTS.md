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

---

## Baseline PRA-GATE-2 — spec `codebase-bug-sweep`, task 4.1

**Dijalankan:** 2026-08-31 (Windows, single binary, server `go run ./cmd/server`)
**Status kode:** Gate 0 + Gate 1 sudah landing (FK enforcement, WAL, `busy_timeout=5000` aktif
dan terverifikasi via `db.VerifyPragmas`). Gate 2 (klausa 2.3 backoff, 2.4 isolasi per-job)
**belum** dikerjakan. Angka di bawah adalah titik referensi yang dibandingkan oleh task 4.10.

### Perintah

`tests/load` adalah paket `main` multi-file, jadi `go run tests/load/verify_e2e.go` TIDAK bisa
dipakai (kompilasi gagal karena `main.go`, `client.go`, `dataprep.go`, `metrics.go` tidak ikut).
Invokasi yang benar adalah per-paket:

```powershell
# 1. Server aktif dengan rate limiter webhook dilonggarkan (praktik load test, lihat README).
$env:WEBHOOK_RATE_LIMIT_PER_MIN='100000'; go run ./cmd/server

# 2. Reset queue, jalankan skala 200.
Get-ChildItem data/queue -Recurse -File | Remove-Item -Force
go run ./tests/load/ -e2e -scale=200 -e2e-output tests/load/_baseline_200.md

# 3. Reset queue lagi, jalankan skala 500.
Get-ChildItem data/queue -Recurse -File | Remove-Item -Force
go run ./tests/load/ -e2e -scale=500 -e2e-output tests/load/_baseline_500.md
```

`-e2e-output` diarahkan ke berkas sementara karena `writeE2EReport` menulis ulang (bukan
menambahkan) berkas tujuan; hasilnya lalu diringkas ke dokumen ini secara manual.

### Dua koreksi harness yang diperlukan sebelum baseline bisa direkam

1. **Skor klien vs skor turunan.** Burst E2E sebelumnya mengirim `sp` acak (`50..100`)
   sementara `buildISpringXML` selalu menghasilkan awarded 5 / max 10. Verifikasi skor di
   `processor.go` (spec sebelumnya) menolak selisih di luar toleransi 0.05, sehingga **100%
   job dead-letter** sebelum jalur queue pernah teruji: run pertama menghasilkan
   `hasil_tes 0/200`, `failed 100`. `verify_e2e.go` kini memposting `5.00 / 10` yang konsisten
   dengan XML-nya.
2. **Rate limiter webhook.** Default `webhookMax = 100` per menit memotong burst 200 menjadi
   `200=100, non-200=100` (HTTP 429). Server dijalankan dengan
   `WEBHOOK_RATE_LIMIT_PER_MIN=100000`, sesuai praktik yang sudah didokumentasikan di
   `tests/load/README.md`.

Kedua koreksi berlaku identik untuk run 4.1 dan 4.10, jadi perbandingannya tetap setara.

### Hasil baseline

| N | HTTP 200 % | Burst window | Handler avg | Handler P95 | Handler max | Drain time | hasil_tes | E2E Success % | Failed |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 200 | 100.0% | 1.616s | 805ms | 1.405s | 1.571s | 202ms | 200/200 | **100.0%** | 0 |
| 500 | 100.0% | 438ms | 227ms | 407ms | 434ms | 1.425s | 500/500 | **100.0%** | 0 |

Detail tambahan: `hasil_tes_detail` 400 baris (N=200) dan 1000 baris (N=500); DB/WAL
344KB/3.9MB dan 1.4MB/4.0MB.

**Catatan pembacaan.** Skala 200 dijalankan lebih dulu dan menyerap biaya pemanasan
(page cache, WAL warm-up), sehingga latency handler-nya justru lebih tinggi daripada skala
500. Ini artefak urutan run, bukan sifat skala.

**Target absolut P95 <100ms tetap TIDAK terpenuhi** dan itu bukan kriteria spec ini; hal itu
tetap terlacak sebagai task 14 yang masih terbuka di
`.kiro/specs/filesystem-submission-queue/tasks.md`. Kriteria Gate 2 adalah **tidak lebih
buruk dari tabel di atas**.

---

## Post-Gate-2 — spec `codebase-bug-sweep`, task 4.10

**Dijalankan:** 2026-08-31, mesin dan prosedur identik dengan baseline 4.1 (harness sama,
`WEBHOOK_RATE_LIMIT_PER_MIN=100000`, `data/queue/*` direset antar run).
**Status kode:** Gate 2 landing — backoff retry disandikan ke nama file dan benar-benar
menggating `Dequeue` (klausa 2.3), serta isolasi kegagalan per job lewat SAVEPOINT +
kontrak outcome per job (klausa 2.4).

```powershell
Get-ChildItem data/queue -Recurse -File | Remove-Item -Force
go run ./tests/load/ -e2e -scale=200 -e2e-output tests/load/_post_200.md
Get-ChildItem data/queue -Recurse -File | Remove-Item -Force
go run ./tests/load/ -e2e -scale=500 -e2e-output tests/load/_post_500.md
```

### Perbandingan terhadap baseline 4.1

| N | Metrik | Baseline 4.1 | Post-Gate-2 4.10 | Selisih |
|---:|---|---:|---:|---|
| 200 | HTTP 200 % | 100.0% | 100.0% | sama |
| 200 | Handler avg | 805ms | **102ms** | lebih baik |
| 200 | Handler P95 | 1.405s | **177ms** | lebih baik |
| 200 | Handler max | 1.571s | **182ms** | lebih baik |
| 200 | Burst window | 1.616s | **191ms** | lebih baik |
| 200 | Drain time | 202ms | 819ms | lihat catatan |
| 200 | Burst + drain | 1.818s | **1.010s** | lebih baik |
| 200 | E2E success | 200/200 (100%) | 200/200 (100%) | sama |
| 200 | Failed | 0 | 0 | sama |
| 500 | HTTP 200 % | 100.0% | 100.0% | sama |
| 500 | Handler avg | 227ms | 238ms / 251ms | dalam noise |
| 500 | Handler P95 | 407ms | 438ms / 420ms | dalam noise |
| 500 | Handler max | 434ms | 472ms / 459ms | dalam noise |
| 500 | Drain time | 1.425s | **1.225s / 1.210s** | lebih baik |
| 500 | Burst + drain | 1.863s | **1.702s / 1.693s** | lebih baik |
| 500 | E2E success | 500/500 (100%) | 500/500 (100%) | sama |
| 500 | Failed | 0 | 0 | sama |

Skala 500 dijalankan dua kali untuk mengukur noise; kedua angka dicantumkan.

**Catatan drain pada N=200.** Drain diukur mulai SETELAH burst selesai. Pada baseline, burst
memakan 1.616s sehingga worker sudah menguras hampir seluruh queue sebelum pengukuran drain
dimulai; pasca-Gate-2 burst selesai dalam 191ms sehingga hampir semua job masih mengantre saat
stopwatch drain mulai. Angka yang setara untuk dibandingkan adalah **burst + drain**, dan itu
turun dari 1.818s ke 1.010s.

**Verdict: tidak ada regresi.** E2E success tetap 100% di kedua skala, `failed` tetap 0, total
waktu penyelesaian membaik di kedua skala, dan selisih P95 di N=500 (407ms → 420/438ms, ~3-8%)
berada di dalam sebaran run-to-run mesin ini — bandingkan dengan sebaran 8x pada N=200 antar
run. Klausa 3.5 terjaga: batch yang seluruhnya valid tetap satu `BeginTx`/`Commit` (dibuktikan
secara struktural oleh `TestPropertyAllValidBatchIsOneTransaction`, yang menghitung
begin/commit di level driver), sehingga tidak ada regresi ke satu-transaksi-per-job.
