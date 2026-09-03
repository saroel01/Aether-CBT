# Codebase Bug Sweep Bugfix Design

## Overview

Dokumen ini merancang perbaikan atas 25 klausa defect di `bugfix.md`. Temuan-temuan itu bukan 25 bug independen: ada satu **akar tunggal yang mendominasi** (klausa 1.1 — DSN SQLite memakai sintaks driver yang salah sehingga `journal_mode`, `foreign_keys`, dan `busy_timeout` tidak pernah aktif) dan sebagian besar klausa lain adalah *konsekuensi* atau *bug yang tersembunyi di belakang* akar itu. Karena itu strategi perbaikan bersifat berlapis, bukan paralel:

1. **Lapis 0 — Prasyarat data.** Sebelum `foreign_keys(on)` boleh dinyalakan, semua penulis nilai sentinel `0` ke kolom ber-FK harus dinormalkan menjadi `NULL` dan data eksisting harus dibersihkan (klausa 2.2). Jika urutan ini dibalik, `Connect` akan sukses tetapi INSERT peserta di produksi langsung gagal.
2. **Lapis 1 — Aktifkan pragma.** Ganti DSN ke bentuk `file:<path>?_pragma=...` dan tambahkan verifikasi pasca-koneksi yang gagal cepat (klausa 2.1).
3. **Lapis 2 — Integritas queue.** Backoff yang benar-benar berlaku (2.3) dan isolasi kegagalan per job (2.4). Keduanya melindungi nilai siswa yang saat ini bisa hilang secara silent.
4. **Lapis 3 — Isolasi tenant.** Tiga jalur yang bocor: attach sesi (2.5), unlink mapping (2.6), room-status (2.7).
5. **Lapis 4 — P1/P2 cleanup.** 18 klausa sisanya adalah perbaikan lokal, sebagian besar dengan pola yang sudah ada di kode (`requireAffected`, `ErrNotFound`, `ParamsInt`, `middleware.RequireRoles`) yang cukup diterapkan secara konsisten.

Prinsip utama: **perubahan minimal, pola yang sudah ada dipakai ulang, tidak ada dependensi eksternal baru.** Aether-CBT tetap satu binary `aether-cbt.exe` yang berjalan offline di LAN sekolah (klausa 3.3).

Baseline verifikasi saat ini bersih (`go vet ./...` exit 0, `go test ./...` hijau), sehingga setiap perbaikan harus disertai test baru yang **gagal sebelum fix dan lulus sesudahnya** — tanpa itu, tidak ada bukti bug pernah ada.

## Glossary

- **Bug_Condition (C)** — Kondisi input yang memicu defect. Karena spec ini menyapu 25 klausa, C adalah disjungsi 25 sub-kondisi `C₁..C₂₅`, masing-masing dipetakan satu-ke-satu ke klausa 1.x. Sebuah input berada di C bila memenuhi setidaknya satu `Cᵢ`.
- **Property (P)** — Perilaku benar yang harus dipenuhi untuk input di C, didefinisikan oleh klausa 2.x yang berpasangan indeks dengan 1.x.
- **Preservation** — Perilaku eksisting untuk input di luar C (`¬C`) yang wajib identik sebelum dan sesudah perbaikan; didefinisikan oleh klausa 3.1–3.20.
- **F** — Kode sebelum perbaikan (unfixed). **F'** — kode setelah perbaikan.
- **DSN pragma syntax** — Bentuk parameter koneksi yang dikenali `modernc.org/sqlite`: `file:<path>?_pragma=name(value)`. Berbeda dari sintaks `mattn/go-sqlite3` (`?_journal_mode=WAL`) yang **diabaikan tanpa error** oleh driver ini.
- **Sentinel 0** — Nilai `0` yang dipakai sebagai pengganti "tidak diketahui" pada kolom `peserta.kelas_id` / `peserta.ruang_id`; bukan `NULL`, sehingga melanggar FOREIGN KEY begitu penegakan aktif.
- **Due time (waktu jatuh tempo)** — Instan paling awal sebuah job gagal boleh di-dequeue ulang, hasil backoff `min(2^(n-1), 30)` detik.
- **Dead letter** — Direktori `failed/` beserta file pendamping `<name>.error.txt` yang dibaca admin sesuai `docs/runbooks/queue-and-litestream.md`.
- **`Connect`** — `internal/db/sqlite.go`; satu-satunya pintu pembukaan koneksi, dipakai oleh `cmd/server`, `cmd/seed`, `cmd/createadmin`, `cmd/migratepasswords`, dan `internal/testutil`.
- **`FilesystemQueue`** — `internal/submission/fsqueue.go`; queue berbasis file dengan transisi state via `os.Rename` atomic dan FIFO yang bergantung pada prefix `unix_nano` di nama file.
- **`ProcessBatch` / `processBatchSafe`** — Pasangan `internal/submission/processor.go` dan `worker.go` yang saat ini hanya mengenal outcome all-or-nothing per batch.

## Bug Details

### Bug Condition

Bug termanifestasi ketika sebuah input memenuhi salah satu dari 25 sub-kondisi berikut. Sub-kondisi dikelompokkan menurut mekanisme kegagalannya, bukan menurut file, karena beberapa klausa berbagi akar yang sama.

**Kelompok A — Konfigurasi driver tidak berlaku (akar tunggal).**
`Connect` menyusun DSN dengan sintaks `mattn/go-sqlite3`; `modernc.org/sqlite` mem-parse nama file dengan benar tetapi **membuang seluruh parameter tanpa error**. Akibatnya koneksi berjalan dengan `journal_mode=delete`, `busy_timeout=0`, `foreign_keys=0`. Ini menyebabkan klausa 1.1, menyembunyikan 1.2, dan menjadi alasan keberadaan retry loop SQLITE_BUSY ad-hoc di `migrate.go` serta `exam_session_repo.go`, dan alasan mengapa 1.25 belum pernah berakibat deadlock.

**Kelompok B — Jadwal dan outcome queue tidak dihormati.**
Backoff dihitung lalu disimpan ke field yang tidak pernah dibaca oleh pemilih kandidat (1.3); outcome batch bersifat all-or-nothing sehingga job valid ikut dihukum (1.4); dead letter tidak menuliskan konteks kegagalan (1.10); `Stop` tidak idempotent (1.8); goroutine writer tidak pernah dilepas (1.9).

**Kelompok C — Predikat kueri kehilangan dimensi.**
Sebuah kueri memfilter pada sebagian dimensi yang seharusnya: `AttachClasses`/`AttachRooms` memvalidasi kepemilikan kelas/ruang tetapi tidak kepemilikan sesi (1.5); `UnlinkClassSubject` tidak memfilter tenant (1.6); `fetchRoomStatus` menjoin `hasil_tes` hanya pada peserta, bukan pada sesi (1.7); `DeleteClass`/`DeleteRoom` kehilangan `deleted_at IS NULL` (1.18).

**Kelompok D — Error diabaikan atau ditelan.**
Nilai kembalian error dibuang: `rows.Scan` dan `rows.Err()` di sembilan handler list/export (1.15), `HashPassword` di `CreateRoom` (1.21), `err` yang ditimpa oleh cabang fallback di `GetAvailableMapels` (1.17), `NULL` yang menyebabkan `continue` senyap di `ExportResultsCSV` (1.16), agregat `NULL` yang gagal di-scan di `GetStats` (1.11).

**Kelompok E — Guard input kosong tidak ada.**
Token kosong diteruskan ke kueri dan bergantung pada asumsi data: `StudentLogin` dengan token `''` (1.12), `ISpringWebhook` dengan `attempt_token` `''` (1.13).

**Kelompok F — Dua sumber kebenaran yang tidak sinkron.**
`"auto"` yang runtuh menjadi `false` (1.14); role check ganda middleware vs handler (1.19); konvensi interval `enterable` inklusif vs `windowsOverlap` setengah terbuka (1.24).

**Kelompok G — Operasi non-atomic dan salah literal.**
`UPDATE` lalu `SELECT` terpisah pada `IncrementInfraction` (1.22); `os.Exit` via `log.Fatal` yang melewati semua `defer` (1.23); `TrimSuffix` yang meng-lowercase literal bukan nama berkas (1.20); pembacaan diagnostik di luar `tx` (1.25).

**Formal Specification:**

```
FUNCTION isBugCondition(input)
  INPUT:  input of type SystemInput
          (satu dari: DSN+connection request, HTTP request, queue job,
           konfigurasi runtime, atau signal proses)
  OUTPUT: boolean

  // Kelompok A - konfigurasi driver
  IF input.kind = CONNECTION AND input.dsn USES legacyParamSyntax THEN
    RETURN true                                              // C1  (1.1)
  IF input.kind = HTTP AND input.route = "POST /api/students"
     AND (input.body.kelas_id IS ABSENT OR input.body.kelas_id = 0
          OR input.body.ruang_id IS ABSENT OR input.body.ruang_id = 0) THEN
    RETURN true                                              // C2  (1.2)

  // Kelompok B - queue
  IF input.kind = QUEUE_JOB AND input.job.RetryCount > 0
     AND now < dueTimeOf(input.job) THEN
    RETURN true                                              // C3  (1.3)
  IF input.kind = QUEUE_BATCH AND COUNT(j IN input.batch WHERE fails(j)) >= 1
     AND COUNT(j IN input.batch WHERE NOT fails(j)) >= 1 THEN
    RETURN true                                              // C4  (1.4)
  IF input.kind = STOP_CALL AND input.callIndex > 1 THEN
    RETURN true                                              // C8  (1.8)
  IF input.kind = QUEUE_LIFECYCLE AND input.phase = DISPOSE THEN
    RETURN true                                              // C9  (1.9)
  IF input.kind = QUEUE_JOB AND input.job.RetryCount >= maxRetries THEN
    RETURN true                                              // C10 (1.10)

  // Kelompok C - predikat kueri kehilangan dimensi
  IF input.kind = HTTP AND input.route IN {sessionClasses, sessionRooms}
     AND tenantOf(input.sessionID) <> input.callerTenantID THEN
    RETURN true                                              // C5  (1.5)
  IF input.kind = HTTP AND input.route = unlinkClassSubject
     AND (tenantOf(input.kelas_id) <> input.callerTenantID
          OR input.kelas_id IS NOT VALID INTEGER) THEN
    RETURN true                                              // C6  (1.6)
  IF input.kind = HTTP AND input.route IN {roomStatus, roomStatusSSE}
     AND EXISTS peserta p WHERE COUNT(hasil_tes WHERE peserta_id = p.id) <> 1 THEN
    RETURN true                                              // C7  (1.7)
  IF input.kind = HTTP AND input.route IN {deleteClass, deleteRoom}
     AND (input.id IS NOT VALID INTEGER
          OR targetRow IS ABSENT
          OR targetRow.deleted_at IS NOT NULL) THEN
    RETURN true                                              // C18 (1.18)

  // Kelompok D - error diabaikan
  IF input.kind = HTTP AND input.route IN listExportRoutes
     AND (rowScanFails(input) OR rowsIterationFails(input)) THEN
    RETURN true                                              // C15 (1.15)
  IF input.kind = HTTP AND input.route = exportResultsCSV
     AND EXISTS row WHERE row.skor IS NULL OR row.skor_maks IS NULL THEN
    RETURN true                                              // C16 (1.16)
  IF input.kind = HTTP AND input.route = availableMapels
     AND classScopedQueryFails(input) THEN
    RETURN true                                              // C17 (1.17)
  IF input.kind = HTTP AND input.route = createRoom
     AND (hashPasswordFails(input)
          OR input.nama_ruang = "" OR input.username = "" OR input.password = ""
          OR usernameAlreadyExists(input.username)) THEN
    RETURN true                                              // C21 (1.21)
  IF input.kind = STATS_CALL AND rowCountOf("submission_queue") = 0 THEN
    RETURN true                                              // C11 (1.11)

  // Kelompok E - guard input kosong
  IF input.kind = HTTP AND input.route = studentLogin
     AND trim(input.token) = "" THEN
    RETURN true                                              // C12 (1.12)
  IF input.kind = HTTP AND input.route = ispringWebhook
     AND trim(input.attempt_token) = "" THEN
    RETURN true                                              // C13 (1.13)

  // Kelompok F - dua sumber kebenaran
  IF input.kind = CONFIG AND input.ContentCookieSecure = "auto"
     AND requestIsTLSTerminatedUpstream(input) THEN
    RETURN true                                              // C14 (1.14)
  IF input.kind = HTTP AND input.callerRole = "superadmin"
     AND input.route IN adminRoutesWithHandlerRoleCheck THEN
    RETURN true                                              // C19 (1.19)
  IF input.kind = SCHEDULING AND input.instant IN {session.mulai, session.selesai}
     AND EXISTS otherSession ADJACENT TO input.session THEN
    RETURN true                                              // C24 (1.24)

  // Kelompok G - non-atomic dan salah literal
  IF input.kind = INFRACTION AND concurrentCallers(input) >= 2 THEN
    RETURN true                                              // C22 (1.22)
  IF input.kind = SIGNAL AND input.signal IN {SIGINT, SIGTERM} THEN
    RETURN true                                              // C23 (1.23)
  IF input.kind = HTTP AND input.route = uploadSoalPackage
     AND hasZipExtension(input.filename)
     AND extensionOf(input.filename) <> ".zip" THEN
    RETURN true                                              // C20 (1.20)
  IF input.kind = QUEUE_JOB AND diagnosticBranchTaken(input) THEN
    RETURN true                                              // C25 (1.25)

  RETURN false
END FUNCTION
```

