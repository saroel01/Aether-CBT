# Bugfix Requirements Document

## Introduction

Sapuan analisis kode menyeluruh atas Aether-CBT (Go + Fiber v2 + `modernc.org/sqlite`, platform CBT multi-tenant, single binary offline-first di LAN sekolah) menemukan 23 temuan yang dikonversi menjadi 25 klausa defect di dokumen ini. Baseline verifikasi saat ini bersih: `go vet ./...` exit 0 dan `go test ./...` lulus di seluruh paket. Artinya **tidak ada satu pun bug di bawah ini yang tertangkap oleh test suite yang ada** — semuanya ditemukan melalui pembacaan kode (plus satu probe empiris terhadap driver SQLite untuk klausa 1.1).

Dampak terberat terkumpul di tiga area:

- **Integritas data hasil ujian.** Pragma SQLite (WAL, `foreign_keys`, `busy_timeout`) sebenarnya tidak pernah aktif, dan satu job submission yang cacat bisa menjatuhkan sampai `batchSize-1` hasil ujian valid ke dead letter. Ini kehilangan nilai siswa yang nyata dan silent.
- **Isolasi tenant.** Tiga jalur tulis/baca (attach kelas/ruang ke sesi, unlink mapping kelas–mapel, monitoring ruang) tidak memfilter tenant atau sesi dengan benar, sehingga admin tenant A dapat memodifikasi data tenant B, dan monitoring dapat menampilkan skor dari ujian lain.
- **Keandalan operasional.** Backoff retry queue tidak berfungsi sama sekali (retry loop ketat yang membuat submission baru kelaparan), tidak ada graceful shutdown sehingga job di `processing/` baru pulih setelah threshold 5 menit, dan puluhan handler list/export menelan error `rows.Scan` sehingga hasil parsial dilaporkan sebagai sukses.

Klausa diurutkan berdasarkan prioritas dan ditandai: **[P0]** kritis/tinggi (1.1–1.7), **[P1]** medium (1.8–1.23), **[P2]** rendah/konsistensi (1.24–1.25). Setiap klausa Current Behavior (1.x) berpasangan indeks dengan Expected Behavior (2.x).

Perbaikan wajib mempertahankan konstrain produk: single binary `aether-cbt.exe`, offline-first di LAN sekolah, tanpa service eksternal tambahan, dan seluruh test suite harus tetap hijau setelah setiap perbaikan.

## Bug Analysis

### Current Behavior (Defect)

Perilaku yang terjadi saat ini pada kode sebelum perbaikan (F).

**[P0] Integritas data dan isolasi tenant**

1.1 WHEN aplikasi membuka koneksi database melalui `internal/db/sqlite.go` `Connect` THEN sistem membentuk DSN dengan sintaks parameter `mattn/go-sqlite3` (`?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000`) yang diabaikan total oleh driver `modernc.org/sqlite`, sehingga koneksi aktif berjalan dengan `journal_mode=delete`, `busy_timeout=0`, dan `foreign_keys=0` — writer memblokir reader, `database is locked` muncul tanpa penundaan (yang memaksa adanya retry loop SQLITE_BUSY ad-hoc di `internal/db/migrate.go` dan `internal/repository/exam_session_repo.go`), dan seluruh constraint FOREIGN KEY di 20+ migrasi tidak pernah ditegakkan.

1.2 WHEN `CreateStudent` di `internal/api/handlers/student.go` menerima request tanpa `kelas_id` atau `ruang_id` THEN sistem menyimpan nilai sentinel `0` ke kolom `peserta.kelas_id`/`peserta.ruang_id` yang bereferensi ke `kelas(id)`/`ruang(id)`; karena `0` bukan `NULL`, baris tersebut melanggar FOREIGN KEY dan hanya lolos selama penegakan FK mati (lihat 1.1).

