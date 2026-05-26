# Implementation Plan: Filesystem Submission Queue

## Overview

Implementasi mengganti tabel `submission_queue` SQLite dengan filesystem queue (`pending/`, `processing/`, `done/`, `failed/`, `tmp/`) yang dioperasikan via `os.Rename` atomic, sekaligus memulihkan validasi anti-cheat sinkron di handler, membungkus penulisan hasil dalam transaksi atomic, menambah panic recovery di worker, dan menambah recovery startup + migrasi data legacy. Bahasa: Go (sesuai design.md). Library PBT: `pgregory.net/rapid`.

Pekerjaan dilakukan di branch baru `feature/filesystem-queue` dengan commit kecil per task major. Test ditulis bersamaan/segera setelah implementasi (TDD-light). Validasi akhir: `tests/load/verify_e2e.go` di skala 50, 100, 200, 500.

Konvensi:
- Sub-tasks dengan `*` adalah test (opsional untuk MVP, **wajib** untuk merge ke main).
- Setiap property test mengacu pada property number di design.md dan requirement clause yang divalidasi.
- Setiap commit mereferensikan task ID.

## Tasks

- [x] 1. Persiapan branch dan baseline drift
  - [x] 1.1 Buat branch `feature/filesystem-queue` dan tangkap baseline test
    - `git checkout -b feature/filesystem-queue` dari `main` (atau branch dev aktif).
    - Jalankan `go test ./internal/...` dan simpan output mentah ke `.kiro/specs/filesystem-submission-queue/baseline-test-output.txt` (gitignored) untuk perbandingan.
    - Konfirmasi 4 test webhook lama (`TestISpringWebhookSuccess`, `TestISpringWebhookForbidden`, `TestISpringWebhookRejectsMissingAttemptToken`, `TestISpringGracePeriod`) status saat ini (kemungkinan panic / fail).
    - Tambahkan `pgregory.net/rapid` ke `go.mod` via `go get pgregory.net/rapid@latest` jika belum ada.
    - _Requirements: 15.1_