### Examples

Setiap contoh di bawah adalah counterexample konkret pada F. Semuanya dapat dieksekusi hari ini dan **tidak satu pun tertangkap oleh test suite yang ada**.

- **1.1 (terverifikasi empiris).** Probe terhadap `modernc.org/sqlite` v1.50.1: `sql.Open("sqlite", "path.db?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")` menghasilkan `PRAGMA journal_mode` → `delete`, `busy_timeout` → `0`, `foreign_keys` → `0`. Bentuk yang benar, `sql.Open("sqlite", "file:path.db?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)")`, menghasilkan `wal`, `5000`, `1`. Bukti pendukung di lapangan: `data/cbt_aether.db` tidak memiliki sidecar `-wal`, artinya WAL memang belum pernah aktif pada DB live. Diharapkan: ketiga pragma aktif; aktual: tidak ada yang aktif, tanpa error apa pun.
- **1.2.** `POST /api/students` dengan body `{"no_id":"123","nama":"Budi"}` menyimpan `peserta.kelas_id = 0`, `ruang_id = 0`. Baris itu melanggar FK ke `kelas(id)`/`ruang(id)`. Diharapkan: `NULL`; aktual: `0` yang lolos hanya karena penegakan FK mati.
- **1.3.** Job gagal pada percobaan pertama menerima `EnqueuedAt = now + 1s`, tetapi `Dequeue` memilih kandidat dari urutan nama `os.ReadDir` dan tidak pernah membaca `EnqueuedAt`. Karena prefix `unix_nano` job itu paling tua di `pending/`, ia selalu terpilih pertama. Diharapkan: dilewati sampai jatuh tempo; aktual: 5 percobaan habis dalam hitungan milidetik sementara submission baru kelaparan.
- **1.4.** Batch berisi 5 job, job ke-3 melampaui grace period. Diharapkan: 4 job commit dan pindah ke `done/`, 1 job ke `failed/`; aktual: seluruh transaksi rollback, `MarkFailed` dipanggil untuk kelima job, dan 4 hasil ujian valid akhirnya berakhir di `failed/`.
- **1.5.** Admin tenant A memanggil `POST /api/exam-sessions/{id_milik_tenant_B}/classes` dengan `kelas_id` miliknya sendiri. Diharapkan: 404/403 tanpa perubahan data; aktual: penulisan berhasil, kelas tenant A tertaut ke sesi tenant B.
- **1.6.** Admin tenant A memanggil `DELETE` unlink dengan `kelas_id`/`mapel_id` milik tenant B. Diharapkan: 404; aktual: mapping kurikulum tenant B terhapus.
- **1.7.** Seorang siswa dengan 3 baris `hasil_tes` muncul 3 kali di `GET /api/supervisor/room-status`, bertentangan dengan komentar fungsinya sendiri ("satu baris per siswa"). Dengan `session_id` diberikan, `skor` yang tampil dapat berasal dari ujian lain karena subquery `cek_login` dibatasi per sesi sedangkan join `hasil_tes` tidak.
- **1.8.** `worker.Stop()` eksplisit di test lalu `defer worker.Stop()` → `panic: close of closed channel`.
- **1.11.** `SQLiteQueue.GetStats` pada `submission_queue` kosong: `SUM(CASE ...)` → `NULL`, `Scan` ke `int` gagal. Diharapkan: statistik nol; aktual: error.
- **1.12.** `StudentLogin` dengan `token: ""`. Karena `migrateTenant` menyalin `settings.token` apa adanya, sebuah `exam_session` dapat benar-benar memiliki token `''`; jika sesi itu enterable, siswa login tanpa token. Cabang fallback legacy yang menolak token kosong tidak pernah tercapai.
- **1.14.** Produksi dengan `ContentCookieSecure="auto"` di belakang reverse proxy yang menerminasi TLS: `cfg.ContentCookieSecure == "true"` → `false`, lalu `c.Secure()` → `false`. Cookie `aether_exam` dikirim tanpa atribut `Secure`.
- **1.16.** Baris `hasil_tes` dengan `skor` `NULL` menyebabkan `continue`; siswa itu **hilang total** dari file ekspor tanpa peringatan kepada admin.
- **1.19.** `superadmin` memanggil `UpdateSettings`: `middleware.RequireRoles("admin","superadmin")` meloloskan, lalu handler menolak dengan 403 karena `role != "admin"`.
- **1.20.** Unggah `PAKET.ZIP`: `strings.TrimSuffix(file.Filename, strings.ToLower(".zip"))` meng-lowercase literal `".zip"` (yang sudah lowercase), bukan nama berkasnya. Diharapkan tersimpan sebagai `PAKET`; aktual `PAKET.ZIP`.
- **1.22.** Dua pelanggaran bersamaan, ambang lock 3, counter di 2. Kedua pemanggil menjalankan `UPDATE` lalu `SELECT` terpisah dan keduanya membaca `3`; keduanya (atau tak satu pun, tergantung interleaving) memicu lock. Diharapkan: satu pemanggil membaca `3`, satu membaca `4`, lock tepat sekali.
- **1.23.** `Ctrl+C` pada server: `log.Fatal(app.Listen(...))` memanggil `os.Exit`, sehingga `defer cancel()`, `defer worker.Stop()`, dan `defer db.Close()` tidak jalan. Job yang tertinggal di `processing/` baru dipulihkan oleh sweep stuck-threshold — dengan `RecoverStartup(forceAll=false)` dan threshold 5 menit, hasil ujian dapat tertahan beberapa menit setelah restart.
- **1.24 (edge case).** Sesi X `[10:00, 11:00]`, sesi Y `[11:00, 12:00]`, token sama, instan `11:00`. `windowsOverlap` setengah terbuka `[start, end)` menyatakan tidak bertumpang tindih, tetapi `enterable` inklusif di kedua ujung menyatakan **keduanya** dapat dimasuki pada `11:00`.

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**

- **Migrasi dan test suite.** 20+ migrasi tetap berjalan sampai selesai dan seluruh paket tetap lulus meskipun `foreign_keys` kini aktif; `go vet ./...` tetap exit 0 dan `go test ./...` tetap hijau (3.1).
- **Relasi peserta yang valid.** `CreateStudent`, seed, dan import CSV dengan `kelas_id`/`ruang_id` `> 0` yang ada di tabel induk tetap menyimpan relasi apa adanya dengan respons identik (3.2).
- **Bentuk deployment.** Tetap single binary offline-first di LAN sekolah pada port yang sama, tanpa dependensi service eksternal baru (3.3).
- **Jalur sukses queue.** Job yang berhasil pada percobaan pertama tetap pindah ke `done/` tanpa penundaan tambahan — backoff hanya berlaku bagi job yang gagal (3.4).
- **Atomisitas dan performa batch.** Batch yang seluruh jobnya valid tetap ditulis dalam **satu** transaksi atomic dan tetap memenuhi target throughput batch serta latency handler pada burst 200 dan 500 siswa (3.5). Ini membatasi ruang solusi 2.4 secara tegas: tidak boleh ada regresi ke satu-transaksi-per-job pada jalur normal.
- **Kegagalan permanen tetap dead letter.** Job yang memang gagal permanen tetap pindah ke `failed/` setelah `maxRetries` terlampaui (3.6).
- **Operasi tenant sendiri.** Attach sesi–kelas/ruang, link, dan unlink pada data milik tenant sendiri tetap selesai dengan respons sukses yang sama (3.7).
- **Room-status kasus normal.** Peserta dengan tepat satu `hasil_tes` untuk sesi yang diminta tetap menampilkan `skor`, `status`, `waktu_selesai` yang sama; peserta tanpa hasil tetap muncul sebagai satu baris bernilai kosong — semantik LEFT JOIN dipertahankan (3.8).
- **`Stop` pemanggilan pertama.** Tetap menghentikan worker beserta loop dequeue-nya seperti sekarang (3.9).
- **Login token benar/salah.** Token tidak kosong dan benar tetap berhasil; token salah tetap ditolak dengan status dan pesan error yang sama (3.10).
- **Webhook token valid.** Tetap 200 dan tetap meng-enqueue job (3.11).
- **Cookie eksplisit dan LAN tanpa TLS.** `ContentCookieSecure` `"true"`/`"false"` tetap dihormati; ujian di LAN sekolah tanpa TLS tetap dapat menetapkan cookie `aether_exam` sehingga siswa tetap bisa mengerjakan ujian (3.12). Ini konstrain paling ketat untuk 2.14: deteksi `"auto"` tidak boleh membuat cookie gagal di-set pada HTTP polos.
- **Payload list dan export.** Ketika semua baris dapat di-scan, setiap endpoint list dan export tetap mengembalikan payload, urutan, dan format identik — termasuk header dan kolom pada keluaran CSV, XLSX, dan PDF (3.13).
- **Scope mapel siswa.** `GetAvailableMapels` tetap mengembalikan hanya mapel yang dimapping ke kelas siswa (3.14).
- **Otorisasi admin dan penolakan non-admin.** Role `admin` tetap diizinkan; siswa dan pengawas tetap ditolak dengan 403 (3.15).
- **Soft delete.** `DeleteClass`, `DeleteRoom`, `DeleteStudent` dengan `id` valid yang menunjuk baris aktif tetap melakukan soft delete dengan respons sukses yang sama (3.16).
- **Nama paket lowercase.** Unggahan `.zip` huruf kecil tetap tersimpan tanpa ekstensi seperti sekarang (3.17).
- **`CreateRoom` jalur valid.** Data lengkap, unik, dan valid tetap membuat ruang, dan hash password-nya tetap dapat diverifikasi saat login pengawas (3.18).
- **Infraction tunggal.** Satu pelanggaran tetap mengembalikan counter yang benar dan tetap mengunci sesi saat ambang tercapai (3.19).
- **Scheduling non-batas.** Waktu yang jelas di dalam atau di luar jendela tetap memberi keputusan enterable dan overlap yang sama (3.20).