1.3 WHEN sebuah job di filesystem queue gagal dan `MarkFailed` di `internal/submission/fsqueue.go` menghitung backoff eksponensial THEN sistem hanya menuliskan hasil hitungan itu ke field `job.EnqueuedAt` tanpa mengubah nama file, sementara `Dequeue` memilih kandidat murni dari urutan nama hasil `os.ReadDir` dan tidak pernah membaca `EnqueuedAt`, sehingga job gagal langsung di-dequeue ulang dan — karena prefix unix_nano-nya paling tua di `pending/` — selalu terpilih lebih dulu: retry loop ketat yang menghabiskan seluruh `maxRetries` dalam hitungan milidetik dan membuat submission baru kelaparan.

1.4 WHEN satu job dalam sebuah batch gagal permanen (grace period terlampaui, skor tidak cocok, peserta tidak ditemukan) THEN `ProcessBatch` di `internal/submission/processor.go` me-rollback seluruh transaksi dan mengembalikan error tunggal, lalu `processBatchSafe` di `internal/submission/worker.go` memanggil `MarkFailed` untuk SETIAP job dalam batch, sehingga sampai `batchSize-1` submission yang sepenuhnya valid ikut di-retry berulang dan akhirnya berakhir di `failed/` — kehilangan hasil ujian nyata secara silent.

1.5 WHEN `LinkSessionClasses` atau `LinkSessionRooms` (`attachSession` di `internal/api/handlers/exam_session_handler.go`) dipanggil dengan `sessionID` dari URL THEN `AttachClasses`/`AttachRooms` di `internal/repository/exam_session_repo.go` hanya memverifikasi bahwa setiap kelas/ruang milik `tenantID` pemanggil dan tidak pernah memverifikasi bahwa sesi tersebut milik tenant yang sama, sehingga admin tenant A dapat menautkan kelas dan ruangnya ke sesi ujian milik tenant B.

1.6 WHEN `UnlinkClassSubject` di `internal/api/handlers/mapping_handler.go` dipanggil THEN sistem menjalankan `DELETE FROM kelas_mapel WHERE kelas_id = ? AND mapel_id = ?` tanpa filter `tenant_id` dan tanpa validasi input, sehingga admin tenant A dapat menghapus mapping kurikulum tenant B — padahal `LinkClassSubject` pasangannya sudah diperketat dengan pemeriksaan tenant.

1.7 WHEN `fetchRoomStatus` di `internal/api/handlers/supervisor.go` melayani `GET /api/supervisor/room-status` atau stream SSE di `supervisor_sse.go` THEN sistem melakukan `LEFT JOIN hasil_tes ht ON p.id = ht.peserta_id AND ht.tenant_id = p.tenant_id` yang hanya menjoin pada peserta, sehingga seorang siswa dengan N baris `hasil_tes` menghasilkan N baris keluaran (bertentangan dengan komentar fungsinya sendiri: satu baris per siswa); dan ketika `session_id` diberikan, subquery `cek_login` dibatasi per sesi sementara join `hasil_tes` tidak, sehingga `skor`, `status`, dan `waktu_selesai` yang ditampilkan untuk satu sesi bisa berasal dari ujian yang sama sekali berbeda.

**[P1] Keandalan worker, autentikasi, dan konsistensi handler**

1.8 WHEN `Worker.Stop` di `internal/submission/worker.go` dipanggil lebih dari sekali (misalnya `defer worker.Stop()` di `cmd/server/main.go` setelah stop eksplisit di test) THEN sistem menjalankan `close(w.stopChan)` tanpa pengaman dan panic dengan `close of closed channel`.

1.9 WHEN `NewFilesystemQueueWithConfig` di `internal/submission/fsqueue.go` membuat instance queue THEN sistem menjalankan `go q.runEnqueueWriter()` sementara `enqueueCh` tidak pernah ditutup dan tidak ada metode `Close`/`Shutdown`, sehingga setiap instance queue membocorkan satu goroutine selama proses hidup — paling terasa di test yang membuat banyak queue.

