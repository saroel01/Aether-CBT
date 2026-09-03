# Implementation Plan: Codebase Bug Sweep

## Overview

Rencana ini mengeksekusi 25 klausa perbaikan dari `design.md` dalam **lima gerbang berurutan** (Gate 0 → Gate 4). Gerbang tidak boleh dilewati: Gate 0 adalah prasyarat data untuk Gate 1, dan Gate 1 mengubah semantik runtime (`journal_mode`, penegakan FK) yang mendasari Gate 2–4.

Baseline saat ini bersih (`go vet ./...` exit 0, `go test ./...` hijau) — **tidak satu pun dari 25 bug ini tertangkap test yang ada**. Karena itu setiap fix wajib didahului test yang **GAGAL pada F** (kode belum diperbaiki) dan lulus pada F'.

## Konvensi

- Sub-task bertanda `*` adalah test. Test ditulis dan dijalankan **sebelum** fix pasangannya (failing-first).
- Property-based test hanya untuk **C3, C4, C7, C20, C22** (domain input luas, per design.md "Fix Checking"). Sisanya unit test. Library: `pgregory.net/rapid`, anotasi `// Feature: codebase-bug-sweep, Property N`, helper DB dari `internal/testutil` (`NewMigratedDB`, `Seed*`).
- **Anggaran verifikasi**: `go test ./...` (~7 menit, `internal/api/handlers` sendiri ~290s) **hanya di batas gerbang**. Selama iterasi di dalam gerbang jalankan paket terdampak saja (`go test ./internal/submission/...`, `./internal/db/...`, dst.). Platform: Windows/PowerShell.
- Klausa dirujuk sebagai `_Requirements: 2.x, 3.y_` sesuai `bugfix.md`.

## Tasks

- [x] 1. Rekam baseline pra-fix dari kode belum diperbaiki
  - [x]* 1.1 Rekam golden file payload seluruh endpoint list
    - Endpoint: `student_exam.go` `GetAvailableMapels`, `ispring.go` `GetEducationalAnalysis`, `mapping_handler.go` `GetClassSubjects`, `student.go` `GetStudents`, `ruang.go` `GetRooms`, `essay_grading.go` `GetEssayAnswers`, `item_analysis.go` `GetItemAnalysis`.
    - Dataset seed deterministik (tanpa `NULL`, tanpa scan error) via `internal/testutil`; simpan payload JSON apa adanya termasuk **urutan baris** ke `internal/api/handlers/testdata/golden/`.
    - Golden file ini adalah satu-satunya bukti 3.13; F dan F' tidak bisa hidup berdampingan untuk perbandingan langsung.
    - _Requirements: 3.13_

  - [x]* 1.2 Rekam golden file keluaran export CSV, XLSX, dan PDF
    - `csv_utility.go` `ExportResultsCSV` dan `ExportEssayResults`, ketiga cabang (csv/xlsx/pdf), pada dataset yang sama seperti 1.1.
    - Rekam header, urutan kolom, dan urutan baris. Untuk XLSX/PDF yang tidak byte-stable, rekam struktur terekstrak (daftar sheet/baris/sel, atau teks per halaman) bukan byte mentah.
    - _Requirements: 3.13_

  - [x] 1.3 Rekam baseline suite dan vet
    - `go vet ./...` dan `go test ./... > .kiro/specs/codebase-bug-sweep/baseline-test-output.txt` (gitignored) sebagai pembanding drift di setiap checkpoint gerbang.
    - Konfirmasi build tetap menghasilkan single binary via `.\build.ps1` dan catat isi `go.mod` sebagai pembanding 3.3.
    - _Requirements: 3.1, 3.3_

- [x] 2. Gate 0 — Prasyarat FK: normalisasi sentinel `0` → `NULL` (klausa 2.2)
  - [x]* 2.1 Test failing-first normalisasi jalur tulis `peserta` (C2)
    - `internal/api/handlers/student_test.go`: `POST /api/students` dengan body tanpa `kelas_id`/`ruang_id` harus menyimpan `NULL`. Pada F menyimpan `0` → **GAGAL**.
    - Kasus tabel: absen → `NULL`, `0` → `NULL`, `> 0` valid → apa adanya. Ulangi untuk seed, `ImportStudentsCSV`, dan `migrateTenant`.
    - Test terpisah dengan `PRAGMA foreign_keys=ON` eksplisit pada koneksi test: INSERT `peserta` dengan `kelas_id=0` harus ditolak.
    - _Requirements: 2.2, 3.2_

  - [x]* 2.2 Test failing-first `PRAGMA foreign_key_check` bersih pasca-migrasi
    - `internal/db/migrate_test.go`: siapkan DB berisi baris `peserta` sentinel-`0`, jalankan seluruh migrasi, lalu `PRAGMA foreign_key_check` harus mengembalikan nol baris. Pada F melaporkan pelanggaran → **GAGAL**.
    - _Requirements: 2.2_

  - [x] 2.3 Normalisasi `CreateStudent` di `internal/api/handlers/student.go`
    - Ubah field `kelas_id`/`ruang_id` pada struct request menjadi `*int64` (atau `sql.NullInt64`) sehingga "tidak diberikan" dan "diberikan sebagai 0" tidak lagi bertabrakan pada satu representasi.
    - Petakan absen atau `0` → `NULL` sebelum INSERT/UPDATE. Nilai `> 0` diteruskan apa adanya.
    - _Requirements: 2.2, 3.2_

  - [x] 2.4 Normalisasi tiga jalur tulis `peserta` lainnya
    - `cmd/seed/main.go`, `internal/api/handlers/csv_utility.go` `ImportStudentsCSV`, `internal/service/legacy_migration.go` `migrateTenant`: pola normalisasi yang sama seperti 2.3.
    - Audit ketiganya untuk penulis sentinel lain dan untuk INSERT yang bergantung pada `tenant_id` atau FK lain yang kini hidup (daftar blast radius di design.md).
    - _Requirements: 2.2, 3.2_

  - [x] 2.5 Migrasi pembersih data `internal/db/migrations/032_peserta_null_fk_repair.sql`
    - `UPDATE peserta SET kelas_id = NULL WHERE kelas_id = 0` dan padanannya untuk `ruang_id`. Idempoten, aman dijalankan berulang.
    - `UPDATE ... SET NULL` aman terhadap FK karena `NULL` selalu memenuhi constraint referensial, jadi migrasi ini tetap benar ketika Gate 1 sudah menyalakan `foreign_keys` pada koneksi yang sama.
    - _Requirements: 2.2_

  - [x] 2.6 Diagnostik `PRAGMA foreign_key_check` pasca-migrasi di `internal/db/migrate.go`
    - Setelah `RunMigrations` selesai, jalankan `PRAGMA foreign_key_check` dan **log peringatan** berisi daftar tabel/rowid yang melanggar.
    - **Tidak boleh menggagalkan startup**: menggagalkan startup karena pelanggaran data akan membuat sekolah tidak bisa menjalankan ujian. Fail-fast di 2.1 menyasar konfigurasi (deterministik), diagnostik ini menyasar data (bergantung instalasi).
    - Diam sepenuhnya bukan opsi — itu mengulangi kesalahan 1.1.
    - _Requirements: 2.2, 3.1_

  - [x]* 2.7 Preservation: relasi peserta yang valid tidak berubah
    - `kelas_id`/`ruang_id` `> 0` yang ada di tabel induk tetap tersimpan apa adanya, dan respons `CreateStudent`/seed/import CSV identik dengan baseline task 1.
    - _Requirements: 3.2_

  - [x] 2.8 Checkpoint Gate 0
    - **Kriteria keluar**: `PRAGMA foreign_key_check` bersih pada DB hasil migrasi; `go test ./...` hijau.
    - Jalankan `go vet ./...` dan `go test ./...` penuh, bandingkan dengan `baseline-test-output.txt`.
    - Gate 1 tidak boleh dimulai sebelum kriteria ini terpenuhi. Tanya user bila ada pelanggaran FK yang tidak berasal dari sentinel `0`.