**Scope:**

Seluruh input yang **tidak** memenuhi `isBugCondition` harus sama sekali tidak terpengaruh. Konkretnya:

- Request HTTP di rute yang tidak disebut klausa 1.x, dan request di rute yang disebut namun dengan input yang sah (id valid, token tidak kosong, tenant cocok, tidak ada `NULL`/scan error).
- Job queue yang sukses pada percobaan pertama, dan batch yang seluruh anggotanya valid.
- Koneksi database untuk data yang sudah konsisten secara referensial.
- Pemanggil dengan role `admin` dan penolakan role non-admin.
- Evaluasi scheduling pada waktu yang bukan instan batas jendela.
- Unggahan berkas dengan ekstensi `.zip` huruf kecil.

**Catatan:** perilaku benar yang diharapkan untuk input **di dalam** C didefinisikan pada bagian Correctness Properties (Property 1). Bagian ini khusus mendefinisikan apa yang **tidak boleh berubah**.

**Konsekuensi khusus untuk artefak test yang sudah ada.** `internal/submission/fsqueue_property_test.go` saat ini meng-assert regex nama file `^\d{19}-\d+-[A-Za-z0-9_-]+-[0-9a-f]{8}\.json$` dan meng-assert backoff `MarkFailed` melalui `EnqueuedAt`. Kedua assertion itu mengunci **implementasi**, bukan perilaku produk, dan klausa 3.x tidak melindunginya. Bila 2.3 diselesaikan dengan mengubah skema nama file, kedua test itu **wajib** diperbarui bersamaan dengan fix — perubahan test ini adalah bagian eksplisit dari lingkup pekerjaan, bukan regresi.

## Hypothesized Root Cause

Berdasarkan analisis kode (dan satu probe empiris untuk klausa 1.1), penyebabnya jatuh ke tujuh kategori. Untuk setiap klausa dicantumkan **akar** dan **lokasi fix** yang konkret.

### 1. Asumsi sintaks driver yang tidak diverifikasi (akar dominan)

Kode ditulis dengan sintaks DSN `mattn/go-sqlite3` sementara dependensinya adalah `modernc.org/sqlite`. Kedua driver mendaftar dengan nama berbeda (`sqlite3` vs `sqlite`) dan mem-parse parameter dengan cara berbeda, tetapi `modernc` **mengabaikan parameter tak dikenal tanpa error** — kegagalan sepenuhnya senyap. Tidak ada verifikasi pasca-koneksi, sehingga tidak ada titik di mana kesalahan ini bisa terdeteksi. Komentar doc `Connect` bahkan mengklaim "WAL mode, foreign keys on, busy timeout" sehingga pembaca berikutnya tidak punya alasan untuk curiga.

- **1.1** → akar: sintaks DSN salah + tidak ada verifikasi. Fix: `internal/db/sqlite.go` `Connect`.
- **1.2** → akar: sentinel `0` dipakai sebagai pengganti `NULL`; **bug ini selalu ada** namun tidak pernah terlihat karena penegakan FK mati. Fix: `internal/api/handlers/student.go` `CreateStudent`, plus audit `cmd/seed/main.go` dan `internal/api/handlers/csv_utility.go` `ImportStudentsCSV`, plus satu migrasi pembersih data.
- **1.25** → akar yang sama secara laten: pembacaan diagnostik `p.db.QueryRowContext` pada koneksi terpisah saat `tx` terbuka tidak berbahaya di bawah `journal_mode=delete` dengan pool 25 koneksi, tetapi menjadi risiko self-deadlock begitu perilaku journal/locking berubah. Fix: `internal/submission/processor.go` `processOneInTx`.

**Blast radius FK.** Begitu `foreign_keys(on)` benar-benar aktif, deklarasi FOREIGN KEY di migrasi berikut menjadi hidup: `002` `users.tenant_id`→`tenants` CASCADE; `005` `settings`, `006` `ruang`, `007` `kelas`, `008` `mapel` `.tenant_id`→`tenants` CASCADE; `009` `peserta.tenant_id`→`tenants` CASCADE, `peserta.kelas_id`→`kelas(id)`, `peserta.ruang_id`→`ruang(id)`; `010` `hasil_tes.tenant_id`→`tenants` CASCADE, `.peserta_id`→`peserta(id)`, `.mapel_id`→`mapel(id)`; `011` `cek_login.tenant_id`→`tenants` CASCADE, `.peserta_id`→`peserta(id)`; `013` `kelas_mapel.kelas_id`→`kelas(id)`, `.mapel_id`→`mapel(id)`; `014` `hasil_tes_detail.hasil_tes_id`→`hasil_tes(id)` CASCADE; `019` `submission_queue.tenant_id` dan `failed_submissions.tenant_id`→`tenants` CASCADE; `021` `soal_package.tenant_id`→`tenants` CASCADE; `022` `exam.tenant_id`→`tenants` CASCADE, `.mapel_id`→`mapel(id)`, `.soal_package_id`→`soal_package(id)`; `023` `exam_session.tenant_id`→`tenants` CASCADE, `.exam_id`→`exam(id)`; `024` `exam_session_kelas.session_id`→`exam_session(id)` CASCADE dan `.kelas_id`→`kelas(id)`, `exam_session_ruang.session_id`→`exam_session(id)` CASCADE dan `.ruang_id`→`ruang(id)`; `029` `kelas_mapel.tenant_id`→`tenants(id)` (via `ALTER TABLE ADD COLUMN`); `031` `hasil_tes.exam_session_id`→`exam_session(id)` (via `ALTER TABLE ADD COLUMN`).

Karena soft delete (`deleted_at`) dipakai di seluruh kode, penegakan FK terutama memengaruhi **INSERT/UPDATE**, bukan DELETE — kecuali jalur CASCADE. Satu-satunya penulis sentinel-`0` yang sudah teridentifikasi adalah `CreateStudent`; `cmd/seed/main.go` dan `ImportStudentsCSV` masih harus diaudit sebelum pragma dinyalakan.

### 2. Nilai dihitung lalu disimpan ke tempat yang tidak pernah dibaca

`MarkFailed` menghitung backoff dengan benar dan menuliskannya ke `job.EnqueuedAt`, tetapi `Dequeue` memilih kandidat **murni dari urutan nama file** dan tidak pernah men-deserialisasi job sebelum meng-klaimnya. Ada dua sumber kebenaran untuk "kapan job boleh jalan": nama file (dipakai) dan `EnqueuedAt` (diabaikan). Komentar `MarkFailed` sendiri menyatakan nama file sengaja dipertahankan "agar admin dapat mengkorelasi file antar direktori" — niat yang sah, dan itulah yang membuat penulis kode tidak menyandikan jadwal ke nama file.

- **1.3** → akar: jadwal disimpan di field yang tidak menjadi input keputusan `Dequeue`. Fix: `internal/submission/fsqueue.go` `Dequeue` + `MarkFailed`.

### 3. Granularitas outcome tidak cocok antar dua lapis

`ProcessBatch` hanya bisa melaporkan satu error untuk seluruh batch (`BeginTx` → loop `processOneInTx` → `Commit`, dengan `return err` pada kegagalan pertama sehingga `defer tx.Rollback()` membatalkan semuanya). `processBatchSafe` hanya bisa bereaksi biner: seluruh batch `MarkCompleted` atau seluruh batch `MarkFailed`. Tidak ada kontrak per-job di antara keduanya, padahal kegagalan bersifat per-job.

- **1.4** → akar: tidak ada kontrak outcome per-job antara processor dan worker. Fix: `internal/submission/processor.go` `ProcessBatch` + `internal/submission/worker.go` `processBatchSafe`.

### 4. Lifecycle resource tidak dikelola

Primitif konkurensi dibuat tanpa jalur pelepasan: `stopChan` ditutup tanpa pengaman idempotensi, dan `enqueueCh` tidak pernah ditutup sehingga `runEnqueueWriter` hidup selamanya. Pola ini lolos karena di produksi proses hanya hidup sekali; hanya test yang membuat banyak instance yang merasakannya.

- **1.8** → akar: `close()` tanpa guard. Fix: `internal/submission/worker.go` `Stop` (`sync.Once`).
- **1.9** → akar: tidak ada metode `Close`/`Shutdown`. Fix: `internal/submission/fsqueue.go` (`NewFilesystemQueueWithConfig` + method baru) dan seluruh pemanggil.
- **1.23** → akar: `log.Fatal` memanggil `os.Exit` sehingga semua `defer` dilewati. Fix: `cmd/server/main.go`.

### 5. Predikat kueri kehilangan satu dimensi

Kueri memfilter sebagian dimensi yang relevan. Dalam sistem multi-tenant, `tenant_id` adalah dimensi wajib; dalam sistem multi-sesi, `session_id` juga. Pola kegagalannya konsisten: **objek yang dinamai di body request divalidasi, objek yang dinamai di path URL tidak**.

- **1.5** → akar: kepemilikan tenant atas `sessionID` (dari URL) tidak pernah diverifikasi, hanya kepemilikan kelas/ruang (dari body). Fix: `internal/repository/exam_session_repo.go` `AttachClasses`/`AttachRooms`.
- **1.6** → akar: pengetatan tenant diterapkan ke `LinkClassSubject` tetapi tidak ke pasangan `UnlinkClassSubject`-nya — fix asimetris. Fix: `internal/api/handlers/mapping_handler.go` `UnlinkClassSubject`.
- **1.7** → dua akar terpisah dalam satu kueri: (a) join `hasil_tes` hanya pada `peserta_id` sehingga kardinalitas 1:N meledak menjadi N baris; (b) `session_id` diterapkan ke subquery `cek_login` tetapi tidak ke join `hasil_tes` sehingga skor bisa datang dari ujian lain. Fix: `internal/api/handlers/supervisor.go` `fetchRoomStatus` (otomatis ikut memperbaiki `supervisor_sse.go` karena berbagi fungsi).
- **1.18** → akar: `deleted_at IS NULL` hilang dan `RowsAffected` tidak diperiksa. Fix: `internal/api/handlers/kelas.go` `DeleteClass`, `internal/api/handlers/ruang.go` `DeleteRoom`.

### 6. Nilai kembalian error dibuang atau tertimpa

Idiom `v, _ := f()` dan `rows.Scan(...)` tanpa pemeriksaan tersebar di lapisan handler. Kasus paling merusak adalah `GetAvailableMapels`: ketika kueri gagal, `rows` bernilai `nil`, dan cabang `if rows == nil` yang **dimaksudkan** sebagai fallback "semua mapel" justru menimpa `err` — kegagalan berubah menjadi kebocoran data lintas kelas.

- **1.15** → akar: `rows.Scan` dan `rows.Err()` diabaikan. Fix (sembilan lokasi): `student_exam.go` `GetAvailableMapels`, `ispring.go` `GetEducationalAnalysis`, `mapping_handler.go` `GetClassSubjects`, `student.go` `GetStudents`, `ruang.go` `GetRooms`, `essay_grading.go` `GetEssayAnswers`, `item_analysis.go` `GetItemAnalysis`, `csv_utility.go` `ExportResultsCSV` dan `ExportEssayResults` (ketiga cabang csv/xlsx/pdf).
- **1.16** → akar: scan `NULL` ke `float64` non-nullable lalu `continue`. Fix: `internal/api/handlers/csv_utility.go` `ExportResultsCSV`.
- **1.17** → akar: sentinel `nil` untuk "tidak ada mapping" berbenturan dengan `nil` untuk "kueri gagal". Fix: `internal/api/handlers/student_exam.go` `GetAvailableMapels`.
- **1.21** → akar: error `HashPassword` diabaikan + validasi field wajib dan keunikan tidak ada. Fix: `internal/api/handlers/ruang.go` `CreateRoom`.
- **1.11** → akar: `SUM(CASE ...)` mengembalikan `NULL` pada tabel kosong, tanpa `COALESCE`. Fix: `internal/submission/queue.go` `SQLiteQueue.GetStats`.