1.10 WHEN `MarkFailed` di `internal/submission/fsqueue.go` memindahkan job ke `failed/` karena `RetryCount >= maxRetries` THEN sistem me-rename file tanpa menuliskan file pendamping `<name>.error.txt` yang ditulis oleh jalur corrupt-file di `Dequeue` dan yang diinstruksikan untuk dibaca admin oleh `docs/runbooks/queue-and-litestream.md`, sehingga dua jalur dead letter menjadi tidak konsisten dan penyebab kegagalan hanya tersembunyi di dalam JSON.

1.11 WHEN `SQLiteQueue.GetStats` di `internal/submission/queue.go` dijalankan sementara tabel `submission_queue` kosong THEN `SUM(CASE WHEN ... END)` menghasilkan `NULL`, `Scan` ke variabel `int` gagal, dan sistem mengembalikan error alih-alih statistik nol — dapat dicapai melalui `BufferedSQLiteQueue` yang masih dipakai fixture legacy.

1.12 WHEN seorang siswa melakukan `StudentLogin` (`internal/api/handlers/exam.go`) dengan token ujian kosong THEN sistem tidak memiliki guard token kosong sebelum resolusi, sehingga `resolveSessionForToken` di `internal/api/handlers/student_session_handler.go` memanggil `FindByToken(tenantID, "")` dan dapat mencocokkan `exam_session` yang tokennya `''` — kondisi yang nyata terjadi karena `migrateTenant` di `internal/service/legacy_migration.go` menyalin `settings.token` apa adanya ke token sesi legacy; jika sesi itu enterable, siswa berhasil login tanpa token sama sekali (cabang fallback legacy yang menolak token kosong tidak pernah tercapai).

1.13 WHEN `ISpringWebhook` di `internal/api/handlers/ispring.go` menerima `attempt_token` kosong THEN sistem langsung menjalankan `WHERE cl.attempt_token = ? AND p.no_id = ?` dan bergantung sepenuhnya pada asumsi bahwa tidak ada baris `cek_login` yang memiliki `attempt_token = ''`, tanpa guard eksplisit yang mengembalikan 403 `invalid attempt token` sebagaimana perilaku yang didokumentasikan.

1.14 WHEN server produksi berjalan dengan `ContentCookieSecure` pada nilai default `"auto"` THEN `cmd/server/main.go` mengevaluasi `handlers.SetContentCookieSecure(cfg.ContentCookieSecure == "true")` sehingga `"auto"` runtuh menjadi `false`, lalu `setContentCookie` jatuh ke `c.Secure()` yang bernilai `false` di belakang reverse proxy yang menerminasi TLS — cookie sesi `aether_exam` dikirim tanpa atribut `Secure`, dan deteksi otomatis yang dijanjikan komentar dokumentasi `internal/config/config.go` tidak pernah ada.

1.15 WHEN sebuah baris hasil kueri gagal di-scan di handler list/export THEN sistem membuang nilai kembalian `rows.Scan(...)` dan tidak pernah memeriksa `rows.Err()`, sehingga hasil parsial dikembalikan sebagai sukses HTTP 200. Terpengaruh: `student_exam.go` `GetAvailableMapels`, `ispring.go` `GetEducationalAnalysis`, `mapping_handler.go` `GetClassSubjects`, `student.go` `GetStudents`, `ruang.go` `GetRooms`, `essay_grading.go` `GetEssayAnswers`, `item_analysis.go` `GetItemAnalysis`, serta `csv_utility.go` `ExportResultsCSV` dan `ExportEssayResults` (ketiga cabang csv/xlsx/pdf).

1.16 WHEN `ExportResultsCSV` di `internal/api/handlers/csv_utility.go` menemui baris dengan `skor` atau `skor_maks` bernilai `NULL` THEN sistem gagal men-scan ke `float64` non-nullable dan menjalankan `continue`, sehingga baris siswa tersebut hilang sepenuhnya dari file ekspor tanpa peringatan apa pun kepada admin.