- [x] 3. Gate 1 — Aktifkan pragma: DSN + verifikasi pasca-koneksi (klausa 2.1)
  - [x]* 3.1 Test failing-first pragma benar-benar aktif (C1)
    - `internal/db/sqlite_test.go`: buka DB via `db.Connect`, lalu kueri `PRAGMA journal_mode`, `foreign_keys`, `busy_timeout`; assert `wal` (case-insensitive), `1`, `5000`. Pada F menghasilkan `delete`/`0`/`0` → **GAGAL**.
    - Ini konversi probe empiris `modernc.org/sqlite` v1.50.1 menjadi test permanen di repo.
    - Tambahkan kasus DSN untuk path yang mengandung karakter khusus URI (`?`, `#`) dan kasus pesan error verifikasi yang harus menyebut nama pragma, nilai diharapkan, dan nilai aktual.
    - _Requirements: 2.1_

  - [x] 3.2 Ganti bentuk DSN di `internal/db/sqlite.go` `Connect`
    - `file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)`. Prefix `file:` wajib agar `modernc.org/sqlite` memproses query string.
    - Sisipkan `databasePath` dengan escaping URI yang benar — jangan bergantung pada keberuntungan parsing.
    - _Requirements: 2.1_

  - [x] 3.3 Verifikasi pasca-koneksi di dalam `Connect` (bukan di pemanggil)
    - Setelah `DB.Ping()` sukses, kueri ketiga pragma dan bandingkan dengan nilai yang diharapkan; kembalikan error yang menyebut pragma mana, nilai diharapkan, dan nilai aktual.
    - Verifikasi hidup di `Connect` karena lima pemanggil memakainya (`cmd/server`, `cmd/seed`, `cmd/createadmin`, `cmd/migratepasswords`, `internal/testutil`) — menempatkannya di pemanggil berarti menulisnya lima kali dan membuat `internal/testutil` menjadi jalur tak terverifikasi.
    - Catat di komentar bahwa `busy_timeout`/`foreign_keys` bersifat per-koneksi: jaminan berasal dari **DSN** (driver menerapkannya ke setiap koneksi baru); verifikasi berperan sebagai deteksi kesalahan sintaks, bukan jaminan per-koneksi pool `MaxOpenConns=25`.
    - Perbarui komentar doc `Connect` agar tidak lagi mengklaim perilaku yang tidak diverifikasi.
    - **Jangan sentuh** retry loop SQLITE_BUSY di `internal/db/migrate.go` (`execWithBusyRetry`) dan `internal/repository/exam_session_repo.go` — dipertahankan sebagai defense in depth, di luar lingkup spec ini.
    - _Requirements: 2.1_

  - [x]* 3.4 Preservation: migrasi dan suite tetap hijau dengan FK menyala
    - Test preservation paling penting di seluruh spec: 20+ migrasi selesai dan seluruh paket lulus setelah `foreign_keys` aktif.
    - Integration test alur ujian lengkap dengan FK aktif: login siswa → kerjakan → submit via webhook → proses queue → hasil tersimpan → ekspor. Membuktikan penegakan FK tidak mematahkan jalur produksi mana pun.
    - _Requirements: 3.1, 3.3_

  - [x] 3.5 Checkpoint Gate 1
    - **Kriteria keluar**: ketiga pragma terverifikasi aktif; seluruh suite hijau dengan FK menyala.
    - `go vet ./...` + `go test ./...` penuh. Setiap kegagalan baru di sini hampir pasti pelanggaran FK yang sebelumnya tersembunyi — diagnosa sebelum lanjut, jangan matikan pragma.
    - Verifikasi `data/cbt_aether.db` kini memiliki sidecar `-wal` saat server berjalan (bukti WAL benar-benar aktif).