### 7. Dua sumber kebenaran yang tidak disinkronkan

Sebuah keputusan dievaluasi di dua tempat dengan aturan berbeda; salah satunya lebih ketat sehingga menang secara tak sengaja.

- **1.12** → akar: guard token kosong hanya ada di cabang fallback legacy, tidak di jalur utama; diperburuk oleh `migrateTenant` yang menyalin `settings.token` kosong apa adanya. Fix: `internal/api/handlers/exam.go` `StudentLogin` (guard di depan) + `internal/api/handlers/student_session_handler.go` `resolveSessionForToken`; pertimbangkan juga menolak token kosong di `internal/service/legacy_migration.go`.
- **1.13** → akar: guard eksplisit tidak ada, keamanan bergantung pada asumsi data. Fix: `internal/api/handlers/ispring.go` `ISpringWebhook`.
- **1.14** → akar: tiga nilai (`"true"`/`"false"`/`"auto"`) dipetakan ke boolean dengan satu perbandingan `== "true"`, sehingga `"auto"` runtuh menjadi `false`; deteksi otomatis yang dijanjikan komentar `internal/config/config.go` tidak pernah diimplementasikan. Fix: `cmd/server/main.go` + `internal/api/handlers` (`SetContentCookieSecure`/`setContentCookie`).
- **1.19** → akar: otorisasi dievaluasi di middleware **dan** di handler dengan daftar role berbeda. Fix: hapus pemeriksaan role di handler pada sembilan handler terdampak; `middleware.RequireRoles` menjadi satu-satunya sumber kebenaran.
- **1.22** → akar: `UPDATE` lalu `SELECT` sebagai dua statement terpisah, bukan satu operasi atomic. Fix: `internal/repository/cek_login_repo.go` `IncrementInfraction`.
- **1.24** → akar: konvensi interval berbeda antara `enterable` (inklusif `[mulai, selesai]`) dan `windowsOverlap` (setengah terbuka `[start, end)`). Fix: `internal/service/scheduling_service.go`.
- **1.20** → akar: `strings.ToLower` diterapkan ke literal suffix, bukan ke nama berkas. Fix: `internal/api/handlers/soal_package_handler.go` `UploadSoalPackage`.

## Correctness Properties

Property 1: Bug Condition - Seluruh 25 Defect Berperilaku Benar Setelah Perbaikan

_For any_ input di mana kondisi bug berlaku (`isBugCondition` mengembalikan `true`), kode yang sudah diperbaiki SHALL menghasilkan perilaku yang didefinisikan oleh klausa Expected Behavior yang berpasangan indeks dengan sub-kondisi yang terpenuhi. Secara operasional: koneksi database SHALL benar-benar aktif dengan `journal_mode=wal`, `foreign_keys=1`, dan `busy_timeout=5000` atau gagal cepat dengan error yang jelas; jalur tulis `peserta` SHALL menyimpan `NULL` alih-alih sentinel `0`; job yang belum jatuh tempo SHALL dilewati oleh pemilih kandidat sehingga submission baru tetap terproses; kegagalan satu job SHALL terisolasi pada job itu saja sementara job valid lain dalam batch yang sama tetap tercommit ke `done/`; setiap operasi lintas tenant SHALL ditolak tanpa mengubah data; `fetchRoomStatus` SHALL mengembalikan tepat satu baris per peserta dengan skor yang berasal dari sesi yang diminta; setiap error `rows.Scan`/`rows.Err()`/`HashPassword` SHALL dilaporkan alih-alih ditelan; setiap input kosong pada jalur token SHALL ditolak sebelum kueri dijalankan; dan setiap keputusan otorisasi, cookie, serta interval SHALL dievaluasi dari satu sumber kebenaran.

**Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9, 2.10, 2.11, 2.12, 2.13, 2.14, 2.15, 2.16, 2.17, 2.18, 2.19, 2.20, 2.21, 2.22, 2.23, 2.24, 2.25**

Property 2: Preservation - Perilaku Non-Bug Tidak Berubah

_For any_ input di mana kondisi bug TIDAK berlaku (`isBugCondition` mengembalikan `false`), kode yang sudah diperbaiki SHALL menghasilkan hasil yang sama dengan kode sebelum perbaikan, mempertahankan: keberhasilan seluruh migrasi dan seluruh test suite pada data yang konsisten secara referensial; penyimpanan relasi `kelas_id`/`ruang_id` yang valid apa adanya; bentuk deployment single binary offline-first di LAN pada port yang sama; pemrosesan job sukses tanpa penundaan tambahan; **atomisitas satu transaksi untuk batch yang seluruhnya valid** beserta target throughput dan latency handler pada burst 200 dan 500 siswa; perpindahan job yang gagal permanen ke `failed/`; keberhasilan operasi attach/link/unlink pada data milik tenant sendiri; nilai `skor`/`status`/`waktu_selesai` untuk peserta dengan tepat satu `hasil_tes` beserta semantik LEFT JOIN bagi peserta tanpa hasil; penghentian worker pada `Stop` pertama; hasil login untuk token benar maupun salah; respons 200 dan enqueue untuk webhook bertoken valid; penghormatan nilai `ContentCookieSecure` eksplisit dan kemampuan menetapkan cookie di LAN tanpa TLS; payload, urutan, dan format identik pada semua endpoint list dan export termasuk header dan kolom CSV/XLSX/PDF; scope mapel per kelas; izin role `admin` dan penolakan 403 role non-admin; soft delete pada `id` valid; penamaan paket `.zip` huruf kecil; pembuatan ruang dengan data valid beserta verifikasi hash saat login pengawas; kebenaran counter dan lock pada pelanggaran tunggal; serta keputusan enterable dan overlap pada waktu non-batas.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14, 3.15, 3.16, 3.17, 3.18, 3.19, 3.20**

## Fix Implementation

### Urutan Pengerjaan (Gating)

Pekerjaan dibagi menjadi empat gerbang. **Gerbang tidak boleh dilewati** karena Gate 0 adalah prasyarat data untuk Gate 1, dan Gate 1 mengubah semantik runtime yang mendasari Gate 2–3.

| Gate | Klausa | Sifat | Kriteria keluar |
|------|--------|-------|-----------------|
| **Gate 0** — Prasyarat FK | 2.2 | Normalisasi penulis + migrasi pembersih | `PRAGMA foreign_key_check` bersih pada DB hasil migrasi; `go test ./...` hijau |
| **Gate 1** — Aktifkan pragma | 2.1 | DSN + verifikasi | Ketiga pragma terverifikasi aktif; seluruh suite hijau dengan FK menyala |
| **Gate 2** — Integritas queue | 2.3, 2.4 | Perubahan perilaku queue | Backoff berlaku; isolasi per-job terbukti; 3.5 terjaga |
| **Gate 3** — Isolasi tenant | 2.5, 2.6, 2.7 | Pengetatan predikat | Operasi lintas tenant ditolak; operasi tenant sendiri tidak berubah |
| **Gate 4** — P1/P2 cleanup | 2.8–2.25 | Perbaikan lokal independen | Masing-masing punya test yang gagal pada F dan lulus pada F' |

Gate 0–3 (klausa P0, 1.1–1.7) harus **landing dan terverifikasi lengkap** sebelum Gate 4 dimulai. Alasannya bukan administratif: Gate 1 mengubah `journal_mode` dan penegakan FK secara global, sehingga setiap kegagalan test setelah itu jauh lebih mudah didiagnosis bila tidak bercampur dengan 18 perubahan lain. Di dalam Gate 4, setiap klausa independen dan boleh dikerjakan dalam urutan apa pun.

### Changes Required

Dengan asumsi analisis akar di atas benar:

---

#### Gate 0 — Klausa 2.2: Normalisasi sentinel `0` → `NULL`

**File**: `internal/api/handlers/student.go` (`CreateStudent`), `cmd/seed/main.go`, `internal/api/handlers/csv_utility.go` (`ImportStudentsCSV`), `internal/service/legacy_migration.go`, plus satu migrasi baru di `internal/db/migrations/`.

**Keputusan desain — pilih ketiganya, bukan salah satu.** Pertanyaan "enable FK unconditionally, data-repair migration first, atau both" dijawab: **both, plus diagnostik**. Alasannya:

1. Hanya menormalkan penulis tanpa migrasi pembersih akan meninggalkan baris sentinel-`0` yang sudah ada di `data/cbt_aether.db` produksi. Baris itu tidak akan menghalangi `Connect` (SQLite tidak memvalidasi data eksisting saat FK dinyalakan) tetapi akan menggagalkan setiap `UPDATE` berikutnya pada baris tersebut, dan `PRAGMA foreign_key_check` akan melaporkannya selamanya.
2. Hanya membersihkan data tanpa menormalkan penulis akan membuat masalah kembali pada INSERT berikutnya — kali ini sebagai kegagalan keras di produksi.
3. Mengaktifkan FK secara bersyarat (mis. hanya bila data bersih) adalah opsi terburuk: menghasilkan dua mode runtime berbeda yang keduanya harus diuji, dan menyembunyikan masalah data alih-alih memaksa penyelesaiannya.

**Perubahan spesifik**:

1. **Normalisasi di lapisan tulis.** Setiap jalur yang menulis `peserta.kelas_id`/`peserta.ruang_id` memetakan nilai absen atau `0` menjadi `NULL` sebelum INSERT/UPDATE. Gunakan `sql.NullInt64` (atau `*int64`) pada struct request/parameter agar "tidak diberikan" dan "diberikan sebagai 0" tidak lagi bertabrakan pada representasi yang sama. Empat jalur harus dicakup: handler, seed, import CSV, migrasi legacy.
2. **Migrasi pembersih data.** Migrasi baru menjalankan `UPDATE peserta SET kelas_id = NULL WHERE kelas_id = 0` dan padanannya untuk `ruang_id`. Migrasi ini harus berjalan **sebelum** Gate 1 aktif; karena `migrate.go` menjalankan migrasi secara berurutan pada koneksi yang sama, dan Gate 1 mengaktifkan FK pada level koneksi, migrasi ini akan berjalan dengan FK sudah menyala — `UPDATE ... SET NULL` aman terhadap FK karena `NULL` selalu memenuhi constraint referensial.
3. **Diagnostik startup.** Tambahkan `PRAGMA foreign_key_check` sebagai langkah diagnostik pasca-migrasi yang **melaporkan** (log peringatan dengan daftar tabel/rowid yang melanggar) tanpa menggagalkan startup. Rasionalnya: menggagalkan startup pada pelanggaran data akan membuat sekolah tidak bisa menjalankan ujian, yang lebih buruk daripada berjalan dengan peringatan; sementara diam sepenuhnya akan mengulangi kesalahan 1.1. Ini melengkapi verifikasi fail-fast di 2.1, yang menyasar **konfigurasi** (deterministik, selalu bisa dibenarkan oleh kode) bukan **data** (bergantung pada instalasi).
4. **Audit** `cmd/seed/main.go` dan `ImportStudentsCSV` untuk penulis sentinel lain yang belum teridentifikasi, dan untuk INSERT yang mengandalkan `tenant_id` atau FK lain yang kini hidup (lihat daftar blast radius di atas).

---

#### Gate 1 — Klausa 2.1: DSN pragma + verifikasi pasca-koneksi