1.17 WHEN kueri mapel yang di-scope per kelas di `GetAvailableMapels` (`internal/api/handlers/student_exam.go`) gagal THEN `rows` bernilai `nil` sehingga alur masuk ke cabang `if rows == nil` yang dimaksudkan sebagai fallback "semua mapel", menimpa `err` dan membuang kegagalan tersebut — siswa lalu ditampilkan seluruh mapel dalam tenant, bukan hanya mapel yang dimapping ke kelasnya.

1.18 WHEN `DeleteClass` (`internal/api/handlers/kelas.go`) atau `DeleteRoom` (`internal/api/handlers/ruang.go`) dipanggil THEN sistem memakai `c.Params("id")` sebagai string mentah tanpa validasi, menghilangkan `deleted_at IS NULL` dari klausa WHERE, dan selalu mengembalikan 200 meskipun tidak ada baris yang cocok — berbeda dari `DeleteStudent` di `student.go` yang sudah benar (`ParamsInt`, filter `deleted_at`, `RowsAffected` → 404).

1.19 WHEN pengguna dengan role `superadmin` memanggil endpoint admin THEN route di `cmd/server/main.go` mengizinkannya lewat `middleware.RequireRoles("admin", "superadmin")` tetapi handler memeriksa ulang `role != "admin"` dan mengembalikan 403, sehingga superadmin terkunci dari `UpdateSettings`, `ImportStudentsCSV`, `GradeEssayAnswer`, `LinkClassSubject`, `UnlinkClassSubject`, `DeleteStudent`, `DeleteClass`, `DeleteMapel`, dan `DeleteRoom`.

1.20 WHEN `UploadSoalPackage` di `internal/api/handlers/soal_package_handler.go` menerima berkas bernama `PAKET.ZIP` THEN sistem menjalankan `strings.TrimSuffix(file.Filename, strings.ToLower(".zip"))` yang meng-lowercase literal suffix alih-alih nama berkasnya, sehingga paket tersimpan dengan nama `PAKET.ZIP` beserta ekstensinya.

1.21 WHEN `CreateRoom` di `internal/api/handlers/ruang.go` dipanggil THEN sistem mengabaikan error dari `hash, _ := utils.HashPassword(req.Password)` sehingga kegagalan hashing menyimpan `password_hash` kosong, dan `nama_ruang`, `username`, serta `password` tidak diwajibkan maupun diperiksa keunikannya.

1.22 WHEN dua pelanggaran anti-cheat terjadi bersamaan THEN `IncrementInfraction` di `internal/repository/cek_login_repo.go` menjalankan UPDATE lalu SELECT terpisah pada statement berbeda, sehingga kedua pemanggil dapat membaca nilai pasca-update yang sama dan ambang lock terlewati tanpa sesi pernah benar-benar terkunci.

1.23 WHEN proses server dihentikan THEN `cmd/server/main.go` berakhir pada `log.Fatal(app.Listen(...))` yang memanggil `os.Exit`, sehingga `defer cancel()`, `defer worker.Stop()`, dan `defer db.Close()` tidak pernah dijalankan; job yang sedang berada di `processing/` hanya dipulihkan oleh sweep stuck-threshold, dan karena `RecoverStartup` default `forceAll=false` dengan threshold 5 menit, hasil ujian dapat tertahan tanpa diproses selama beberapa menit setelah restart.

**[P2] Konsistensi**

1.24 WHEN dua sesi ujian bersebelahan berbagi token dan waktu berada tepat di batas jendela THEN `internal/service/scheduling_service.go` memperlakukan `enterable` sebagai inklusif di kedua ujung `[mulai, selesai]` sementara `windowsOverlap` memakai setengah terbuka `[start, end)`, sehingga kedua sesi dinyatakan tidak bertumpang tindih namun keduanya enterable pada instan yang sama.