- [x] 2. SubmissionJob struct dan serialization
  - [x] 2.1 Revisi `SubmissionJob` struct di `internal/submission/job.go`
    - Tambah field `EnqueuedAt time.Time` dengan tag `json:"enqueued_at"`.
    - Atur urutan tag JSON sesuai Requirement 10.1 (`validasi`, `tenant_id`, `no_id`, `score`, `max_score`, `attempt_token`, `enqueued_at`, `retry_count`, `last_error`, `detail_xml`).
    - Tandai `ID`, `Status`, `CreatedAt`, `UpdatedAt`, `NextRetryAt` dengan `json:"-"` agar tidak diserialisasi ke `Job_File`.
    - Tambah field internal `fileName string` (unexported) untuk handle in-memory.
    - _Requirements: 10.1, 10.2, 11.4_

  - [x] 2.2 Implementasi `MarshalJob` dan `UnmarshalJob` di `internal/submission/job.go`
    - `MarshalJob` memakai `json.Encoder` dengan `SetEscapeHTML(false)` dan `SetIndent("", "  ")`.
    - `UnmarshalJob` memvalidasi field wajib (`tenant_id != 0`, `no_id != ""`, `validasi != ""`, `enqueued_at != zero`); error deskriptif menyebut field yang bermasalah.
    - _Requirements: 10.5, 11.1, 11.2, 11.3_

  - [x]* 2.3 Property test round-trip serialization di `internal/submission/job_property_test.go`
    - **Property 2: Round-trip Serialization**
    - Generator `genSubmissionJob` dengan `DetailXML` 50% kosong, 50% XML berisi karakter khusus (`<`, `>`, `&`, `\n`, `"`, unicode).
    - Assert `UnmarshalJob(MarshalJob(j))` ekuivalen field-by-field dengan `j` (gunakan `reflect.DeepEqual` setelah normalisasi `EnqueuedAt` ke UTC).
    - Annotation: `// Feature: filesystem-submission-queue, Property 2`
    - _Validates: Requirements 1.5, 10.5, 11.4_

  - [x]* 2.4 Property test format output di `internal/submission/job_property_test.go`
    - **Property 3: Format Output `MarshalJob`**
    - Assert (a) `json.Unmarshal` standar berhasil tanpa error, (b) tiap baris non-pertama dimulai dengan minimal 2 spasi (indent), (c) urutan key tetap (parse via `encoding/json`'s `Decoder` dengan `Token()` API), (d) `enqueued_at` match regex ISO 8601 UTC `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z`.
    - _Validates: Requirements 10.1, 10.2, 11.5_

- [x] 3. FilesystemQueue core
  - [x] 3.1 Buat skeleton `internal/submission/fsqueue.go` dengan `NewFilesystemQueue`
    - Definisi struct `FilesystemQueue` dengan field `root`, `pendingDir`, `processingDir`, `doneDir`, `failedDir`, `tmpDir`, `maxRetries`, `stuckThreshold`, `doneRetention`, `inFlight map[int64]string`, `nextID int64`, `mu sync.Mutex`.
    - `NewFilesystemQueue(root string) (*FilesystemQueue, error)` memanggil `os.MkdirAll` untuk lima sub-direktori dengan mode `0755`.
    - Hardcode default: `maxRetries=5`, `stuckThreshold=5*time.Minute`, `doneRetention=7*24*time.Hour`. Konstruktor opsional `WithConfig` ditunda; default cukup untuk task awal.
    - _Requirements: 1.6, 9.3, 9.4_

  - [x] 3.2 Implementasi `Enqueue` dengan write-tmp-then-rename di `internal/submission/fsqueue.go`
    - Generate filename `<unix_nano>-<tenant_id>-<sanitize(no_id)>-<8hex>.json`. Sanitize regex: pertahankan `[A-Za-z0-9_-]`, ganti lainnya dengan `_`.
    - Set `job.EnqueuedAt = time.Now().UTC()`.
    - `os.OpenFile` dengan `O_WRONLY|O_CREATE|O_EXCL` di `tmp/`. Tulis hasil `MarshalJob`. Close.
    - `os.Rename(tmpPath, pendingPath)`. Jika rename gagal, `os.Remove(tmpPath)` lalu return error.
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

  - [x] 3.3 Implementasi `Dequeue` dan `DequeueBatch` di `internal/submission/fsqueue.go`
    - `Dequeue`: `os.ReadDir(pendingDir)` (Go 1.21+ sudah sorted by name = FIFO via unix_nano prefix). Untuk tiap `.json`, coba `os.Rename(pending, processing)`. Jika `errors.Is(err, fs.ErrNotExist)` → continue. Jika sukses, `os.ReadFile`, `UnmarshalJob`. Jika unmarshal gagal, pindahkan file ke `failed/` dengan companion `<name>.error.txt` dan `continue`.
    - Assign `job.ID` via `q.nextID++` (under `q.mu`). Simpan `q.inFlight[job.ID] = entry.Name()`.
    - Return `(nil, nil)` jika pending kosong, dalam < 50ms (no sleep di dalam).
    - `DequeueBatch(ctx, maxBatch)`: panggil `Dequeue` dalam loop hingga `maxBatch` job atau pending kosong.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 17.3_

  - [x] 3.4 Implementasi `MarkCompleted` dan `MarkFailed` di `internal/submission/fsqueue.go`
    - `MarkCompleted(ctx, jobID)`: lookup `q.inFlight[jobID]`, `os.Rename(processing/<name>, done/<name>)`. Hapus dari `inFlight`.
    - `MarkFailed(ctx, jobID, processErr)`: read file dari `processing/`, `UnmarshalJob`, increment `RetryCount`, set `LastError = processErr.Error()`, set `EnqueuedAt = time.Now().UTC()`. Tulis ulang via tmp. Rename ke `pending/` jika `RetryCount < maxRetries`, else ke `failed/`. Hapus file lama di `processing/`. Hapus dari `inFlight`.
    - Backoff exponential: `EnqueuedAt = time.Now().UTC().Add(min(2^(retryCount-1), 30) * time.Second)`. Filename tetap sama agar admin korelasi.
    - _Requirements: 2.5, 3.1, 3.2, 3.3, 3.4, 3.5_

  - [x] 3.5 Implementasi `GetStats` dan tambah field `DoneCount` ke `QueueStats` di `internal/submission/queue.go` + `internal/submission/fsqueue.go`
    - Di `internal/submission/queue.go`, tambah field `DoneCount int json:"done_count"` ke struct `QueueStats`.
    - Update implementasi `GetStats` di `InMemoryQueue`, `SQLiteQueue`, `BufferedSQLiteQueue` untuk return 0 di `DoneCount` (atau nilai akurat jika tersedia).
    - Di `fsqueue.go`, `FilesystemQueue.GetStats(ctx)` count `*.json` di empat direktori dengan `os.ReadDir`. Return error jika ada direktori yang tidak terbaca.
    - _Requirements: 12.1, 12.2_

- [x] 4. FilesystemQueue tests
  - [x]* 4.1 Property test atomicity Enqueue di `internal/submission/fsqueue_property_test.go`
    - **Property 1: Enqueue Atomicity dan Penamaan Unik**
    - Generator: N (1..50) job valid. Buat queue di `t.TempDir()`. Enqueue semua. Assert `tmp/` kosong, `len(pending/) == N`, semua nama unik dan match regex `^\d{19}-\d+-[A-Za-z0-9_-]+-[0-9a-f]{8}\.json$`.
    - _Validates: Requirements 1.1, 1.2_

  - [x]* 4.2 Property test FIFO + state transition di `internal/submission/fsqueue_property_test.go`
    - **Property 4: Dequeue FIFO dan State Transition**
    - Generator: enqueue N job dengan `time.Sleep(time.Microsecond)` antar enqueue agar `unix_nano` distinct. Dequeue N kali. Assert urutan `EnqueuedAt` menaik (atau urutan yang konsisten dengan filename sort), file dipindah dari `pending/` ke `processing/`.
    - _Validates: Requirements 2.1, 2.2_

  - [x]* 4.3 Property test MarkCompleted di `internal/submission/fsqueue_property_test.go`
    - **Property 5: MarkCompleted Memindahkan ke Done**
    - Enqueue, Dequeue, capture file content. `MarkCompleted`. Assert file tidak ada di `processing/`, ada di `done/`, isinya identik byte-for-byte (atau setelah re-marshal field-equal).
    - _Validates: Requirements 2.5_

  - [x]* 4.4 Property test MarkFailed retry/dead letter di `internal/submission/fsqueue_property_test.go`
    - **Property 6: MarkFailed Mengikuti Aturan Retry dan Dead Letter**
    - Generator: `retry_count` awal 0..6, error message arbitrary. Untuk tiap kasus, dequeue lalu `MarkFailed`. Assert: jika `retry_count+1 < maxRetries` → file di `pending/` dengan field `retry_count` dan `last_error` benar; else file di `failed/`. Verifikasi `EnqueuedAt` mencerminkan backoff `min(2^n, 30)` detik.
    - _Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5_

  - [x]* 4.5 Property test debug counts di `internal/submission/fsqueue_property_test.go`
    - **Property 15: Debug Counts Match Filesystem**
    - Generator: tuple `(n_p, n_pr, n_d, n_f)` dengan tiap nilai 0..20. Bangun state queue dengan menulis file dummy `.json` di tiap sub-direktori. Panggil `GetStats`. Assert tiap count match.
    - _Validates: Requirements 12.1, 12.2_

  - [x]* 4.6 Edge case unit test di `internal/submission/fsqueue_test.go`
    - File JSON corrupt di `pending/`: assert Dequeue memindahkan ke `failed/` dengan companion `.error.txt`, tidak panic, tidak return error fatal.
    - Race rename: simulasikan dengan dua call `Dequeue` paralel; assert satu sukses, satu mendapat `(nil, nil)` atau lewat ke entry berikutnya tanpa error.
    - ENOSPC simulation: gunakan `os.Setenv` mock atau (jika sulit) inject hook untuk `os.OpenFile` gagal; assert Enqueue return error dan tidak meninggalkan file di `tmp/`.
    - _Requirements: 1.3, 1.4, 2.4, Edge Case 1, Edge Case 2 di design.md_

- [x] 5. Checkpoint - FilesystemQueue siap
  - Jalankan `go test ./internal/submission/...` (termasuk PBT). Pastikan semua hijau.
  - Commit: `feat(queue): filesystem queue core + serialization`.
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Worker batching dan panic recovery
  - [x] 6.1 Refactor `internal/submission/worker.go` untuk batching dan panic recovery
    - Ubah signature `processFunc` jadi `func(ctx context.Context, jobs []*SubmissionJob) error` (batch).
    - Tambah field `batchSize int` (default 5), `batchTimeout time.Duration` (default 100ms).
    - Implementasi `collectBatch(ctx)` per pseudocode design (loop Dequeue sampai batchSize atau timeout).
    - Implementasi `processBatchSafe(ctx, batch)` dengan `defer recover()`. Pada panic, log stack via `runtime/debug.Stack()`, lalu `MarkFailed` semua job di batch dengan error `fmt.Errorf("worker panic: %v", r)`.
    - Tambah konstruktor backward-compat `NewWorkerSingle(q, single func(ctx, *SubmissionJob) error)` yang membungkus jadi batch-of-1 jika test fixture lama masih memakainya. Boleh dihapus jika tidak ada caller.
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 17.3, 17.4_

  - [x]* 6.2 Property test panic resilience worker di `internal/submission/worker_property_test.go`
    - **Property 11: Worker Tetap Hidup di Hadapan Panic**
    - Generator: N job (5..30) dengan flag `shouldPanic` random. `processFunc` mock yang panic jika ada flag. Run worker satu pass per batch sampai queue habis.
    - Assert: worker tidak berhenti (loop selesai natural), job sukses ada di `done/`, job panic dengan `retry_count < max` di `pending/`, job panic dengan `retry_count >= max` di `failed/` dengan `last_error` mengandung `"worker panic"`.
    - _Validates: Requirements 6.2, 6.3, 6.4_

- [x] 7. Processor batch transaction atomicity
  - [x] 7.1 Refactor `internal/submission/processor.go` jadi `ProcessBatch`
    - Hapus validasi anti-cheat dari processor (sudah pindah ke handler). Tetap lookup `peserta_id`+`mapel_id` di awal `processOneInTx` jika diperlukan untuk INSERT.
    - `ProcessBatch(ctx, jobs []*SubmissionJob) error`: `db.BeginTx`, loop `processOneInTx`, commit; defer rollback.
    - `processOneInTx(ctx, tx, job)`: UPSERT `hasil_tes` dengan `ON CONFLICT(tenant_id, validasi) DO UPDATE`. DELETE lalu INSERT N `hasil_tes_detail` (replace strategy untuk Req 14.3). DELETE `cek_login` untuk peserta tersebut.
    - Pastikan `Validasi` di-set ke `<tenant_id>_<no_id>_<mapel_id>` (handler sudah men-set, processor tinggal pakai).
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 14.1, 14.3, 17.3, 17.4_

  - [x]* 7.2 Property test detail count di `internal/submission/processor_property_test.go`
    - **Property 8: Jumlah Baris Detail Sama dengan Jumlah Pertanyaan**
    - Setup: SQLite tmpfile dengan migrasi `cek_login`, `hasil_tes`, `hasil_tes_detail`, `peserta`, `mapel`. Seed peserta + mapel + cek_login.
    - Generator: `detail_xml` parseable dengan N pertanyaan (0..50). `ProcessBatch([job])`.
    - Assert: `SELECT COUNT(*) FROM hasil_tes_detail WHERE hasil_tes_id = ?` == N.
    - _Validates: Requirements 5.3_

  - [x]* 7.3 Property test idempotency di `internal/submission/processor_property_test.go`
    - **Property 9: Idempotensi terhadap Reprocessing**
    - Generator: dua job dengan `validasi` sama tetapi `score`/`detail_xml` berbeda. Process keduanya berurutan.
    - Assert: `COUNT(*) FROM hasil_tes WHERE tenant_id=? AND validasi=?` == 1; `COUNT(*) FROM hasil_tes_detail` == jumlah pertanyaan job kedua; tidak ada baris detail orphan dari job pertama.
    - _Validates: Requirements 5.4, 14.1, 14.3, 14.4_

  - [x]* 7.4 Property test atomicity batch di `internal/submission/processor_property_test.go`
    - **Property 10: Atomicity Batch**
    - Generator: batch N (1..10) job. Random satu job memiliki `peserta`/`mapel` yang tidak ada (memicu error UPSERT atau lookup).
    - Skenario A (semua valid): assert `COUNT(*) FROM hasil_tes` bertambah N, satu commit.
    - Skenario B (satu invalid): assert tidak ada perubahan persistent, error dipropagasi.
    - _Validates: Requirements 5.1, 5.2, 17.3, 17.4_

- [x] 8. Handler validasi sinkron anti-cheat
  - [x] 8.1 Revisi `internal/api/handlers/ispring.go` untuk validasi sinkron
    - Ambil `sid`/`USER_NAME`, `sp`, `tp`, `dr`, `attempt_token`/`AETHER_ATTEMPT_TOKEN` dari form.
    - Single SELECT `peserta JOIN cek_login` per pseudocode design (Req 4.7). Return 403 `"active session not found"` untuk `sql.ErrNoRows`.
    - `subtle.ConstantTimeCompare(token, expectedToken)`. Return 403 `"invalid attempt token"` jika tidak cocok atau `expectedToken == ""`.
    - Parse `detail_xml` jika non-empty via `ispringparser.ParseDetailedResults`. Return 400 `"Invalid iSpring detailed results XML"` jika gagal.
    - Set `job.Validasi = fmt.Sprintf("%d_%s_%d", tenantID, noID, mapelID)`.
    - Enqueue. Return 200 `"Result received successfully"`.
    - Hapus enqueue path tanpa validasi yang sebelumnya ada (jika ada).
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7_

  - [x] 8.2 Refactor test fixture di `internal/api/handlers/ispring_test.go` untuk FilesystemQueue
    - Helper `setupISpringTestApp(t)` yang membuat `t.TempDir()`, instansiasi `FilesystemQueue` di sana, panggil `handlers.SetSubmissionQueue(fsQueue)`, return `(app, db, fsQueue, cleanup)`.
    - Pastikan setiap test case independen (fresh dir, fresh db).
    - _Requirements: 15.1_

  - [x]* 8.3 Test webhook example-based di `internal/api/handlers/ispring_test.go`
    - `TestISpringWebhookSuccess`: cek_login valid, token cocok. POST. Assert HTTP 200, body `"Result received successfully"`, `len(pending/) == 1`. Run worker satu pass via `Worker.processBatchSafe` atau `Processor.ProcessBatch` langsung. Assert baris `hasil_tes` muncul dengan `validasi == "<tenant>_<noID>_<mapelID>"`.
    - `TestISpringWebhookForbidden`: tanpa cek_login. POST. Assert HTTP 403 body `"active session not found"`, `pending/` kosong.
    - `TestISpringWebhookRejectsMissingAttemptToken`: cek_login ada, token kosong DAN token salah (dua sub-case). Assert HTTP 403 body `"invalid attempt token"`, `pending/` kosong.
    - _Requirements: 4.2, 4.3, 4.5, 4.6, 15.2, 15.3, 15.4_

  - [x]* 8.4 Test grace period di `internal/api/handlers/ispring_test.go`
    - `TestISpringGracePeriod`: cek_login dengan `login_time` lampau (durasi + 5 menit + 1 detik). POST. Assert HTTP 200 (handler tetap enqueue—grace dievaluasi di processor sesuai design existing). Run `Processor.ProcessBatch`. Assert error `"grace period exceeded"`. Setelah `Max_Retries` panggilan `MarkFailed`, file ada di `failed/`.
    - _Requirements: 15.5_

  - [x]* 8.5 Property test handler happy path di `internal/api/handlers/ispring_test.go`
    - **Property 7: Handler Happy Path Meng-enqueue Job Valid**
    - Generator: tuple (peserta no_id, mapel_id, attempt_token 32hex, score, max_score, detail_xml empty atau parseable). Setup cek_login dengan token tersebut. POST webhook dengan token sama.
    - Assert: HTTP 200, body fixed, `len(pending/) == 1`, isi file (via `UnmarshalJob`) match input field-by-field.
    - _Validates: Requirements 4.6_

- [x] 9. Startup recovery dan migrasi legacy
  - [x] 9.1 Implementasi `RecoverStartup(ctx, forceAll bool)` di `internal/submission/fsqueue.go`
    - Glob `tmp/*.json`, `os.Remove` semua (Req 7.5). Log jumlah.
    - `os.ReadDir(processing/)`. Untuk tiap file, `os.Stat`. Jika `forceAll == true` ATAU `now.Sub(stat.ModTime()) > stuckThreshold`: rename ke `pending/`. Log path tiap file dipromosi (Req 7.4).
    - `os.ReadDir(done/)`. Untuk tiap file, jika `now.Sub(stat.ModTime()) > doneRetention`: `os.Remove`. Log jumlah.
    - Update signature interface `Queue` jika perlu (atau biarkan method konkret di `*FilesystemQueue`).
    - _Requirements: 7.1, 7.2, 7.4, 7.5, Edge Case 3 di design.md_

  - [x] 9.2 Implementasi `MigrateLegacyTable(ctx, db)` di `internal/submission/fsqueue.go`
    - `SELECT id, tenant_id, no_id, score, max_score, detail_xml, attempt_token, retry_count, last_error, validasi, created_at FROM submission_queue WHERE status IN ('pending','processing')`.
    - Untuk tiap row: bangun `SubmissionJob`, panggil `q.Enqueue(ctx, job)`. Track `id` yang sukses.
    - Setelah semua sukses, dalam satu transaksi: `DELETE FROM submission_queue WHERE id IN (...)`. Jika ada error di tengah, jangan DELETE; return error agar startup gagal (Req 8.4).
    - Log jumlah baris dimigrasi dan dilewati (Req 8.5).
    - Skip gracefully jika tabel `submission_queue` tidak ada (cek via `sqlite_master`).
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [x]* 9.3 Property test recovery promote di `internal/submission/recovery_property_test.go`
    - **Property 12: Recovery Promosi File Stuck**
    - Generator: N (0..30) file di `processing/` dengan `mtime` random `[-10m, +1m]` relatif sekarang. Panggil `RecoverStartup(ctx, false)`.
    - Assert: file mtime > 5 menit lalu ada di `pending/`; file mtime ≤ 5 menit lalu tetap di `processing/`; total file (`pending` + `processing`) == N.
    - Test terpisah untuk `forceAll == true`: semua N file pindah ke `pending/`.
    - _Validates: Requirements 7.1, 7.2_

  - [x]* 9.4 Property test tmp cleanup di `internal/submission/recovery_property_test.go`
    - **Property 13: Recovery Membersihkan Tmp**
    - Generator: N (0..20) file di `tmp/` dengan nama dan isi arbitrary. Bangun juga state arbitrary di `pending/`, `processing/`, `done/`, `failed/`. Panggil `RecoverStartup`.
    - Assert: `len(tmp/) == 0`; jumlah file di sub-direktori lain tidak berkurang relatif pre-state (kecuali done > 7 hari yang dihapus retention—pisahkan kasus ini dengan mtime kontrol).
    - _Validates: Requirements 7.5_

  - [x]* 9.5 Property test migrasi legacy di `internal/submission/recovery_property_test.go`
    - **Property 14: Migrasi Legacy Lengkap**
    - Setup: SQLite tmpfile dengan tabel `submission_queue` lama. Generator: M (0..50) baris dengan distribusi status acak (`pending`, `processing`, `completed`, `failed`).
    - Panggil `MigrateLegacyTable`. Assert: `len(pending/)` baru == count(status IN ('pending','processing')) sebelumnya; baris yang dimigrasi sudah `DELETE`d; baris status lain tetap; isi file match field row asli.
    - _Validates: Requirements 8.1, 8.2, 8.3_

- [x] 10. Checkpoint - core feature siap
  - Jalankan `go test ./internal/...`. Pastikan semua hijau termasuk PBT.
  - Commit: `feat(queue): worker batching, processor atomicity, handler validation, recovery`.
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Wiring main.go dan debug endpoint
  - [x] 11.1 Wire `FilesystemQueue` di `cmd/server/main.go`
    - Hapus instansiasi `BufferedSQLiteQueue` (atau biarkan struct exist tapi tidak dipakai—dideprecate di task 13.1).
    - Parse env: `QUEUE_DIR` (default `data/queue`), `QUEUE_MAX_RETRIES` (5), `QUEUE_BATCH_SIZE` (5), `QUEUE_BATCH_TIMEOUT_MS` (100), `QUEUE_STUCK_THRESHOLD_MIN` (5), `QUEUE_DONE_RETENTION_DAYS` (7).
    - `fsQueue := submission.NewFilesystemQueue(queueDir)` → `log.Fatal` jika error.
    - `fsQueue.RecoverStartup(ctx, true)` (forceAll=true karena single-process startup, lihat Edge Case 3 design.md). `log.Fatal` jika error.
    - `fsQueue.MigrateLegacyTable(ctx, db.DB)`. `log.Fatal` jika error.
    - `processor := submission.NewProcessor(db.DB)`. `worker := submission.NewWorker(fsQueue, processor.ProcessBatch, batchSize, batchTimeout)`. `handlers.SetSubmissionQueue(fsQueue)`.
    - `go worker.Run(ctx)`. `defer worker.Stop()`. Pastikan urutan: recovery → migrasi → wiring → `app.Listen` (Req 7.3, 9.x).
    - _Requirements: 7.3, 9.1, 9.2, 16.1, 16.2, 16.3, 16.4, 17.1, 17.2, 17.5_

  - [x] 11.2 Update `internal/api/handlers/debug_queue.go` untuk `done_count`
    - Pastikan handler memanggil `Queue.GetStats(ctx)` dan men-serialize seluruh `QueueStats` termasuk `DoneCount`.
    - Handle error: jika `GetStats` return error, response HTTP 500 body `{"error": "<deskripsi>"}` (Req 12.5).
    - _Requirements: 12.1, 12.2, 12.5, 12.6_

  - [x]* 11.3 Update test debug endpoint di `internal/api/handlers/debug_queue_test.go` (atau `features_test.go` jika di sana)
    - Setup `FilesystemQueue` di tmpdir dengan jumlah file kontrol di tiap sub-direktori. GET `/api/debug/queue`. Assert JSON berisi `done_count` dan nilai cocok.
    - Test error path: hapus salah satu sub-direktori manual, GET, assert HTTP 500 dengan body JSON `error`.
    - _Requirements: 12.1, 12.2, 12.5_

- [x] 12. End-to-end load test
  - [x] 12.1 Revisi `tests/load/verify_e2e.go` untuk skala 500 dan assert E2E
    - Tambah CLI flag `-scale=500` (default 500). Setup 500 peserta dengan cek_login dan attempt_token random per peserta.
    - 500 goroutine concurrent POST `/api/ispring/webhook` dengan payload XML 10..30 questions.
    - Poll `GET /api/debug/queue` interval 1s sampai `pending + processing == 0` atau timeout 5 menit. Catat waktu drain.
    - Assert: `COUNT(*) FROM hasil_tes WHERE tenant_id=? AND validasi LIKE '<tenant>_%'` == 500; `len(failed/) == 0`; `len(processing/)` setelah drain == 0; throughput = 500 / waktu_drain >= 3 jobs/sec; handler P95 < 100ms.
    - Output ringkas ke stdout dan tulis ke `tests/load/E2E_RESULTS.md` (append run baru dengan timestamp).
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5_

  - [x]* 12.2 Jalankan `verify_e2e.go` di skala 50, 100, 200, 500
    - `go run tests/load/verify_e2e.go -scale=50`, `-scale=100`, `-scale=200`, `-scale=500` berturut-turut, dengan reset state queue antar run (`rm -rf data/queue/*`).
    - Konfirmasi 100% E2E success dan 0 file di `failed/` di setiap skala.
    - Jika ada skala yang fail, kembali ke task terkait (likely 7.x, 9.x, atau 11.1) dan diagnosa.
    - _Requirements: 13.1, 13.3, 13.4_
    - Catatan run lokal 2026-05-26 19:14: skala 50/100/200/500 mencapai 100% persist dan 0 failed. Optimasi single-writer enqueue menurunkan P95 500 ke 314ms, tetapi budget HTTP P95 <100ms baru terpenuhi sampai skala 100. Lihat `tests/load/E2E_RESULTS.md`.

- [x] 13. Cleanup deprecation dan dokumentasi
  - [x] 13.1 Tandai `BufferedSQLiteQueue` deprecated di `internal/submission/queue.go`
    - Tambah komentar `// Deprecated: gunakan FilesystemQueue. Dipertahankan untuk test fixture lama saja.` di atas `BufferedSQLiteQueue` dan konstruktornya.
    - Pastikan tidak ada caller di production code (selain test). Jika ada call site di tempat tak terduga, update atau dokumentasikan migrasi.
    - _Requirements: 16.1, 16.4_

  - [x] 13.2 Update `docs/runbooks/queue-and-litestream.md` untuk filesystem queue
    - Section baru: "Memantau Queue via File Explorer". Cara buka `data/queue/`, arti masing-masing sub-direktori, cara menafsirkan `failed/<file>.json` + `<file>.error.txt`.
    - Section baru: "Konfigurasi env var queue" (`QUEUE_DIR`, `QUEUE_MAX_RETRIES`, dll.).
    - Section baru: "Migrasi dari versi lama" (apa yang terjadi saat startup pertama setelah upgrade).
    - Tandai bagian SQL `submission_queue` lama sebagai legacy/historis.
    - _Requirements: 12.1, 12.2_

- [ ] 14. Final checkpoint - feature lengkap
  - Jalankan `go test ./...` (full repo). Konfirmasi semua hijau.
  - Konfirmasi load test skala 500 lulus (12.2).
  - Catatan 2026-05-26: functional E2E skala 500 sudah lulus (500/500 tersimpan, 0 failed, drain 1.212s), tetapi SLA handler P95 <100ms belum lulus untuk burst 200/500. Jangan merge sebagai "feature lengkap" sebelum tim memutuskan apakah SLA ini wajib atau disesuaikan untuk pilot.
  - Commit final: `chore(queue): deprecate buffered sqlite queue, update runbook`.
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Sub-tasks bertanda `*` adalah test (property-based atau example-based). Wajib dijalankan sebelum merge ke main; opsional untuk MVP iteratif.
- Property test mengacu langsung ke nomor properti di design.md section "Correctness Properties".
- Setiap task major (1-13) idealnya selesai dengan satu commit. Boleh lebih kecil jika perlu.
- Branch `feature/filesystem-queue` di-merge ke main hanya setelah load test 500 lulus dan checkpoint task 14 hijau.
- Library PBT: `pgregory.net/rapid` (alasan di design.md "Library PBT").
- Anti-pattern yang dihindari: stand-alone test task (semua test adalah sub-task di bawah implementasi), top-level task ber-`*` (hanya sub-task yang boleh opsional), perubahan signature interface `Queue` yang merusak `InMemoryQueue`/`SQLiteQueue` (tetap kompatibel dengan tambahan field `DoneCount`).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1"] },
    { "id": 2, "tasks": ["2.2"] },
    { "id": 3, "tasks": ["2.3", "3.1"] },
    { "id": 4, "tasks": ["2.4", "3.2"] },
    { "id": 5, "tasks": ["3.3"] },
    { "id": 6, "tasks": ["3.4"] },
    { "id": 7, "tasks": ["3.5"] },
    { "id": 8, "tasks": ["4.1", "6.1", "7.1", "8.1"] },
    { "id": 9, "tasks": ["4.2", "6.2", "7.2", "8.2"] },
    { "id": 10, "tasks": ["4.3", "7.3", "8.3"] },
    { "id": 11, "tasks": ["4.4", "7.4", "8.4"] },
    { "id": 12, "tasks": ["4.5", "8.5", "9.1"] },
    { "id": 13, "tasks": ["4.6", "9.2"] },
    { "id": 14, "tasks": ["9.3", "11.1", "11.2"] },
    { "id": 15, "tasks": ["9.4", "11.3"] },
    { "id": 16, "tasks": ["9.5"] },
    { "id": 17, "tasks": ["12.1", "13.1", "13.2"] },
    { "id": 18, "tasks": ["12.2"] }
  ]
}
```