**File**: `internal/db/sqlite.go`

**Function**: `Connect`

**Perubahan spesifik**:

1. **Ganti bentuk DSN** menjadi `file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)`. Prefix `file:` wajib agar `modernc.org/sqlite` memproses query string. `databasePath` harus disisipkan dengan aman — jika path dapat mengandung karakter khusus URI (`?`, `#`), lakukan escaping; probe menunjukkan nama file tetap di-parse benar, tetapi ini tidak boleh bergantung pada keberuntungan.
2. **Verifikasi pasca-koneksi di dalam `Connect`.** Keputusan desain: verifikasi **hidup di dalam `Connect`**, bukan di `cmd/server/main.go`. Alasannya, `Connect` dipakai oleh lima pemanggil (`cmd/server`, `cmd/seed`, `cmd/createadmin`, `cmd/migratepasswords`, `internal/testutil`); menempatkan verifikasi di pemanggil berarti menuliskannya lima kali dan membuat `internal/testutil` — satu-satunya pemanggil yang paling sering dieksekusi — menjadi jalur yang tidak terverifikasi. Menempatkannya di `Connect` juga membuat test manapun yang membuka DB otomatis menjadi test regresi untuk 2.1.
3. **Isi verifikasi**: setelah `DB.Ping()` sukses, jalankan `PRAGMA journal_mode`, `PRAGMA foreign_keys`, `PRAGMA busy_timeout` dan bandingkan dengan nilai yang diharapkan (`wal` case-insensitive, `1`, `5000`). Bila ada yang tidak cocok, kembalikan error yang menyebutkan **pragma mana**, **nilai yang diharapkan**, dan **nilai aktual** — sehingga kesalahan sintaks DSN di masa depan tidak lagi bisa senyap.
4. **Catat batasan pool.** `PRAGMA busy_timeout` dan `foreign_keys` bersifat **per-koneksi**. Karena `sql.DB` adalah pool (`MaxOpenConns=25`) yang membuka koneksi secara lazy, memverifikasi satu koneksi tidak otomatis menjamin koneksi ke-2..25. DSN pragma diterapkan oleh driver ke **setiap** koneksi baru, jadi jaminannya berasal dari DSN; verifikasi berperan sebagai deteksi kesalahan sintaks, bukan sebagai jaminan per-koneksi. Untuk memperkuat, verifikasi dapat dijalankan pada beberapa koneksi paralel — namun ini opsional dan harus ditimbang terhadap kompleksitasnya. `journal_mode=WAL` bersifat persisten pada file database sehingga tidak terpengaruh isu ini.
5. **Perbarui komentar doc** `Connect` agar tidak lagi mengklaim perilaku yang tidak diverifikasi.

**Keputusan tentang retry loop SQLITE_BUSY yang sudah ada.** `internal/db/migrate.go` dan `internal/repository/exam_session_repo.go` memiliki retry loop SQLITE_BUSY hand-rolled yang ada **hanya** karena `busy_timeout` tidak pernah berlaku. Keputusan: **keduanya dipertahankan sebagai defense in depth pada Gate 1, tidak disederhanakan.** Alasannya: (a) menghapusnya di gerbang yang sama dengan mengaktifkan pragma akan mencampur dua perubahan berisiko pada satu titik verifikasi; (b) `busy_timeout` tidak melindungi terhadap `SQLITE_BUSY` yang berasal dari upgrade deadlock antar transaksi write, sehingga retry loop tetap punya nilai nyata; (c) biayanya nol pada jalur sukses. Penyederhanaan keduanya adalah kandidat pekerjaan lanjutan **setelah** Gate 1 terbukti stabil, dan bukan bagian dari lingkup spec ini.

---

#### Gate 2 — Klausa 2.3: Backoff yang benar-benar berlaku

**File**: `internal/submission/fsqueue.go`

**Functions**: `Dequeue`, `MarkFailed`, plus test terkait

**Ruang solusi dan keputusan.** Ada dua pendekatan, dan keduanya berbenturan dengan sesuatu yang bernilai:

- **Opsi A — sandikan waktu jatuh tempo ke nama file.** Pemilihan kandidat tetap murni berbasis nama (murah, tanpa I/O tambahan), dan FIFO tetap terjaga karena `unix_nano` tetap menjadi sort key. Biayanya: melanggar niat eksplisit "filename tetap sama agar admin dapat mengkorelasi file antar direktori", dan mematahkan regex di `fsqueue_property_test.go`.
- **Opsi B — baca `EnqueuedAt` saat pemilihan kandidat.** Nama file tidak berubah sehingga korelasi admin dan regex test terjaga. Biayanya: `Dequeue` harus membaca dan mem-parse file **sebelum** meng-klaimnya, membalik urutan "rename-then-read" yang saat ini menjadi dasar klaim atomic antar worker.
- **Opsi C — sidecar due-time di `retryDir`.** Ada preseden: `retry/<name>.attempts` sudah dipakai untuk counter kegagalan parse. Biayanya: satu file lagi per job untuk dikelola, dibersihkan, dan dipulihkan saat crash; dan `Dequeue` tetap harus melakukan `Stat`/`ReadFile` sidecar sebelum klaim.

**Keputusan: Opsi A, dengan mitigasi korelasi.** Sandikan waktu jatuh tempo ke nama file (mis. sebagai segmen tambahan yang tidak mengganggu posisi sort `unix_nano`), sehingga `Dequeue` dapat memutuskan "sudah jatuh tempo atau belum" **tanpa** membaca isi file dan tanpa mengubah urutan rename-then-read yang menjadi dasar klaim atomic. Ini satu-satunya opsi yang tidak menyentuh properti konkurensi inti queue — properti yang paling mahal bila salah.

Niat korelasi admin dimitigasi, bukan dibuang: segmen `unix_nano`, `tenant_id`, dan `no_id` **tetap** menjadi prefix nama file, sehingga sebuah job masih dapat dilacak antar direktori dengan pencocokan prefix. Yang hilang hanya kesamaan nama byte-per-byte. Runbook `docs/runbooks/queue-and-litestream.md` harus diperbarui agar instruksi korelasi untuk admin mencerminkan pencocokan prefix ini — tanpa itu, memperbaiki bug ini akan merusak prosedur operasional yang terdokumentasi.

**Perubahan spesifik**:

1. `MarkFailed` menghitung waktu jatuh tempo seperti sekarang (`min(2^(n-1), 30)` detik) lalu menuliskannya ke **nama file** tujuan, bukan hanya ke `job.EnqueuedAt`. `EnqueuedAt` tetap diisi agar payload JSON tetap informatif untuk admin dan agar tidak ada konsumen lain yang pecah.
2. `Dequeue` mem-parse waktu jatuh tempo dari nama file dan **melewati** entri yang belum jatuh tempo, melanjutkan ke kandidat berikutnya alih-alih berhenti. Ini krusial untuk 2.3: job yang belum jatuh tempo harus dilewati agar submission baru tetap terproses, bukan memblokir queue.
3. `RecoverStartup` harus tetap mengenali kedua bentuk nama (dengan dan tanpa segmen due-time) karena `pending/` produksi dapat berisi file yang ditulis oleh versi sebelumnya. Job tanpa segmen due-time diperlakukan sebagai sudah jatuh tempo.
4. **Perbarui `internal/submission/fsqueue_property_test.go`**: regex `^\d{19}-\d+-[A-Za-z0-9_-]+-[0-9a-f]{8}\.json$` dan assertion backoff berbasis `EnqueuedAt` harus disesuaikan dengan skema baru. Ini perubahan yang disengaja dan wajib, bukan regresi — lihat catatan di Preservation Requirements.
5. Klausa 3.4 dijaga secara struktural: segmen due-time hanya ditambahkan oleh `MarkFailed`, sehingga job yang belum pernah gagal tidak pernah memiliki penundaan.

---

#### Gate 2 — Klausa 2.4: Isolasi kegagalan per job

**File**: `internal/submission/processor.go` (`ProcessBatch`), `internal/submission/worker.go` (`processBatchSafe`)

**Ruang solusi dan keputusan.** Dua kandidat:

- **Opsi A — savepoint per job.** `ProcessBatch` membungkus setiap `processOneInTx` dalam `SAVEPOINT`, me-rollback hanya savepoint job yang gagal, lalu commit sisanya, dan mengembalikan **outcome per job** kepada worker.
- **Opsi B — fallback job-by-job.** Worker mencoba batch; bila gagal, ia memproses ulang job satu per satu dalam transaksi masing-masing untuk mengidentifikasi mana yang benar-benar gagal.

**Keputusan: Opsi A (savepoint per job) sebagai jalur utama.** Alasannya:

1. **Klausa 3.5 dipenuhi secara langsung.** Batch yang seluruhnya valid tetap satu `BeginTx`/`Commit` tunggal dengan savepoint yang semuanya sukses — satu transaksi, satu commit, satu fsync WAL. Tidak ada perubahan pada jalur normal, sehingga target throughput dan latency handler pada burst 200/500 tidak tersentuh.
2. **Opsi B menggandakan kerja pada jalur kegagalan.** Batch gagal harus diproses dua kali (sekali sebagai batch, sekali per job), dan `processOneInTx` harus idempotent terhadap efek samping partial dari percobaan pertama — asumsi yang mahal untuk diverifikasi.
3. **Opsi B tidak dapat membedakan** kegagalan per-job dari kegagalan tingkat transaksi (mis. `SQLITE_BUSY` pada commit). Savepoint membuat perbedaan itu eksplisit di tempat kegagalan terjadi.

**Perubahan spesifik**:

1. **Kontrak baru antar lapis.** `ProcessBatch` tidak lagi mengembalikan `error` tunggal, melainkan hasil per job (mis. `map[int64]error` atau slice sejajar dengan `jobs`) di samping error tingkat transaksi. Ini adalah inti fix: tanpa kontrak per-job, `processBatchSafe` **secara struktural tidak dapat** membedakan job yang gagal dari job yang tidak.
2. **`ProcessBatch`**: `BeginTx` → untuk setiap job, `SAVEPOINT` → `processOneInTx` → `RELEASE` bila sukses atau `ROLLBACK TO SAVEPOINT` bila gagal (catat errornya, lanjut ke job berikutnya) → `Commit` di akhir. Bila `Commit` sendiri gagal, itu kegagalan tingkat transaksi yang berlaku untuk seluruh batch dan semua job harus di-`MarkFailed` — perilaku lama tetap benar untuk kasus ini.
3. **`processBatchSafe`**: `MarkCompleted` untuk job yang tidak punya error, `MarkFailed` hanya untuk job yang punya error. Jalur panic recovery tetap men-`MarkFailed` seluruh batch, karena panic membuat state transaksi tidak dapat dipercaya.
4. **Kompatibilitas pemanggil lain.** `NewWorkerSingle` dan fixture legacy yang memakai signature `func(ctx, *SubmissionJob) error` harus tetap bekerja; adaptor batch-of-1 yang ada dipetakan ke kontrak baru.
5. Klausa 3.6 dijaga: job yang memang gagal permanen tetap menempuh `MarkFailed` berulang sampai `maxRetries` lalu pindah ke `failed/`.

---

#### Gate 3 — Klausa 2.5: Guard tenancy sesi

**File**: `internal/repository/exam_session_repo.go`

**Functions**: `AttachClasses`, `AttachRooms`