1.25 WHEN `processOneInTx` di `internal/submission/processor.go` menjalankan cabang diagnostiknya THEN sistem melakukan `p.db.QueryRowContext(...)` pada koneksi terpisah sementara transaksi tulis `tx` masih terbuka — tidak berbahaya di bawah WAL, tetapi menjadi risiko self-deadlock begitu perilaku journal/locking berubah (lihat 1.1) dan tidak konsisten dengan seluruh operasi baca lain di fungsi tersebut yang memakai `tx`.

### Expected Behavior (Correct)

Perilaku yang harus dipenuhi kode setelah perbaikan (F').

**[P0] Integritas data dan isolasi tenant**

2.1 WHEN aplikasi membuka koneksi database THEN sistem SHALL menggunakan sintaks DSN yang dikenali `modernc.org/sqlite` (`file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)`) DAN SHALL memverifikasi setelah koneksi terbentuk bahwa `journal_mode=wal`, `foreign_keys=1`, dan `busy_timeout=5000` benar-benar aktif, gagal cepat dengan error yang jelas bila tidak.

2.2 WHEN `CreateStudent` menerima request tanpa `kelas_id` atau `ruang_id` THEN sistem SHALL menormalkan nilai kosong/`0` menjadi `NULL` sebelum INSERT pada semua jalur tulis `peserta` (handler, seed, import CSV, migrasi legacy) DAN SHALL memverifikasi serta membersihkan baris eksisting yang menyimpan sentinel `0` sebelum penegakan FK diaktifkan, sehingga pengaktifan `foreign_keys` di 2.1 tidak mematahkan alur yang sudah berjalan.

2.3 WHEN sebuah job gagal dan backoff eksponensial `min(2^(n-1), 30)` detik dihitung THEN sistem SHALL membuat jadwal itu benar-benar berlaku — job SHALL tidak dapat di-dequeue sebelum waktunya tiba (misalnya dengan mengkodekan waktu jadwal ke dalam nama file dan/atau membaca `EnqueuedAt` saat pemilihan kandidat) DAN job yang belum jatuh tempo SHALL dilewati sehingga submission baru tetap terproses.

2.4 WHEN satu job dalam sebuah batch gagal THEN sistem SHALL mengisolasi kegagalan pada job tersebut saja (savepoint per job, atau fallback ke pemrosesan satu-per-satu ketika batch gagal) DAN SHALL memanggil `MarkFailed` hanya untuk job yang benar-benar gagal, sementara seluruh job valid lainnya di batch yang sama SHALL tercommit dan berpindah ke `done/`.

2.5 WHEN `AttachClasses` atau `AttachRooms` dipanggil THEN sistem SHALL memverifikasi bahwa `sessionID` milik `tenantID` pemanggil sebelum melakukan penulisan apa pun DAN SHALL mengembalikan error not-found/forbidden tanpa mengubah data bila sesi tersebut milik tenant lain.

2.6 WHEN `UnlinkClassSubject` dipanggil THEN sistem SHALL memvalidasi `kelas_id` dan `mapel_id` sebagai input, SHALL menyertakan filter `tenant_id` pada perintah DELETE (setara dengan pengetatan yang sudah diterapkan di `LinkClassSubject`), DAN SHALL mengembalikan 404 bila tidak ada mapping milik tenant tersebut yang cocok.

2.7 WHEN `fetchRoomStatus` melayani room-status atau stream SSE THEN sistem SHALL mengembalikan tepat satu baris per peserta DAN, ketika `session_id` diberikan, SHALL membatasi join `hasil_tes` pada sesi/mapel yang sama dengan subquery `cek_login` sehingga `skor`, `status`, dan `waktu_selesai` selalu berasal dari sesi yang diminta.

**[P1] Keandalan worker, autentikasi, dan konsistensi handler**

2.8 WHEN `Worker.Stop` dipanggil berapa kali pun THEN sistem SHALL menutup `stopChan` paling banyak satu kali (`sync.Once` atau guard select) DAN SHALL kembali tanpa panic pada pemanggilan kedua dan selanjutnya.

2.9 WHEN sebuah instance filesystem queue tidak lagi dibutuhkan THEN sistem SHALL menyediakan metode `Close`/`Shutdown` yang menghentikan `runEnqueueWriter` dan melepaskan goroutine-nya DAN pemanggil (server serta test) SHALL memakai metode tersebut sehingga tidak ada goroutine yang bocor.

2.10 WHEN sebuah job dipindahkan ke `failed/` karena melampaui `maxRetries` THEN sistem SHALL menuliskan file pendamping `<name>.error.txt` berisi `LastError` dan konteks retry, konsisten dengan jalur corrupt-file di `Dequeue` dan dengan `docs/runbooks/queue-and-litestream.md`.

2.11 WHEN `SQLiteQueue.GetStats` dijalankan pada tabel `submission_queue` yang kosong THEN sistem SHALL mengembalikan statistik bernilai nol tanpa error, dengan setiap agregat dibungkus `COALESCE(..., 0)`.

2.12 WHEN `StudentLogin` menerima token ujian kosong atau hanya berisi whitespace THEN sistem SHALL menolak permintaan sebelum resolusi sesi dilakukan DAN sistem SHALL tidak pernah memperlakukan `exam_session` bertoken `''` sebagai sesi yang dapat dimasuki, termasuk sesi hasil migrasi legacy dari `settings.token` yang kosong.

2.13 WHEN `ISpringWebhook` menerima `attempt_token` kosong THEN sistem SHALL mengembalikan 403 `invalid attempt token` melalui guard eksplisit sebelum kueri `cek_login` dijalankan.

2.14 WHEN `ContentCookieSecure` bernilai `"auto"` THEN sistem SHALL benar-benar mendeteksi konteks request (skema request serta header proxy tepercaya seperti `X-Forwarded-Proto`, dan/atau environment) untuk menentukan atribut `Secure`, sehingga cookie `aether_exam` dikirim dengan `Secure` pada deployment produksi di belakang reverse proxy yang menerminasi TLS.

2.15 WHEN sebuah baris hasil kueri gagal di-scan di handler list/export THEN sistem SHALL memeriksa error `rows.Scan` maupun `rows.Err()` DAN SHALL melaporkan kegagalan tersebut (error 500 untuk endpoint list, atau log plus penandaan eksplisit yang tidak menyembunyikan data hilang untuk ekspor) alih-alih mengembalikan hasil parsial sebagai sukses.

2.16 WHEN `ExportResultsCSV` menemui `skor` atau `skor_maks` bernilai `NULL` THEN sistem SHALL men-scan ke tipe nullable dan tetap menuliskan baris siswa tersebut dengan penanda nilai kosong, sehingga tidak ada peserta yang hilang dari file ekspor.

2.17 WHEN kueri mapel yang di-scope per kelas gagal THEN `GetAvailableMapels` SHALL mengembalikan error kepada pemanggil DAN SHALL tidak pernah jatuh ke fallback "semua mapel" akibat kegagalan kueri, sehingga siswa hanya melihat mapel yang dimapping ke kelasnya.

2.18 WHEN `DeleteClass` atau `DeleteRoom` dipanggil THEN sistem SHALL memvalidasi `id` sebagai integer, SHALL menyertakan `deleted_at IS NULL` pada klausa WHERE, DAN SHALL mengembalikan 404 ketika `RowsAffected` bernilai 0 — sejalan dengan `DeleteStudent`.

2.19 WHEN pengguna dengan role `superadmin` memanggil endpoint admin THEN sistem SHALL menerapkan satu sumber kebenaran otorisasi (diutamakan middleware route) DAN pemeriksaan role di handler SHALL konsisten dengannya, sehingga superadmin dapat mengakses `UpdateSettings`, `ImportStudentsCSV`, `GradeEssayAnswer`, `LinkClassSubject`, `UnlinkClassSubject`, `DeleteStudent`, `DeleteClass`, `DeleteMapel`, dan `DeleteRoom`.

2.20 WHEN `UploadSoalPackage` menerima berkas dengan ekstensi `.zip` dalam kapitalisasi apa pun THEN sistem SHALL membuang ekstensi tersebut secara case-insensitive, sehingga `PAKET.ZIP` tersimpan dengan nama `PAKET`.

2.21 WHEN `CreateRoom` dipanggil THEN sistem SHALL menangani error dari `utils.HashPassword` dan membatalkan operasi bila hashing gagal, DAN SHALL memvalidasi `nama_ruang`, `username`, serta `password` sebagai wajib serta memeriksa keunikan `username` sebelum INSERT.

2.22 WHEN pelanggaran anti-cheat bersamaan terjadi THEN `IncrementInfraction` SHALL menaikkan dan membaca counter secara atomic dalam satu operasi (`UPDATE ... RETURNING` atau di dalam satu transaksi), sehingga setiap pemanggil menerima nilai yang berbeda dan ambang lock selalu terdeteksi tepat sekali.

2.23 WHEN proses server menerima SIGINT atau SIGTERM THEN sistem SHALL menjalankan graceful shutdown melalui `app.Shutdown()` sehingga worker dihentikan, job in-flight diselesaikan atau dikembalikan ke `pending/`, context dibatalkan, dan koneksi database ditutup sebelum proses keluar — tanpa bergantung pada sweep stuck-threshold 5 menit setelah restart.

**[P2] Konsistensi**

2.24 WHEN kelayakan masuk sesi dan tumpang tindih jendela dievaluasi THEN `scheduling_service.go` SHALL memakai konvensi interval yang sama untuk `enterable` dan `windowsOverlap`, sehingga dua sesi bersebelahan tidak dapat sekaligus dinyatakan tidak bertumpang tindih dan keduanya enterable pada instan batas.

2.25 WHEN `processOneInTx` menjalankan cabang diagnostiknya THEN sistem SHALL melakukan pembacaan melalui `tx` yang sedang terbuka, konsisten dengan pembacaan lain di fungsi tersebut dan bebas dari risiko self-deadlock.

### Unchanged Behavior (Regression Prevention)

Perilaku eksisting yang harus tetap sama untuk input di luar kondisi bug (¬C(X)).

3.1 WHEN seluruh migrasi dan test suite dijalankan pada data yang konsisten secara referensial THEN sistem SHALL CONTINUE TO menjalankan 20+ migrasi sampai selesai dan seluruh paket SHALL CONTINUE TO lulus meskipun `foreign_keys` kini aktif — `go vet ./...` tetap exit 0 dan `go test ./...` tetap hijau.

3.2 WHEN `CreateStudent`, seed, atau import CSV menerima `kelas_id` dan `ruang_id` yang valid (`> 0` dan ada di tabel induk) THEN sistem SHALL CONTINUE TO menyimpan relasi itu apa adanya dan mengembalikan respons yang sama seperti sekarang.

3.3 WHEN aplikasi berjalan normal THEN sistem SHALL CONTINUE TO beroperasi sebagai single binary offline-first di LAN sekolah pada port yang sama, tanpa dependensi service eksternal baru.

3.4 WHEN sebuah job berhasil diproses pada percobaan pertama THEN sistem SHALL CONTINUE TO memindahkannya ke `done/` tanpa penundaan tambahan — backoff hanya berlaku bagi job yang gagal.

3.5 WHEN seluruh job dalam satu batch valid THEN sistem SHALL CONTINUE TO menuliskannya dalam satu transaksi atomic dan SHALL CONTINUE TO memenuhi throughput batch serta target latency handler yang sudah ada pada burst 200 dan 500 siswa.

3.6 WHEN sebuah job memang gagal permanen (grace period terlampaui, skor tidak cocok, peserta tidak ditemukan) THEN sistem SHALL CONTINUE TO memindahkannya ke `failed/` setelah `maxRetries` terlampaui.

3.7 WHEN admin melakukan attach sesi–kelas/ruang, link, atau unlink pada data milik tenant sendiri THEN sistem SHALL CONTINUE TO menyelesaikan operasi tersebut dengan respons sukses yang sama.

3.8 WHEN seorang peserta memiliki tepat satu `hasil_tes` untuk sesi yang diminta THEN room-status dan stream SSE SHALL CONTINUE TO menampilkan `skor`, `status`, dan `waktu_selesai` yang sama seperti sekarang, dan peserta tanpa hasil SHALL CONTINUE TO muncul sebagai satu baris dengan nilai kosong (perilaku LEFT JOIN).

3.9 WHEN `Worker.Stop` dipanggil sekali THEN sistem SHALL CONTINUE TO menghentikan worker beserta loop dequeue-nya seperti sekarang.

3.10 WHEN token ujian tidak kosong dan benar THEN login siswa SHALL CONTINUE TO berhasil, dan token yang salah SHALL CONTINUE TO ditolak dengan status serta pesan error yang sama seperti sekarang.

3.11 WHEN webhook iSpring mengirim `attempt_token` yang valid THEN sistem SHALL CONTINUE TO mengembalikan 200 dan meng-enqueue job seperti sekarang.

3.12 WHEN `ContentCookieSecure` di-set eksplisit ke `"true"` atau `"false"` THEN sistem SHALL CONTINUE TO menghormati nilai tersebut, dan ujian di LAN sekolah tanpa TLS SHALL CONTINUE TO dapat menetapkan cookie `aether_exam` sehingga siswa tetap bisa mengerjakan ujian.

3.13 WHEN semua baris hasil kueri dapat di-scan THEN setiap endpoint list dan export SHALL CONTINUE TO mengembalikan payload, urutan, serta format yang identik — termasuk header dan kolom pada keluaran CSV, XLSX, dan PDF.

3.14 WHEN seorang siswa memiliki mapping `kelas_mapel` THEN `GetAvailableMapels` SHALL CONTINUE TO mengembalikan hanya mapel yang dimapping ke kelasnya.

3.15 WHEN pengguna dengan role `admin` memanggil endpoint admin THEN sistem SHALL CONTINUE TO mengizinkannya, dan role non-admin (siswa, pengawas) SHALL CONTINUE TO ditolak dengan 403.

3.16 WHEN `DeleteClass`, `DeleteRoom`, atau `DeleteStudent` dipanggil dengan `id` valid yang menunjuk baris aktif THEN sistem SHALL CONTINUE TO melakukan soft delete dan mengembalikan respons sukses seperti sekarang.

3.17 WHEN sebuah paket soal diunggah dengan nama berakhiran `.zip` huruf kecil THEN sistem SHALL CONTINUE TO menyimpannya dengan nama tanpa ekstensi seperti sekarang.

3.18 WHEN `CreateRoom` dipanggil dengan data lengkap, unik, dan valid THEN sistem SHALL CONTINUE TO membuat ruang tersebut dan hash password-nya SHALL CONTINUE TO dapat diverifikasi saat login pengawas.

3.19 WHEN hanya satu pelanggaran anti-cheat terjadi THEN `IncrementInfraction` SHALL CONTINUE TO mengembalikan nilai counter yang benar dan SHALL CONTINUE TO mengunci sesi saat ambang tercapai.

3.20 WHEN sesi ujian dievaluasi pada waktu yang jelas berada di dalam atau di luar jendela (bukan instan batas) THEN `scheduling_service.go` SHALL CONTINUE TO memberikan keputusan enterable dan overlap yang sama seperti sekarang.