- [x] 4. Gate 2 — Integritas queue: backoff berlaku dan isolasi per-job (klausa 2.3, 2.4)
  - [x]* 4.1 Baseline beban pra-Gate-2
    - `go run ./tests/load/ -e2e -scale=200` lalu `-scale=500`, reset `data/queue/*` antar run. Catat hasil di `tests/load/E2E_RESULTS.md` dan tandai sebagai baseline pra-Gate-2.
    - Target absolut handler P95 <100ms **belum terpenuhi** dan tetap menjadi pekerjaan `.kiro/specs/filesystem-submission-queue/tasks.md` task 14. Kriteria spec ini: **tidak lebih buruk dari baseline yang direkam**.
    - _Requirements: 3.5_

  - [x]* 4.2 Property test jadwal backoff (C3) di `internal/submission/fsqueue_backoff_property_test.go`
    - **Property 1: Bug Condition** - C3 jadwal backoff benar-benar berlaku
    - Anotasi `// Feature: codebase-bug-sweep, Property 1`.
    - Bangkitkan urutan enqueue, kegagalan, dan lompatan waktu acak (domain: waktu jatuh tempo × `RetryCount`). Assert tidak ada job yang pernah di-dequeue sebelum waktu jatuh temponya, dan job baru tetap terproses ketika ada job belum jatuh tempo di `pending/`.
    - Pada F job gagal langsung terpilih ulang (prefix `unix_nano`-nya paling tua) dan menghabiskan `maxRetries` dalam milidetik → **GAGAL**.
    - _Requirements: 2.3, 3.4_

  - [x]* 4.3 Property test isolasi kegagalan batch (C4) di `internal/submission/processor_isolation_property_test.go`
    - **Property 1: Bug Condition** - C4 isolasi kegagalan per job
    - Anotasi `// Feature: codebase-bug-sweep, Property 1`.
    - Bangkitkan batch berukuran acak dengan subset job gagal acak (domain: komposisi batch × posisi kegagalan). Assert himpunan job yang berakhir di `done/` tepat sama dengan himpunan job valid, dan himpunan yang di-`MarkFailed` tepat sama dengan himpunan job gagal.
    - Pada F seluruh transaksi rollback dan kelima job di-`MarkFailed` → **GAGAL**.
    - _Requirements: 2.4, 3.6_

  - [x] 4.4 Sandikan waktu jatuh tempo ke nama file di `internal/submission/fsqueue.go`
    - `MarkFailed`: hitung `min(2^(n-1), 30)` detik seperti sekarang, lalu tulis jadwal itu ke **nama file** tujuan sebagai segmen tambahan yang tidak mengganggu posisi sort `unix_nano`. `EnqueuedAt` tetap diisi agar payload JSON tetap informatif.
    - `Dequeue`: parse jatuh tempo dari nama file dan **lewati** entri yang belum jatuh tempo, lanjut ke kandidat berikutnya (bukan berhenti) — job belum jatuh tempo tidak boleh memblokir queue.
    - Keputusan Opsi A dipilih karena ia satu-satunya yang tidak menyentuh urutan rename-then-read yang menjadi dasar klaim atomic antar worker.
    - `RecoverStartup`: kenali kedua bentuk nama (dengan dan tanpa segmen due-time); nama lama diperlakukan sebagai sudah jatuh tempo, karena `pending/` produksi bisa berisi file versi sebelumnya.
    - Segmen `unix_nano`, `tenant_id`, `no_id` **tetap** menjadi prefix, sehingga korelasi admin antar direktori masih mungkin lewat pencocokan prefix. 3.4 dijaga secara struktural: segmen due-time hanya ditambahkan oleh `MarkFailed`.
    - _Requirements: 2.3, 3.4_

  - [x]* 4.5 Perbarui `internal/submission/fsqueue_property_test.go` ke skema nama file baru
    - Sesuaikan regex `^\d{19}-\d+-[A-Za-z0-9_-]+-[0-9a-f]{8}\.json$` dengan skema baru, dan ganti assertion backoff berbasis `EnqueuedAt` menjadi assertion terhadap segmen due-time di nama file.
    - Kedua assertion lama mengunci **implementasi**, bukan perilaku produk, dan tidak dilindungi klausa 3.x. Perubahan ini **disengaja dan wajib**, bukan regresi.
    - _Requirements: 2.3_

  - [x] 4.6 Kontrak outcome per-job + SAVEPOINT di `internal/submission/processor.go` `ProcessBatch`
    - `ProcessBatch` tidak lagi mengembalikan `error` tunggal, melainkan hasil per job (`map[int64]error` atau slice sejajar dengan `jobs`) **di samping** error tingkat transaksi. Ini inti fix: tanpa kontrak per-job, `processBatchSafe` secara struktural tidak dapat membedakan job gagal dari job yang tidak.
    - Alur: `BeginTx` → per job `SAVEPOINT` → `processOneInTx` → `RELEASE` bila sukses / `ROLLBACK TO SAVEPOINT` bila gagal (catat error, lanjut) → `Commit` di akhir.
    - `Commit` yang gagal adalah kegagalan tingkat transaksi yang berlaku untuk seluruh batch — semua job di-`MarkFailed`; perilaku lama tetap benar untuk kasus ini.
    - 3.5 dipenuhi secara struktural: batch yang seluruhnya valid tetap satu `BeginTx`/`Commit`, satu fsync WAL. Tidak boleh ada regresi ke satu-transaksi-per-job pada jalur normal.
    - _Requirements: 2.4, 3.5, 3.6_

  - [x] 4.7 Konsumsi kontrak per-job di `internal/submission/worker.go` `processBatchSafe`
    - `MarkCompleted` untuk job tanpa error, `MarkFailed` **hanya** untuk job yang punya error.
    - Jalur panic recovery tetap men-`MarkFailed` seluruh batch — panic membuat state transaksi tidak dapat dipercaya.
    - `NewWorkerSingle` dan fixture legacy bersignature `func(ctx, *SubmissionJob) error` harus tetap bekerja: petakan adaptor batch-of-1 ke kontrak baru.
    - _Requirements: 2.4, 3.6_

  - [x] 4.8 Perbarui `docs/runbooks/queue-and-litestream.md` untuk korelasi via prefix
    - Instruksi korelasi file antar direktori berubah dari kesamaan nama byte-per-byte menjadi **pencocokan prefix** (`unix_nano-tenant_id-no_id`). Sertakan contoh perintah PowerShell.
    - Jelaskan arti segmen due-time pada file di `pending/` (job hasil retry yang menunggu jatuh tempo) agar admin tidak menyalahartikannya sebagai job macet.
    - Tanpa update ini, memperbaiki 2.3 merusak prosedur operasional yang terdokumentasi.
    - _Requirements: 2.3_

  - [x]* 4.9 Preservation jalur normal queue
    - **Property 2: Preservation** - Atomisitas batch seluruhnya valid
    - Anotasi `// Feature: codebase-bug-sweep, Property 2`. Bangkitkan batch seluruhnya valid berukuran acak; assert tepat **satu** commit per batch (3.5).
    - Unit: job sukses percobaan pertama tetap pindah ke `done/` tanpa penundaan tambahan (3.4); job gagal permanen tetap pindah ke `failed/` setelah `maxRetries` (3.6).
    - _Requirements: 3.4, 3.5, 3.6_

  - [x]* 4.10 Load test pasca-Gate-2
    - `go run ./tests/load/ -e2e -scale=200` lalu `-scale=500` dengan reset `data/queue/*` antar run.
    - **Kriteria: tidak lebih buruk dari baseline yang direkam di 4.1** (E2E success, drain time, handler avg/P95/max). Bila lebih buruk, kembali ke 4.6 — kemungkinan besar jalur normal tidak lagi satu transaksi.
    - _Requirements: 3.5_

  - [x] 4.11 Checkpoint Gate 2
    - **Kriteria keluar**: backoff berlaku; isolasi per-job terbukti; 3.5 terjaga.
    - `go test ./...` penuh + konfirmasi 4.10 tidak regresi terhadap 4.1.