**Perubahan spesifik**: tambahkan verifikasi kepemilikan `sessionID` oleh `tenantID` **sebelum penulisan apa pun**, di samping verifikasi kelas/ruang yang sudah ada. File ini sudah memiliki helper `validateExam` dan `tenantHasRow`; guard baru ditempatkan berdampingan dengan keduanya dan mengikuti bentuk yang sama, sehingga tidak ada pola baru yang diperkenalkan. Kegagalan verifikasi mengembalikan `ErrNotFound` dari `internal/repository/errors.go` (bukan error ad-hoc), dan handler `attachSession` di `internal/api/handlers/exam_session_handler.go` memetakannya ke 404 melalui `utils.ErrorResponse`. Memilih 404 alih-alih 403 disengaja: memberi tahu pemanggil bahwa sebuah sesi milik tenant lain *ada* adalah kebocoran informasi lintas tenant, sekecil apa pun.

---

#### Gate 3 — Klausa 2.6: Filter tenant pada unlink

**File**: `internal/api/handlers/mapping_handler.go`

**Function**: `UnlinkClassSubject`

**Perubahan spesifik**: validasi `kelas_id` dan `mapel_id` sebagai integer; tambahkan `tenant_id = ?` pada klausa WHERE `DELETE`, setara dengan pengetatan yang sudah diterapkan di `LinkClassSubject` pasangannya; periksa `RowsAffected` dan kembalikan 404 bila `0`. Gunakan `ErrNotFound` / pola `requireAffected()` yang sudah ada di `internal/repository/cek_login_repo.go` agar konsisten dengan lapisan repository, dan `utils.ErrorResponse` untuk respons.

---

#### Gate 3 — Klausa 2.7: Kardinalitas dan scoping room-status

**File**: `internal/api/handlers/supervisor.go`

**Function**: `fetchRoomStatus` (memperbaiki `GET /api/supervisor/room-status` dan stream SSE di `supervisor_sse.go` sekaligus, karena keduanya memakai fungsi ini)

**Perubahan spesifik**: dua akar harus diperbaiki bersamaan, memperbaiki satu saja meninggalkan bug yang lain.

1. **Kardinalitas.** Pastikan join `hasil_tes` menghasilkan paling banyak satu baris per peserta, sehingga keluaran kembali sesuai kontrak yang dinyatakan komentar fungsinya sendiri.
2. **Scoping sesi.** Ketika `session_id` diberikan, batasi join `hasil_tes` pada sesi/mapel yang sama dengan subquery `cek_login`. Migrasi `031` menambahkan `hasil_tes.exam_session_id` → `exam_session(id)`, yang menyediakan kolom untuk melakukan pembatasan ini secara langsung; perhatikan bahwa kolom ini ditambahkan via `ALTER TABLE ADD COLUMN` sehingga baris historis dapat bernilai `NULL` dan penanganan baris tersebut harus eksplisit, bukan implisit.
3. Semantik LEFT JOIN harus dipertahankan: peserta tanpa `hasil_tes` tetap muncul sebagai satu baris dengan nilai kosong (klausa 3.8).

---

#### Gate 4 — Klausa 2.8–2.25 (P1/P2)

Setiap item di bawah bersifat lokal dan independen.

1. **2.8 — `Stop` idempotent.** `internal/submission/worker.go` `Stop`: bungkus `close(w.stopChan)` dengan `sync.Once`. Pemanggilan kedua dan selanjutnya kembali tanpa panic; pemanggilan pertama tetap menghentikan worker seperti sekarang (3.9).
2. **2.9 — Lifecycle queue.** `internal/submission/fsqueue.go`: tambahkan `Close`/`Shutdown` yang menghentikan `runEnqueueWriter` dan melepaskan goroutine-nya, idempotent (`sync.Once`) dengan alasan yang sama seperti 2.8. `Enqueue` setelah `Close` harus mengembalikan error yang jelas, bukan panic pada channel tertutup atau blok selamanya. Perbarui pemanggil: `cmd/server/main.go` (via graceful shutdown 2.23) dan test yang membuat instance queue.
3. **2.10 — Dead letter konsisten.** `MarkFailed`: ketika `RetryCount >= maxRetries`, tulis `<name>.error.txt` berisi `LastError` dan konteks retry, sejajar dengan jalur corrupt-file di `Dequeue` yang sudah melakukannya. Gunakan konvensi penamaan yang sama (`strings.TrimSuffix(name, ".json") + ".error.txt"`) agar `docs/runbooks/queue-and-litestream.md` berlaku untuk kedua jalur.
4. **2.11 — `GetStats` pada tabel kosong.** `internal/submission/queue.go`: bungkus setiap agregat dengan `COALESCE(..., 0)`.
5. **2.12 — Guard token login.** `internal/api/handlers/exam.go` `StudentLogin`: tolak token kosong/whitespace-only sebelum resolusi sesi. `internal/api/handlers/student_session_handler.go` `resolveSessionForToken`: jangan pernah memperlakukan `exam_session` bertoken `''` sebagai enterable. Pertimbangkan juga menolak/menormalkan token kosong di `internal/service/legacy_migration.go` `migrateTenant` agar sumber data yang cacat tidak terus diproduksi — perbaikan di hilir saja meninggalkan baris `''` di database.
6. **2.13 — Guard token webhook.** `internal/api/handlers/ispring.go` `ISpringWebhook`: guard eksplisit yang mengembalikan 403 `invalid attempt token` sebelum kueri `cek_login` dijalankan.
7. **2.14 — Deteksi `Secure` untuk `"auto"`.** `cmd/server/main.go` dan `internal/api/handlers` (`SetContentCookieSecure`, `setContentCookie`): representasikan tiga nilai (`"true"`, `"false"`, `"auto"`) sebagai tri-state, bukan boolean, sehingga `"auto"` tidak lagi runtuh menjadi `false`. Untuk `"auto"`, tentukan atribut `Secure` dari konteks request: skema request dan header proxy tepercaya seperti `X-Forwarded-Proto`. **Header proxy hanya boleh dipercaya bila proxy-nya tepercaya** — Fiber menyediakan konfigurasi trusted proxy untuk ini, dan mempercayai `X-Forwarded-Proto` tanpa syarat akan memberi klien kendali atas atribut keamanan cookie. Klausa 3.12 adalah batas keras: pada HTTP polos di LAN sekolah, `"auto"` harus menghasilkan `Secure=false` sehingga cookie tetap ter-set dan siswa tetap bisa ujian. Perbarui juga komentar di `internal/config/config.go` agar cocok dengan perilaku sesungguhnya.
8. **2.15 — Periksa `rows.Scan` dan `rows.Err()`.** Sembilan lokasi: `student_exam.go` `GetAvailableMapels`, `ispring.go` `GetEducationalAnalysis`, `mapping_handler.go` `GetClassSubjects`, `student.go` `GetStudents`, `ruang.go` `GetRooms`, `essay_grading.go` `GetEssayAnswers`, `item_analysis.go` `GetItemAnalysis`, `csv_utility.go` `ExportResultsCSV` dan `ExportEssayResults` (ketiga cabang csv/xlsx/pdf). Endpoint list mengembalikan 500 via `utils.ErrorResponse`. Untuk ekspor, keputusannya berbeda: sebuah ekspor yang sebagian sudah ter-stream tidak selalu bisa dibatalkan, jadi kegagalan harus dicatat di log **dan** ditandai secara eksplisit sehingga data hilang tidak tersembunyi — sebuah file ekspor yang tampak lengkap padahal tidak adalah kegagalan yang lebih buruk daripada error yang jelas.
9. **2.16 — `NULL` pada ekspor.** `csv_utility.go` `ExportResultsCSV`: scan `skor`/`skor_maks` ke tipe nullable (`sql.NullFloat64`) dan tetap tulis baris siswa dengan penanda nilai kosong. Hapus `continue` yang membuang baris. Format kolom untuk baris non-`NULL` tidak boleh berubah (3.13).
10. **2.17 — Jangan fallback karena error.** `student_exam.go` `GetAvailableMapels`: pisahkan "kueri gagal" dari "tidak ada mapping". Kembalikan error kepada pemanggil pada kegagalan kueri; cabang fallback "semua mapel" hanya boleh tercapai lewat kondisi yang memang dimaksudkan, bukan lewat `rows == nil` akibat error.
11. **2.18 — Delete yang benar.** `kelas.go` `DeleteClass` dan `ruang.go` `DeleteRoom`: pakai `c.ParamsInt`, sertakan `deleted_at IS NULL`, kembalikan 404 saat `RowsAffected == 0`. Cetak biru sudah ada di `student.go` `DeleteStudent` — samakan strukturnya, dan pakai `ErrNotFound`/`requireAffected()` yang sudah tersedia.
12. **2.19 — Satu sumber kebenaran otorisasi.** Hapus pemeriksaan `role != "admin"` di sembilan handler (`UpdateSettings`, `ImportStudentsCSV`, `GradeEssayAnswer`, `LinkClassSubject`, `UnlinkClassSubject`, `DeleteStudent`, `DeleteClass`, `DeleteMapel`, `DeleteRoom`) dan jadikan `middleware.RequireRoles` di `cmd/server/main.go` satu-satunya penentu. Middleware dipilih sebagai sumber kebenaran karena ia sudah menjadi mekanisme otorisasi untuk seluruh route dan letaknya bersebelahan dengan definisi route, sehingga daftar role terbaca di tempat yang sama dengan daftar endpoint. **Sebelum menghapus**, verifikasi untuk setiap handler bahwa route-nya memang dibungkus `RequireRoles` dengan daftar role yang diinginkan — menghapus pemeriksaan handler pada route yang tidak terlindungi middleware akan mengubah bug otorisasi menjadi lubang otorisasi. Klausa 3.15 harus tetap terjaga: `admin` diizinkan, non-admin ditolak 403.
13. **2.20 — TrimSuffix case-insensitive.** `soal_package_handler.go` `UploadSoalPackage`: buang ekstensi `.zip` secara case-insensitive (mis. via `filepath.Ext` + perbandingan lowercase, atau `strings.EqualFold`), bukan dengan meng-lowercase literalnya. `PAKET.ZIP` → `PAKET`; `paket.zip` → `paket` (3.17).
14. **2.21 — `CreateRoom` yang ketat.** `ruang.go` `CreateRoom`: tangani error `utils.HashPassword` dan batalkan operasi bila hashing gagal; wajibkan `nama_ruang`, `username`, `password`; periksa keunikan `username` sebelum INSERT dan kembalikan `ErrConflict` (sudah tersedia di `internal/repository/errors.go`) → 409 bila sudah ada.
15. **2.22 — Increment atomic.** `cek_login_repo.go` `IncrementInfraction`: gabungkan increment dan pembacaan menjadi satu operasi dengan `UPDATE ... RETURNING` (didukung SQLite modern) atau bungkus keduanya dalam satu transaksi. `RETURNING` lebih disukai karena menghilangkan race tanpa menambah transaksi. Setiap pemanggil bersamaan harus menerima nilai yang berbeda sehingga ambang lock terdeteksi tepat sekali.
16. **2.23 — Graceful shutdown.** `cmd/server/main.go`: ganti `log.Fatal(app.Listen(...))` dengan pola yang menjalankan `Listen` di goroutine, menunggu `SIGINT`/`SIGTERM` via `signal.NotifyContext` atau channel, lalu memanggil `app.Shutdown()` dan membiarkan urutan pembersihan berjalan: hentikan worker (2.8), tutup queue (2.9), batalkan context, tutup DB. Job in-flight diselesaikan atau dikembalikan ke `pending/` sehingga hasil ujian tidak lagi menunggu sweep stuck-threshold 5 menit setelah restart. Perhatikan Windows: `SIGTERM` tidak dikirim seperti di Unix, sehingga `SIGINT` (Ctrl+C) adalah jalur yang benar-benar akan teruji di platform target.
17. **2.24 — Konvensi interval seragam.** `internal/service/scheduling_service.go`: samakan konvensi antara `enterable` dan `windowsOverlap`. Setengah terbuka `[mulai, selesai)` adalah pilihan yang direkomendasikan karena itulah konvensi yang sudah dipakai `windowsOverlap` dan konvensi standar untuk interval waktu; konsekuensinya `enterable` menjadi eksklusif di ujung akhir. Perubahan ini **mengubah perilaku pada instan `selesai`** dan harus ditulis eksplisit di test agar keputusan itu terdokumentasi, bukan tersembunyi. Klausa 3.20 membatasi: waktu non-batas tidak boleh berubah keputusannya.
18. **2.25 — Baca melalui `tx`.** `processor.go` `processOneInTx`: ganti `p.db.QueryRowContext(...)` di cabang diagnostik menjadi `tx.QueryRowContext(...)`, konsisten dengan pembacaan lain di fungsi yang sama dan bebas dari risiko self-deadlock setelah Gate 1 mengubah `journal_mode`.