- [x] 5. Gate 3 — Isolasi tenant (klausa 2.5, 2.6, 2.7)
  - [x]* 5.1 Test failing-first attach lintas tenant (C5)
    - `internal/repository/exam_session_repo_test.go` + handler test: admin tenant A attach kelas/ruang miliknya ke sesi milik tenant B harus not-found **dan tidak ada baris yang tertulis** di `exam_session_kelas`/`exam_session_ruang`. Pada F penulisan berhasil → **GAGAL**.
    - _Requirements: 2.5, 3.7_

  - [x]* 5.2 Test failing-first unlink lintas tenant (C6)
    - `internal/api/handlers/mapping_handler_test.go`: admin tenant A unlink mapping tenant B harus 404 dan mapping tenant B tetap ada. Pada F mapping terhapus → **GAGAL**.
    - Tambahkan kasus `kelas_id`/`mapel_id` non-integer → 400.
    - _Requirements: 2.6, 3.7_

  - [x]* 5.3 Property test kardinalitas dan scoping room-status (C7)
    - **Property 1: Bug Condition** - C7 satu baris per peserta, skor dari sesi yang diminta
    - Anotasi `// Feature: codebase-bug-sweep, Property 1`, file `internal/api/handlers/supervisor_property_test.go`.
    - Bangkitkan peserta dengan jumlah `hasil_tes` acak (0..N) di sesi-sesi acak (domain: jumlah `hasil_tes` × ada/tidaknya `session_id`). Assert jumlah baris keluaran **tepat sama** dengan jumlah peserta, dan nilai `skor`/`status`/`waktu_selesai` selalu berasal dari `session_id` yang diminta.
    - Pada F peserta dengan 3 `hasil_tes` menghasilkan 3 baris dan skor bisa datang dari ujian lain → **GAGAL**.
    - _Requirements: 2.7, 3.8_

  - [x] 5.4 Guard tenancy sesi di `internal/repository/exam_session_repo.go`
    - `AttachClasses` dan `AttachRooms`: verifikasi kepemilikan `sessionID` oleh `tenantID` **sebelum penulisan apa pun**, di samping verifikasi kelas/ruang yang sudah ada.
    - Tempatkan guard berdampingan dengan `validateExam`/`tenantHasRow` dan ikuti bentuknya — jangan perkenalkan pola baru. Kembalikan `ErrNotFound` dari `internal/repository/errors.go`, bukan error ad-hoc.
    - `attachSession` di `internal/api/handlers/exam_session_handler.go` memetakannya ke **404** via `utils.ErrorResponse`. 404 dipilih alih-alih 403 dengan sengaja: memberi tahu pemanggil bahwa sesi milik tenant lain *ada* adalah kebocoran informasi lintas tenant.
    - _Requirements: 2.5, 3.7_

  - [x] 5.5 Filter tenant pada `UnlinkClassSubject` di `internal/api/handlers/mapping_handler.go`
    - Validasi `kelas_id` dan `mapel_id` sebagai integer; tambahkan `tenant_id = ?` pada klausa WHERE `DELETE`, setara dengan pengetatan yang sudah ada di `LinkClassSubject`.
    - Periksa `RowsAffected` dan kembalikan 404 bila `0`. Pakai `ErrNotFound` / pola `requireAffected()` yang sudah ada di `internal/repository/cek_login_repo.go`, dan `utils.ErrorResponse` untuk respons.
    - _Requirements: 2.6, 3.7_

  - [x] 5.6 Kardinalitas dan scoping `fetchRoomStatus` di `internal/api/handlers/supervisor.go`
    - Dua akar harus diperbaiki **bersamaan**; memperbaiki satu saja meninggalkan bug yang lain.
    - (a) Kardinalitas: pastikan join `hasil_tes` menghasilkan paling banyak satu baris per peserta, sesuai kontrak yang dinyatakan komentar fungsinya sendiri.
    - (b) Scoping: ketika `session_id` diberikan, batasi join `hasil_tes` pada sesi/mapel yang sama dengan subquery `cek_login`. Migrasi `031` menyediakan `hasil_tes.exam_session_id`; kolom ini ditambahkan via `ALTER TABLE ADD COLUMN` sehingga baris historis bisa `NULL` — tangani eksplisit, bukan implisit.
    - Semantik LEFT JOIN dipertahankan: peserta tanpa `hasil_tes` tetap satu baris bernilai kosong (3.8).
    - Perbaikan ini otomatis ikut memperbaiki stream SSE di `supervisor_sse.go` karena berbagi fungsi ini.
    - _Requirements: 2.7, 3.8_

  - [x]* 5.7 Preservation operasi tenant sendiri dan room-status kasus normal
    - Attach sesi–kelas/ruang, link, dan unlink pada data milik tenant sendiri tetap sukses dengan respons identik baseline (3.7).
    - Peserta dengan tepat satu `hasil_tes` untuk sesi yang diminta menampilkan nilai yang sama; peserta tanpa hasil tetap satu baris kosong (3.8).
    - Integration: endpoint HTTP `GET /api/supervisor/room-status` dan stream SSE menghasilkan data konsisten satu sama lain.
    - _Requirements: 3.7, 3.8_

  - [x] 5.8 Checkpoint Gate 3
    - **Kriteria keluar**: operasi lintas tenant ditolak; operasi tenant sendiri tidak berubah.
    - `go test ./...` penuh + integration isolasi tenant end-to-end: dua tenant berdata lengkap, jalankan seluruh operasi lintas tenant di 2.5/2.6/2.7 dan verifikasi tidak ada kebocoran baca maupun tulis.
    - Gate 4 baru boleh dimulai setelah Gate 0–3 (klausa P0) landing dan terverifikasi lengkap.

- [x] 6. Gate 4 — P1/P2 cleanup (klausa 2.8–2.25, saling independen)
  - [x]* 6.1 Test failing-first lifecycle queue, stats, dan diagnostik tx (C8, C9, C10, C11, C25)
    - `Stop()` dipanggil dua kali tidak boleh panic. Pada F `panic: close of closed channel` → **GAGAL**.
    - Buat lalu tutup N instance `FilesystemQueue`, bandingkan jumlah goroutine sebelum/sesudah. Pada F setiap instance membocorkan satu goroutine → **GAGAL**. Tambah kasus `Close` idempotent dan `Enqueue` pasca-`Close` → error jelas.
    - Habiskan `maxRetries` sebuah job: `failed/<name>.error.txt` harus ada dan memuat `LastError`. Pada F tidak ada → **GAGAL**.
    - `SQLiteQueue.GetStats` pada `submission_queue` kosong → statistik nol tanpa error. Pada F error → **GAGAL**.
    - `RecoverStartup` terhadap nama file lama maupun baru (regresi Gate 2).
    - _Requirements: 2.8, 2.9, 2.10, 2.11, 2.25, 3.9_

  - [x] 6.2 `Stop` idempotent di `internal/submission/worker.go`
    - Bungkus `close(w.stopChan)` dengan `sync.Once`. Pemanggilan pertama tetap menghentikan worker dan loop dequeue seperti sekarang (3.9).
    - _Requirements: 2.8, 3.9_

  - [x] 6.3 `Close`/`Shutdown` untuk `FilesystemQueue` di `internal/submission/fsqueue.go`
    - Tambahkan metode yang menghentikan `runEnqueueWriter` dan melepaskan goroutine-nya; idempotent via `sync.Once` dengan alasan yang sama seperti 2.8.
    - `Enqueue` setelah `Close` mengembalikan error yang jelas — bukan panic pada channel tertutup, bukan blok selamanya.
    - Perbarui pemanggil: test yang membuat instance queue, dan `cmd/server/main.go` (lewat graceful shutdown di 6.26).
    - _Requirements: 2.9_

  - [x] 6.4 Dead letter konsisten di `MarkFailed` (`internal/submission/fsqueue.go`)
    - Ketika `RetryCount >= maxRetries`, tulis `<name>.error.txt` berisi `LastError` dan konteks retry, sejajar dengan jalur corrupt-file di `Dequeue` yang sudah melakukannya.
    - Konvensi nama sama: `strings.TrimSuffix(name, ".json") + ".error.txt"`, agar `docs/runbooks/queue-and-litestream.md` berlaku untuk kedua jalur.
    - _Requirements: 2.10_

  - [x] 6.5 `COALESCE` pada agregat `GetStats` di `internal/submission/queue.go`
    - Bungkus setiap `SUM(CASE WHEN ... END)` dengan `COALESCE(..., 0)` sehingga tabel kosong menghasilkan statistik nol, bukan `NULL` yang gagal di-scan ke `int`.
    - _Requirements: 2.11_

  - [x] 6.6 Baca melalui `tx` di `internal/submission/processor.go` `processOneInTx`
    - Ganti `p.db.QueryRowContext(...)` di cabang diagnostik menjadi `tx.QueryRowContext(...)`, konsisten dengan pembacaan lain di fungsi yang sama.
    - Wajib setelah Gate 1 mengubah `journal_mode`: pembacaan pada koneksi terpisah saat `tx` terbuka menjadi risiko self-deadlock.
    - _Requirements: 2.25_

  - [x]* 6.7 Test failing-first guard token kosong (C12, C13)
    - `StudentLogin` dengan `token: ""` dan `"   "` harus ditolak, **termasuk** saat ada `exam_session` bertoken `''` yang enterable (kondisi nyata hasil `migrateTenant`). Pada F siswa dapat login tanpa token → **GAGAL**.
    - `ISpringWebhook` dengan `attempt_token: ""` harus 403 `invalid attempt token`. Pada F bergantung pada asumsi data → **GAGAL** bila ada baris `cek_login` bertoken `''`.
    - _Requirements: 2.12, 2.13, 3.10, 3.11_

  - [x] 6.8 Guard token login (klausa 2.12)
    - `internal/api/handlers/exam.go` `StudentLogin`: tolak token kosong/whitespace-only **sebelum** resolusi sesi.
    - `internal/api/handlers/student_session_handler.go` `resolveSessionForToken`: jangan pernah memperlakukan `exam_session` bertoken `''` sebagai enterable.
    - `internal/service/legacy_migration.go` `migrateTenant`: tolak/normalkan `settings.token` kosong agar sumber data cacat tidak terus diproduksi — perbaikan di hilir saja meninggalkan baris `''` di database.
    - _Requirements: 2.12, 3.10_

  - [x] 6.9 Guard token webhook (klausa 2.13)
    - `internal/api/handlers/ispring.go` `ISpringWebhook`: guard eksplisit yang mengembalikan 403 `invalid attempt token` **sebelum** kueri `cek_login` dijalankan.
    - _Requirements: 2.13, 3.11_

  - [x]* 6.10 Test failing-first cookie `Secure` tri-state (C14)
    - `ContentCookieSecure="auto"` + request dengan `X-Forwarded-Proto: https` dari proxy tepercaya: cookie `aether_exam` harus punya atribut `Secure`. Pada F tidak → **GAGAL**.
    - Pasangan wajib: `"auto"` + HTTP polos (LAN sekolah tanpa TLS) → cookie tetap ter-set **tanpa** `Secure`; dan `"true"`/`"false"` eksplisit tetap dihormati (3.12).
    - _Requirements: 2.14, 3.12_

  - [x] 6.11 Tri-state `ContentCookieSecure` (klausa 2.14)
    - `cmd/server/main.go` + `internal/api/handlers` (`SetContentCookieSecure`, `setContentCookie`): representasikan `"true"`/`"false"`/`"auto"` sebagai tri-state, bukan boolean — `cfg.ContentCookieSecure == "true"` membuat `"auto"` runtuh menjadi `false`.
    - Untuk `"auto"`, tentukan `Secure` dari skema request dan `X-Forwarded-Proto`. **Header proxy hanya dipercaya bila proxy-nya tepercaya** — pakai konfigurasi trusted proxy Fiber; mempercayai `X-Forwarded-Proto` tanpa syarat memberi klien kendali atas atribut keamanan cookie.
    - Batas keras 3.12: pada HTTP polos, `"auto"` harus menghasilkan `Secure=false` sehingga siswa tetap bisa ujian.
    - Perbarui komentar `internal/config/config.go` agar cocok dengan perilaku sesungguhnya.
    - _Requirements: 2.14, 3.12_

  - [x]* 6.12 Test failing-first penanganan error handler (C15, C16, C17)
    - Paksa kegagalan scan (mis. tipe kolom tidak cocok) pada endpoint list → harus 500, bukan 200 dengan hasil parsial. Sama untuk `rows.Err()` non-nil. Pada F 200 → **GAGAL**.
    - Baris `hasil_tes` dengan `skor`/`skor_maks` `NULL` → file ekspor tetap memuat baris siswa itu. Pada F baris hilang total → **GAGAL**.
    - Paksa kueri mapel per-kelas gagal → `GetAvailableMapels` harus error, bukan daftar seluruh mapel tenant. Pada F mengembalikan semua mapel → **GAGAL**.
    - _Requirements: 2.15, 2.16, 2.17, 3.13, 3.14_

  - [x] 6.13 Periksa `rows.Scan` dan `rows.Err()` di sembilan lokasi (klausa 2.15)
    - `student_exam.go` `GetAvailableMapels`, `ispring.go` `GetEducationalAnalysis`, `mapping_handler.go` `GetClassSubjects`, `student.go` `GetStudents`, `ruang.go` `GetRooms`, `essay_grading.go` `GetEssayAnswers`, `item_analysis.go` `GetItemAnalysis`, `csv_utility.go` `ExportResultsCSV` dan `ExportEssayResults` (ketiga cabang csv/xlsx/pdf).
    - Endpoint list: 500 via `utils.ErrorResponse`.
    - Ekspor: keputusan berbeda — ekspor yang sebagian sudah ter-stream tidak selalu bisa dibatalkan, jadi kegagalan dicatat di log **dan** ditandai eksplisit di keluaran. File ekspor yang tampak lengkap padahal tidak adalah kegagalan yang lebih buruk daripada error yang jelas.
    - _Requirements: 2.15, 3.13_

  - [x] 6.14 Scan nullable pada ekspor (klausa 2.16)
    - `csv_utility.go` `ExportResultsCSV`: scan `skor`/`skor_maks` ke `sql.NullFloat64`, hapus `continue` yang membuang baris, tulis penanda nilai kosong.
    - Format kolom untuk baris non-`NULL` tidak boleh berubah — verifikasi terhadap golden file task 1.2 (3.13).
    - _Requirements: 2.16, 3.13_

  - [x] 6.15 Pisahkan "kueri gagal" dari "tidak ada mapping" (klausa 2.17)
    - `student_exam.go` `GetAvailableMapels`: kembalikan error pada kegagalan kueri; cabang fallback "semua mapel" hanya boleh tercapai lewat kondisi yang memang dimaksudkan, bukan lewat `rows == nil` akibat error yang menimpa `err`.
    - _Requirements: 2.17, 3.14_

  - [x]* 6.16 Test failing-first delete dan otorisasi (C18, C19)
    - `DeleteClass`/`DeleteRoom` pada baris yang sudah soft-deleted → 404; pada `id` non-integer → 400. Pada F selalu 200 → **GAGAL**.
    - `superadmin` memanggil kesembilan endpoint admin → harus berhasil. Pada F 403 → **GAGAL**. Pasangan wajib: `admin` tetap diizinkan, siswa/pengawas tetap 403 (3.15).
    - Soft delete pada `id` valid baris aktif tetap sukses (3.16).
    - _Requirements: 2.18, 2.19, 3.15, 3.16_

  - [x] 6.17 Delete yang benar (klausa 2.18)
    - `internal/api/handlers/kelas.go` `DeleteClass` dan `internal/api/handlers/ruang.go` `DeleteRoom`: pakai `c.ParamsInt`, sertakan `deleted_at IS NULL`, kembalikan 404 saat `RowsAffected == 0`.
    - Cetak biru sudah ada di `student.go` `DeleteStudent` — samakan strukturnya, pakai `ErrNotFound`/`requireAffected()` yang sudah tersedia.
    - _Requirements: 2.18, 3.16_

  - [x] 6.18 Satu sumber kebenaran otorisasi (klausa 2.19)
    - Hapus pemeriksaan `role != "admin"` di sembilan handler: `UpdateSettings`, `ImportStudentsCSV`, `GradeEssayAnswer`, `LinkClassSubject`, `UnlinkClassSubject`, `DeleteStudent`, `DeleteClass`, `DeleteMapel`, `DeleteRoom`.
    - **Sebelum menghapus**, verifikasi untuk setiap handler bahwa route-nya di `cmd/server/main.go` memang dibungkus `middleware.RequireRoles` dengan daftar role yang diinginkan — menghapus pemeriksaan handler pada route yang tidak terlindungi middleware mengubah bug otorisasi menjadi **lubang** otorisasi.
    - Middleware dipilih sebagai sumber kebenaran karena letaknya bersebelahan dengan definisi route.
    - _Requirements: 2.19, 3.15_

  - [x]* 6.19 Property test kapitalisasi ekstensi (C20)
    - **Property 1: Bug Condition** - C20 ekstensi `.zip` case-insensitive
    - Anotasi `// Feature: codebase-bug-sweep, Property 1`.
    - Bangkitkan nama berkas dengan kapitalisasi `.zip` acak (`.zip`, `.ZIP`, `.Zip`, `.ziP`, plus nama tanpa ekstensi); assert nama tersimpan selalu tanpa ekstensi. Pada F `PAKET.ZIP` tersimpan apa adanya → **GAGAL**.
    - _Requirements: 2.20, 3.17_

  - [x] 6.20 TrimSuffix case-insensitive (klausa 2.20)
    - `internal/api/handlers/soal_package_handler.go` `UploadSoalPackage`: buang ekstensi `.zip` secara case-insensitive via `filepath.Ext` + perbandingan lowercase atau `strings.EqualFold` — bukan dengan meng-lowercase literal suffix-nya.
    - `PAKET.ZIP` → `PAKET`; `paket.zip` → `paket` (3.17).
    - _Requirements: 2.20, 3.17_

  - [x]* 6.21 Test failing-first `CreateRoom` ketat (C21)
    - Input tidak lengkap (`nama_ruang`/`username`/`password` kosong) → ditolak; `username` duplikat → 409; kegagalan `HashPassword` → operasi batal tanpa baris tersimpan. Pada F menyimpan `password_hash` kosong dan menerima input tidak lengkap → **GAGAL**.
    - Pasangan wajib: data lengkap dan valid tetap membuat ruang, dan hash tetap terverifikasi saat login pengawas (3.18).
    - _Requirements: 2.21, 3.18_

  - [x] 6.22 `CreateRoom` yang ketat (klausa 2.21)
    - `internal/api/handlers/ruang.go` `CreateRoom`: tangani error `utils.HashPassword` dan batalkan operasi bila hashing gagal (hapus `hash, _ :=`).
    - Wajibkan `nama_ruang`, `username`, `password`; periksa keunikan `username` sebelum INSERT dan kembalikan `ErrConflict` (`internal/repository/errors.go`) → 409.
    - _Requirements: 2.21, 3.18_

  - [x]* 6.23 Property test atomisitas increment (C22)
    - **Property 1: Bug Condition** - C22 increment infraction atomic
    - Anotasi `// Feature: codebase-bug-sweep, Property 1`, file `internal/repository/cek_login_infraction_property_test.go`.
    - Bangkitkan jumlah pemanggil konkuren acak; assert himpunan nilai kembalian seluruhnya **distinct** dan ambang lock terpicu **tepat sekali**. Jalankan dengan `-race` dan iterasi cukup banyak (pada F kegagalan bisa flaky).
    - Pada F kedua pemanggil membaca nilai pasca-update yang sama → nilai duplikat → **GAGAL**.
    - _Requirements: 2.22, 3.19_

  - [x] 6.24 Increment atomic (klausa 2.22)
    - `internal/repository/cek_login_repo.go` `IncrementInfraction`: gabungkan increment dan pembacaan menjadi satu operasi dengan `UPDATE ... RETURNING` (didukung SQLite modern); alternatif: bungkus keduanya dalam satu transaksi. `RETURNING` lebih disukai karena menghilangkan race tanpa menambah transaksi.
    - Satu pelanggaran tunggal tetap mengembalikan counter yang benar dan tetap mengunci sesi saat ambang tercapai (3.19).
    - _Requirements: 2.22, 3.19_

  - [x]* 6.25 Test failing-first graceful shutdown (C23)
    - Kirim `SIGINT` ke server saat ada job in-flight: worker berhenti, `processing/` kosong (job selesai atau kembali ke `pending/`), DB tertutup. Pada F semua `defer` dilewati oleh `os.Exit` → **GAGAL**.
    - Integration restart-dan-pemulihan: restart setelah sinyal, verifikasi tidak ada hasil ujian yang hilang dan tidak ada penantian sweep stuck-threshold 5 menit (menguji 2.23 + 2.9 + 2.8 bersama-sama).
    - Windows: `SIGTERM` tidak dikirim seperti di Unix, jadi `SIGINT` (Ctrl+C) adalah jalur yang benar-benar teruji di platform target.
    - _Requirements: 2.23, 3.3_

  - [x] 6.26 Graceful shutdown di `cmd/server/main.go` (klausa 2.23)
    - Ganti `log.Fatal(app.Listen(...))` dengan pola: `Listen` di goroutine, tunggu `SIGINT`/`SIGTERM` via `signal.NotifyContext`, lalu `app.Shutdown()`.
    - Biarkan urutan pembersihan berjalan: hentikan worker (6.2), tutup queue (6.3), batalkan context, tutup DB.
    - Bergantung pada 6.2 dan 6.3 — jangan kerjakan sebelum keduanya landing.
    - _Requirements: 2.23, 2.8, 2.9_

  - [x]* 6.27 Test failing-first instan batas scheduling (C24)
    - Sesi X `[10:00, 11:00]` dan sesi Y `[11:00, 12:00]` bertoken sama, evaluasi pada `11:00`: tidak boleh sekaligus dinyatakan tidak bertumpang tindih **dan** keduanya enterable. Pada F kontradiksi ini terjadi → **GAGAL**.
    - Tabel kasus eksplisit untuk `enterable` dan `windowsOverlap` di bawah satu konvensi: instan `mulai`, instan `selesai`, di dalam, di luar.
    - Waktu non-batas tidak boleh berubah keputusannya (3.20).
    - _Requirements: 2.24, 3.20_

  - [x] 6.28 Konvensi interval seragam (klausa 2.24)
    - `internal/service/scheduling_service.go`: samakan konvensi `enterable` dan `windowsOverlap` ke setengah terbuka `[mulai, selesai)` — konvensi yang sudah dipakai `windowsOverlap` dan standar untuk interval waktu.
    - Konsekuensinya `enterable` menjadi **eksklusif di ujung akhir**. Perubahan perilaku pada instan `selesai` ini harus ditulis eksplisit di test (6.27) agar keputusannya terdokumentasi, bukan tersembunyi.
    - _Requirements: 2.24, 3.20_

  - [x]* 6.29 Preservation menyeluruh terhadap baseline task 1
    - **Property 2: Preservation** - Perilaku non-bug tidak berubah
    - Anotasi `// Feature: codebase-bug-sweep, Property 2`. Bangkitkan input non-bug acak (id valid, token tidak kosong, tenant cocok, tanpa `NULL`, batch seluruhnya valid) dan assert hasil F' cocok dengan baseline yang direkam.
    - Replay golden file 1.1 dan 1.2: payload, urutan, dan format identik pada seluruh endpoint list dan ekspor termasuk header dan kolom CSV/XLSX/PDF (3.13).
    - Ekspor lintas format pada data yang mengandung `NULL` maupun tidak, setelah 6.13 dan 6.14.
    - Sisa klausa preservation yang belum tercakup gerbang sebelumnya: 3.10, 3.11, 3.12, 3.14, 3.15, 3.16, 3.17, 3.18, 3.19, 3.20.
    - _Requirements: 3.10, 3.11, 3.12, 3.13, 3.14, 3.15, 3.16, 3.17, 3.18, 3.19, 3.20_

  - [x] 6.30 Checkpoint Gate 4 — final
    - **Kriteria keluar**: masing-masing dari 18 klausa punya test yang gagal pada F dan lulus pada F'. Konfirmasi setiap sub-task `*` di Gate 4 pernah tercatat GAGAL sebelum fix pasangannya.
    - `go vet ./...` exit 0 dan `go test ./...` hijau; bandingkan dengan `baseline-test-output.txt` (3.1).
    - `.\build.ps1` tetap menghasilkan single binary `aether-cbt.exe` pada port yang sama, `go.mod` tanpa dependensi service eksternal baru (3.3).
    - Ensure all tests pass, ask the user if questions arise.