## Testing Strategy

### Validation Approach

Strategi ini mengikuti dua fase: pertama, munculkan counterexample yang **membuktikan** bug pada kode yang belum diperbaiki; kedua, verifikasi bahwa fix bekerja dan perilaku eksisting tidak berubah.

Fase pertama bukan formalitas. Baseline saat ini `go vet ./...` exit 0 dan `go test ./...` hijau di seluruh paket, artinya **tidak satu pun dari 25 bug ini tertangkap oleh test yang ada**. Sebuah test baru yang langsung ditulis di atas kode yang sudah diperbaiki tidak membuktikan apa pun — ia bisa saja lulus karena menguji hal yang salah. Setiap klausa karena itu harus punya test yang **gagal pada F** sebelum fix ditulis.

**Anggaran waktu verifikasi.** `go test ./...` memakan ~7 menit total, dengan `internal/api/handlers` sendiri ~290 detik. Karena 25 klausa dikerjakan dalam empat gerbang, jalankan suite lengkap **di batas gerbang**, dan jalankan paket terdampak saja (`go test ./internal/submission/...`, `./internal/repository/...`, dan seterusnya) selama iterasi di dalam gerbang. Platform target adalah Windows/PowerShell; `syncDir` di `internal/submission/fsync.go` sudah memperhitungkan bahwa fsync direktori adalah no-op di sana, jadi test durabilitas tidak boleh mengasumsikan sebaliknya.

**Perkakas yang sudah tersedia** (pakai ini, jangan buat baru): `internal/testutil/db.go` dan `seeds.go` menyediakan DB per-test yang sudah termigrasi beserta seed; `pgregory.net/rapid` sudah menjadi dependensi; property test yang ada berada di `internal/submission/*_property_test.go` dan mengikuti konvensi anotasi `// Feature: <spec>, Property N` — property test spec ini mengikuti konvensi yang sama dengan `// Feature: codebase-bug-sweep, Property 1` dan `Property 2`.

### Exploratory Bug Condition Checking

**Goal**: Munculkan counterexample yang mendemonstrasikan bug **SEBELUM** fix diimplementasikan. Konfirmasi atau bantah analisis akar. Bila terbantah, akar harus dihipotesiskan ulang sebelum fix ditulis.

**Test Plan**: Untuk setiap klausa, tulis test yang mereproduksi kondisi bug pada kode yang belum diperbaiki dan meng-assert perilaku yang **benar** (bukan perilaku aktual). Jalankan pada F untuk mengamati kegagalan dan memahami akarnya. Untuk klausa yang menyentuh konfigurasi driver (1.1), pembuktian sudah ada dalam bentuk probe empiris terhadap `modernc.org/sqlite` v1.50.1 dan harus dikonversi menjadi test permanen di dalam repo agar tidak terulang.

**Test Cases (Gate 0–3, prioritas P0):**

1. **Pragma benar-benar aktif** — buka DB via `db.Connect`, lalu kueri `PRAGMA journal_mode`, `foreign_keys`, `busy_timeout`. Pada F menghasilkan `delete`/`0`/`0` (akan gagal pada kode belum diperbaiki). Ini konversi probe empiris menjadi test permanen.
2. **Sentinel `0` tertolak oleh FK** — dengan `foreign_keys` aktif, INSERT `peserta` dengan `kelas_id=0` harus ditolak; `CreateStudent` tanpa `kelas_id` harus menyimpan `NULL`. Pada F menyimpan `0` (akan gagal).
3. **`PRAGMA foreign_key_check` bersih** — jalankan seluruh migrasi pada DB yang berisi baris sentinel-`0`, lalu `foreign_key_check` harus kosong. Pada F melaporkan pelanggaran (akan gagal).
4. **Backoff benar-benar menunda** — `MarkFailed` sebuah job, lalu segera `Dequeue`. Job itu tidak boleh terpilih sebelum jatuh tempo, dan job baru yang di-enqueue setelahnya harus terpilih. Pada F job gagal langsung terpilih ulang (akan gagal).
5. **Job belum jatuh tempo tidak memblokir** — satu job gagal yang belum jatuh tempo di depan queue plus beberapa job baru; semua job baru harus terproses. Pada F job gagal menghabiskan `maxRetries` dalam milidetik sambil membuat yang lain kelaparan (akan gagal).
6. **Isolasi kegagalan batch** — batch 5 job dengan job ke-3 gagal permanen. 4 job harus berakhir di `done/`, 1 di jalur retry/`failed/`. Pada F kelimanya di-`MarkFailed` (akan gagal).
7. **Attach lintas tenant tertolak** — admin tenant A attach kelas miliknya ke sesi tenant B: harus not-found/forbidden **dan tidak ada baris yang tertulis**. Pada F penulisan berhasil (akan gagal).
8. **Unlink lintas tenant tertolak** — admin tenant A unlink mapping tenant B: harus 404 dan mapping tenant B tetap ada. Pada F mapping terhapus (akan gagal).
9. **Room-status satu baris per peserta** — peserta dengan 3 baris `hasil_tes`: keluaran harus 1 baris. Pada F menghasilkan 3 (akan gagal).
10. **Room-status ter-scope sesi** — peserta dengan `hasil_tes` di dua sesi berbeda, minta `session_id` sesi pertama: `skor`/`status`/`waktu_selesai` harus milik sesi pertama. Pada F dapat menampilkan skor sesi lain (akan gagal).

**Test Cases (Gate 4, P1/P2):**

11. **`Stop` ganda** — panggil `Stop()` dua kali: tidak boleh panic. Pada F `panic: close of closed channel` (akan gagal).
12. **Tidak ada goroutine bocor** — buat lalu tutup N instance queue, bandingkan jumlah goroutine. Pada F setiap instance membocorkan satu goroutine (akan gagal).
13. **Dead letter punya `.error.txt`** — habiskan `maxRetries` sebuah job: `failed/<name>.error.txt` harus ada dan memuat `LastError`. Pada F tidak ada (akan gagal).
14. **`GetStats` tabel kosong** — `SQLiteQueue.GetStats` pada `submission_queue` kosong harus mengembalikan statistik nol tanpa error. Pada F error (akan gagal).
15. **Login token kosong tertolak** — `StudentLogin` dengan `token: ""` dan `"   "`, termasuk saat ada `exam_session` bertoken `''` yang enterable (kondisi hasil `migrateTenant`): harus ditolak. Pada F dapat berhasil login (akan gagal).
16. **Webhook token kosong → 403** — `attempt_token: ""` harus 403 `invalid attempt token`. Pada F bergantung pada asumsi data (akan gagal bila ada baris `cek_login` bertoken `''`).
17. **Cookie `Secure` pada `"auto"` + proxy TLS** — request dengan `X-Forwarded-Proto: https` dari proxy tepercaya dan `ContentCookieSecure="auto"`: cookie harus punya atribut `Secure`. Pada F tidak (akan gagal). Pasangan wajibnya: HTTP polos + `"auto"` → cookie tetap ter-set tanpa `Secure` (3.12).
18. **Scan error tidak ditelan** — paksa kegagalan scan (mis. tipe kolom tidak cocok) pada endpoint list: harus 500, bukan 200 dengan hasil parsial. Pada F 200 (akan gagal).
19. **Ekspor tidak kehilangan baris** — baris `hasil_tes` dengan `skor` `NULL`: file ekspor harus tetap memuat baris siswa itu. Pada F baris hilang total (akan gagal).
20. **`GetAvailableMapels` tidak fallback karena error** — paksa kueri per-kelas gagal: harus error, bukan daftar seluruh mapel tenant. Pada F mengembalikan semua mapel (akan gagal).
21. **Delete pada baris terhapus → 404** — `DeleteClass`/`DeleteRoom` pada baris yang sudah soft-deleted dan pada `id` non-integer: harus 404/400. Pada F selalu 200 (akan gagal).
22. **`superadmin` diizinkan** — `superadmin` memanggil kesembilan endpoint admin: harus berhasil. Pada F 403 (akan gagal). Pasangan wajibnya: non-admin tetap 403 (3.15).
23. **`PAKET.ZIP` → `PAKET`** — unggah dengan ekstensi kapital: nama tersimpan tanpa ekstensi. Pada F `PAKET.ZIP` (akan gagal).
24. **`CreateRoom` menolak input tidak lengkap dan username duplikat**; error hashing membatalkan operasi. Pada F menyimpan hash kosong dan menerima input tidak lengkap (akan gagal).
25. **`IncrementInfraction` konkuren** — N goroutine bersamaan: himpunan nilai yang dikembalikan harus tidak ada duplikat dan lock terpicu tepat sekali. Pada F nilai duplikat muncul (akan gagal, mungkin flaky pada F — jalankan dengan `-race` dan iterasi cukup banyak).
26. **Graceful shutdown** — kirim sinyal ke server: worker berhenti, `processing/` kosong (job selesai atau kembali ke `pending/`), DB tertutup. Pada F semua `defer` dilewati (akan gagal).
27. **Instan batas scheduling** — sesi `[10:00,11:00]` dan `[11:00,12:00]` pada `11:00`: tidak boleh sekaligus dinyatakan tidak bertumpang tindih dan keduanya enterable. Pada F kontradiksi ini terjadi (akan gagal).

**Expected Counterexamples**:

- Pragma tidak aktif: `journal_mode=delete`, `foreign_keys=0`, `busy_timeout=0` — sudah dikonfirmasi empiris, jadi analisis akar untuk 1.1 tidak perlu dibantah.
- Job gagal di-dequeue ulang secara instan, `RetryCount` melompat ke `maxRetries` dalam milidetik.
- Sampai `batchSize-1` job valid berakhir di `failed/`.
- Penulisan lintas tenant sukses tanpa error.
- Jumlah baris room-status melebihi jumlah peserta.
- Kemungkinan penyebab yang harus dibedakan saat mengamati kegagalan: sintaks DSN yang diabaikan driver, jadwal yang disimpan di field yang tidak dibaca, granularitas outcome batch yang tidak cocok, predikat kueri yang kehilangan dimensi, dan nilai error yang dibuang.

### Fix Checking

**Goal**: Verifikasi bahwa untuk semua input di mana kondisi bug berlaku, kode yang sudah diperbaiki menghasilkan perilaku yang diharapkan.

**Pseudocode:**
```
FOR ALL input WHERE isBugCondition(input) DO
  result := fixedSystem(input)
  ASSERT expectedBehavior(result)
END FOR
```

Karena C adalah disjungsi 25 sub-kondisi, fix checking didekomposisi per sub-kondisi:

```
FOR EACH i IN 1..25 DO
  FOR ALL input WHERE C_i(input) DO
    ASSERT clause_2_i_holds(fixedSystem(input))
  END FOR
END FOR
```