## Notes

- Sub-task bertanda `*` adalah test dan **wajib dijalankan pada F terlebih dahulu**. Test yang langsung ditulis di atas kode yang sudah diperbaiki tidak membuktikan apa pun — ia bisa lulus karena menguji hal yang salah.
- Property-based test hanya untuk C3 (4.2), C4 (4.3), C7 (5.3), C20 (6.19), C22 (6.23). Semua klausa lain memakai unit test karena domain inputnya sempit dan diskrit.
- Perubahan pada `internal/submission/fsqueue_property_test.go` (4.5) dan `docs/runbooks/queue-and-litestream.md` (4.8) adalah **bagian eksplisit dari lingkup**, bukan regresi.
- `syncDir` di `internal/submission/fsync.go` sudah memperhitungkan bahwa fsync direktori adalah no-op di Windows — test durabilitas tidak boleh mengasumsikan sebaliknya.
- Target absolut handler P95 <100ms tetap di luar lingkup spec ini; kriteria beban di sini adalah "tidak lebih buruk dari baseline yang direkam" (4.1 vs 4.10).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "1.3"] },
    { "id": 1, "tasks": ["2.1", "2.2"] },
    { "id": 2, "tasks": ["2.3", "2.4", "2.5", "2.6"] },
    { "id": 3, "tasks": ["2.7"] },
    { "id": 4, "tasks": ["2.8"] },
    { "id": 5, "tasks": ["3.1"] },
    { "id": 6, "tasks": ["3.2", "3.3"] },
    { "id": 7, "tasks": ["3.4"] },
    { "id": 8, "tasks": ["3.5"] },
    { "id": 9, "tasks": ["4.1", "4.2", "4.3"] },
    { "id": 10, "tasks": ["4.4", "4.6"] },
    { "id": 11, "tasks": ["4.5", "4.7", "4.8"] },
    { "id": 12, "tasks": ["4.9"] },
    { "id": 13, "tasks": ["4.10"] },
    { "id": 14, "tasks": ["4.11"] },
    { "id": 15, "tasks": ["5.1", "5.2", "5.3"] },
    { "id": 16, "tasks": ["5.4", "5.5", "5.6"] },
    { "id": 17, "tasks": ["5.7"] },
    { "id": 18, "tasks": ["5.8"] },
    { "id": 19, "tasks": ["6.1", "6.7", "6.10", "6.12", "6.16", "6.19", "6.21", "6.23", "6.25", "6.27"] },
    { "id": 20, "tasks": ["6.2", "6.3", "6.4", "6.5", "6.6", "6.8", "6.9", "6.11", "6.13", "6.14", "6.15", "6.17", "6.18", "6.20", "6.22", "6.24", "6.28"] },
    { "id": 21, "tasks": ["6.26"] },
    { "id": 22, "tasks": ["6.29"] },
    { "id": 23, "tasks": ["6.30"] }
  ]
}
```