Sub-kondisi dengan domain input yang luas dan terstruktur diverifikasi dengan property-based testing memakai `rapid`; sub-kondisi dengan domain sempit dan diskrit diverifikasi dengan unit test. Pembagiannya:

- **Property-based** (domain luas): C3 (waktu jatuh tempo × `RetryCount`), C4 (komposisi batch × posisi kegagalan), C7 (jumlah `hasil_tes` per peserta × ada/tidaknya `session_id`), C22 (jumlah pemanggil konkuren), C20 (kapitalisasi nama berkas).
- **Unit** (domain sempit): C1, C2, C5, C6, C8–C19, C21, C23–C25.

### Preservation Checking

**Goal**: Verifikasi bahwa untuk semua input di mana kondisi bug TIDAK berlaku, kode yang sudah diperbaiki menghasilkan hasil yang sama dengan kode sebelum perbaikan.

**Pseudocode:**
```
FOR ALL input WHERE NOT isBugCondition(input) DO
  ASSERT originalSystem(input) = fixedSystem(input)
END FOR
```

**Testing Approach**: Property-based testing direkomendasikan untuk preservation checking karena:

- Ia membangkitkan banyak kasus uji otomatis di seluruh domain input.
- Ia menangkap edge case yang mudah terlewat oleh unit test manual.
- Ia memberi jaminan kuat bahwa perilaku tidak berubah untuk **semua** input non-bug — dan inilah risiko terbesar spec ini: 25 perubahan yang tersebar di lapisan db, repository, handler, service, dan queue punya permukaan regresi yang jauh lebih luas daripada bug yang diperbaikinya.

Ada satu batasan penting: karena F dan F' tidak dapat hidup berdampingan dalam satu binary untuk perbandingan langsung, preservation checking dijalankan sebagai **perbandingan terhadap baseline yang direkam**. Amati perilaku pada F terlebih dahulu (payload respons, isi file ekspor, urutan hasil, jumlah baris), rekam sebagai fixture/golden file, lalu assert bahwa F' menghasilkan hal yang sama. Untuk klausa 3.13 pendekatan golden file ini bersifat wajib, karena "payload, urutan, serta format identik — termasuk header dan kolom CSV, XLSX, dan PDF" tidak praktis diverifikasi dengan assertion manual.

**Test Plan**: Rekam perilaku pada kode belum diperbaiki untuk input non-bug, lalu tulis test (property-based di mana domainnya luas, golden file di mana formatnya kaya) yang menangkap perilaku itu dan menjalankannya terhadap F'.

**Test Cases**:

1. **Migrasi dan suite dengan FK aktif** — amati bahwa 20+ migrasi selesai dan seluruh paket lulus pada F, lalu verifikasi keduanya tetap berlaku setelah `foreign_keys` menyala. Ini test preservation paling penting di seluruh spec: `go vet ./...` exit 0 dan `go test ./...` hijau (3.1).
2. **Relasi peserta valid** — amati respons `CreateStudent`/seed/import CSV dengan `kelas_id`/`ruang_id` valid pada F, lalu verifikasi relasi tersimpan apa adanya dan respons identik pada F' (3.2).
3. **Job sukses percobaan pertama** — amati bahwa job pindah ke `done/` tanpa penundaan pada F, lalu verifikasi tidak ada penundaan tambahan yang diperkenalkan oleh segmen due-time (3.4).
4. **Atomisitas batch seluruhnya valid** — amati satu transaksi tunggal pada F, lalu verifikasi savepoint per job tetap menghasilkan satu `BeginTx`/`Commit` (3.5).
5. **Kegagalan permanen tetap dead letter** — amati perpindahan ke `failed/` setelah `maxRetries` pada F, lalu verifikasi tetap terjadi (3.6).
6. **Operasi tenant sendiri** — amati respons sukses attach/link/unlink pada data sendiri di F, lalu verifikasi identik (3.7).
7. **Room-status kasus normal dan LEFT JOIN** — amati nilai untuk peserta dengan tepat satu `hasil_tes` dan baris kosong untuk peserta tanpa hasil pada F, lalu verifikasi keduanya tidak berubah (3.8).
8. **Login token benar dan salah** — amati status dan pesan error pada F, lalu verifikasi identik (3.10).
9. **Webhook token valid** — amati 200 + enqueue pada F, lalu verifikasi identik (3.11).
10. **Cookie eksplisit dan LAN tanpa TLS** — amati perilaku `"true"`/`"false"` dan penetapan cookie pada HTTP polos di F, lalu verifikasi keduanya tetap (3.12).
11. **Golden file list dan export** — rekam payload seluruh endpoint list serta keluaran CSV, XLSX, dan PDF pada F, lalu verifikasi F' menghasilkan byte/struktur yang sama untuk data tanpa `NULL` dan tanpa scan error (3.13).
12. **Scope mapel per kelas** — amati hasil `GetAvailableMapels` untuk siswa dengan mapping pada F, lalu verifikasi identik (3.14).
13. **Otorisasi `admin` dan penolakan non-admin** — amati izin `admin` dan 403 untuk siswa/pengawas pada F, lalu verifikasi identik setelah pemeriksaan handler dihapus (3.15). Ini pasangan wajib dari 2.19.
14. **Soft delete `id` valid** — amati respons sukses `DeleteClass`/`DeleteRoom`/`DeleteStudent` pada baris aktif di F, lalu verifikasi identik (3.16).
15. **Nama paket lowercase** — amati `paket.zip` → `paket` pada F, lalu verifikasi tidak berubah (3.17).
16. **`CreateRoom` valid dan verifikasi hash** — amati pembuatan ruang serta keberhasilan login pengawas pada F, lalu verifikasi keduanya tetap (3.18).
17. **Infraction tunggal** — amati counter dan lock pada satu pelanggaran di F, lalu verifikasi identik (3.19).
18. **Scheduling non-batas** — amati keputusan enterable dan overlap pada waktu yang jelas di dalam/di luar jendela pada F, lalu verifikasi identik (3.20).
19. **Bentuk deployment** — verifikasi build tetap menghasilkan single binary `aether-cbt.exe` pada port yang sama tanpa dependensi service eksternal baru (3.3). Diperiksa lewat `build.ps1` dan inspeksi `go.mod`.

### Unit Tests

- **Konfigurasi koneksi**: nilai ketiga pragma pasca-`Connect`; pesan error verifikasi menyebut pragma, nilai diharapkan, dan nilai aktual; bentuk DSN untuk path yang mengandung karakter khusus.
- **Normalisasi peserta**: absen → `NULL`, `0` → `NULL`, nilai valid → apa adanya; keempat jalur tulis (handler, seed, CSV, migrasi legacy).
- **Queue**: `Stop` idempotent; `Close` idempotent dan `Enqueue` pasca-`Close`; keberadaan dan isi `.error.txt`; `GetStats` pada tabel kosong; `RecoverStartup` terhadap nama file lama maupun baru.
- **Guard input**: token login kosong/whitespace; `attempt_token` kosong; `id` non-integer pada delete; field wajib `CreateRoom`; keunikan `username`.
- **Predikat tenant**: attach/unlink lintas tenant → not-found tanpa efek samping; attach/unlink tenant sendiri → sukses.
- **Penanganan error**: scan error → 500; `rows.Err()` non-nil → 500; `NULL` skor → baris tetap ada; kegagalan kueri per-kelas → error bukan fallback; kegagalan `HashPassword` → operasi batal.
- **Otorisasi**: `admin` dan `superadmin` diizinkan di kesembilan endpoint; siswa dan pengawas ditolak 403.
- **Ekstensi berkas**: `.zip`, `.ZIP`, `.Zip`, dan nama tanpa ekstensi.
- **Interval scheduling**: instan `mulai`, instan `selesai`, di dalam, dan di luar jendela — untuk `enterable` maupun `windowsOverlap` di bawah satu konvensi.

### Property-Based Tests

Memakai `pgregory.net/rapid` (sudah menjadi dependensi), mengikuti konvensi anotasi yang ada dengan `// Feature: codebase-bug-sweep, Property 1` dan `// Feature: codebase-bug-sweep, Property 2`.

- **Property 1 / C3 — jadwal backoff.** Bangkitkan urutan enqueue, kegagalan, dan lompatan waktu acak; assert bahwa tidak ada job yang pernah di-dequeue sebelum waktu jatuh temponya, dan bahwa job baru tetap terproses ketika ada job yang belum jatuh tempo di queue.
- **Property 1 / C4 — isolasi batch.** Bangkitkan batch berukuran acak dengan subset job gagal yang acak; assert bahwa himpunan job yang berakhir di `done/` tepat sama dengan himpunan job valid, dan himpunan yang di-`MarkFailed` tepat sama dengan himpunan job gagal.
- **Property 1 / C7 — kardinalitas room-status.** Bangkitkan peserta dengan jumlah `hasil_tes` acak (0..N) di sesi-sesi acak; assert jumlah baris keluaran tepat sama dengan jumlah peserta, dan bahwa nilai yang tampil selalu berasal dari `session_id` yang diminta.
- **Property 1 / C22 — atomisitas increment.** Bangkitkan jumlah pemanggil konkuren acak; assert nilai kembalian seluruhnya distinct dan ambang lock terpicu tepat sekali.
- **Property 1 / C20 — kapitalisasi ekstensi.** Bangkitkan nama berkas dengan kapitalisasi `.zip` acak; assert nama tersimpan selalu tanpa ekstensi.
- **Property 2 — preservation di seluruh domain non-bug.** Bangkitkan input non-bug acak (id valid, token tidak kosong, tenant cocok, tanpa `NULL`, batch seluruhnya valid) dan assert hasil F' cocok dengan baseline yang direkam dari F.
- **Property 2 / 3.5 — atomisitas jalur normal.** Bangkitkan batch yang seluruhnya valid dengan ukuran acak; assert tepat satu commit terjadi per batch.

### Integration Tests

- **Alur ujian lengkap dengan FK aktif** — login siswa, kerjakan, submit lewat webhook, proses queue, hasil tersimpan, ekspor. Dijalankan setelah Gate 1 untuk membuktikan penegakan FK tidak mematahkan jalur produksi mana pun.
- **Restart dan pemulihan** — kirim sinyal saat ada job in-flight, restart, verifikasi tidak ada hasil ujian yang hilang dan tidak ada penantian sweep 5 menit (2.23 + 2.9 + 2.8 bersama-sama).
- **Isolasi tenant end-to-end** — dua tenant dengan data lengkap; jalankan seluruh operasi lintas tenant di 2.5, 2.6, 2.7 dan verifikasi tidak ada kebocoran baca maupun tulis.
- **Room-status dan SSE bersama** — verifikasi endpoint HTTP dan stream SSE menghasilkan data yang konsisten setelah 2.7, karena keduanya berbagi `fetchRoomStatus`.
- **Beban dan latency** — `tests/load/verify_e2e.go` menerima flag `-scale` (50/100/200/500) dan menambahkan hasil ke `tests/load/E2E_RESULTS.md`. Jalankan pada 200 dan 500 **sebelum dan sesudah** Gate 2 untuk membuktikan klausa 3.5. Catatan: target handler P95 <100ms pada burst 200/500 **saat ini belum terpenuhi** — ini terlacak sebagai task 14 yang masih terbuka di `.kiro/specs/filesystem-submission-queue/tasks.md`. Spec ini karena itu diukur terhadap **baseline yang direkam**, bukan terhadap target absolut: kriterianya adalah "tidak lebih buruk dari sebelum Gate 2", dan menutup target absolut tetap menjadi pekerjaan spec tersebut.
- **Ekspor lintas format** — verifikasi CSV, XLSX, dan PDF setelah 2.15 dan 2.16 pada data yang mengandung `NULL` maupun tidak.
