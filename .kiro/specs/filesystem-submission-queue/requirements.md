# Requirements Document

## Introduction

Aether CBT saat ini menyimpan antrean submission (hasil ujian iSpring) di tabel `submission_queue` dalam file SQLite yang sama dengan tabel hasil ujian utama (`hasil_tes`, `hasil_tes_detail`, `cek_login`). Hasil load test end-to-end (`tests/load/E2E_RESULTS.md`) menunjukkan kebocoran data signifikan: pada burst 200 siswa hanya 27% nilai benar-benar tersimpan, 72% berakhir di dead letter, dan beberapa job stuck di status `processing` selamanya. HTTP 200 dikembalikan untuk semua submission, tetapi data hilang silent di worker.

Fitur ini menggantikan tabel `submission_queue` dengan antrean berbasis filesystem (satu file JSON per submission, dengan direktori `pending/`, `processing/`, `done/`, `failed/`) yang dioperasikan dengan rename atomic. Bersamaan dengan itu, validasi anti-cheat sinkron dipulihkan di handler webhook, penulisan hasil dibungkus dalam satu transaksi atomic, worker diberi panic recovery, dan recovery startup ditambahkan untuk job zombie. Tujuan utama: 100% E2E success rate pada burst hingga 500 siswa concurrent (skala ujian rata-rata di sekolah pengguna), dengan operasionalitas yang dapat dipantau admin sekolah awam melalui File Explorer.

Fitur ini WAJIB mempertahankan konstrain produk: single binary `aether-cbt.exe`, offline-first di LAN sekolah, tidak ada service eksternal tambahan (Postgres, Redis, dll).

## Glossary

- **Aether_CBT**: Aplikasi Computer-Based Testing multi-tenant (Go Fiber + SQLite WAL + SvelteKit) yang dijalankan sebagai single binary di LAN sekolah.
- **Filesystem_Queue**: Komponen antrean submission baru yang menyimpan setiap job sebagai file JSON di dalam direktori `pending/`, `processing/`, `done/`, atau `failed/` di bawah root queue (default `data/queue/`).
- **Queue_Root**: Direktori root antrean yang dikonfigurasi via env var `QUEUE_DIR` (default `data/queue/`). Berisi sub-direktori `pending/`, `processing/`, `done/`, `failed/`, dan `tmp/`.
- **Pending_Dir**: Sub-direktori `Queue_Root/pending/` yang berisi job menunggu diproses.
- **Processing_Dir**: Sub-direktori `Queue_Root/processing/` yang berisi job sedang diproses oleh worker.
- **Done_Dir**: Sub-direktori `Queue_Root/done/` yang berisi job berhasil diproses.
- **Failed_Dir**: Sub-direktori `Queue_Root/failed/` (dead letter) yang berisi job yang gagal melebihi `Max_Retries`.
- **Tmp_Dir**: Sub-direktori `Queue_Root/tmp/` tempat file ditulis sebelum di-rename atomic ke `Pending_Dir`.
- **Job_File**: File JSON tunggal di salah satu sub-direktori queue. Nama file mengikuti pola `<timestamp>-<tenant_id>-<no_id>-<random_suffix>.json`.
- **Submission_Job**: Struktur data yang merepresentasikan satu webhook iSpring (tenant_id, no_id, score, max_score, detail_xml, attempt_token, validasi, retry_count, last_error, enqueued_at).
- **ISpring_Webhook_Handler**: HTTP handler `POST /api/ispring/webhook` yang menerima form-data dari iSpring, menjalankan validasi sinkron, lalu enqueue ke `Filesystem_Queue`.
- **Submission_Worker**: Goroutine background tunggal yang melakukan dequeue dari `Filesystem_Queue` dan memanggil `Submission_Processor`.
- **Submission_Processor**: Komponen yang menulis hasil ujian (INSERT `hasil_tes`, INSERT N `hasil_tes_detail`, DELETE `cek_login`) dalam satu transaksi atomic.
- **Anti_Cheat_Validation**: Sekumpulan pemeriksaan murah yang dijalankan di `ISpring_Webhook_Handler` sebelum job di-enqueue: (a) sesi `cek_login` aktif untuk peserta, (b) `attempt_token` cocok dengan token sesi, (c) `detail_xml` (jika ada) parseable.
- **Max_Retries**: Konstanta jumlah maksimum retry sebelum job dipindahkan ke `Failed_Dir`. Default 5.
- **Stuck_Threshold**: Durasi minimum (default 5 menit) di mana sebuah file di `Processing_Dir` dianggap zombie dan dipromosi balik ke `Pending_Dir` saat startup.
- **Debug_Queue_Endpoint**: HTTP handler `GET /api/debug/queue` yang melaporkan jumlah file per sub-direktori queue.
- **Atomic_Rename**: Operasi `os.Rename` yang dijamin atomic di POSIX dan Windows untuk file di volume yang sama.
- **Validasi_Key**: String unik `<tenant_id>_<no_id>_<mapel_id>` yang dipakai sebagai constraint UNIQUE di tabel `hasil_tes` untuk idempotency.

## Requirements

### Requirement 1: Atomic Enqueue dari Handler ke Pending

**User Story:** Sebagai operator sekolah, saya ingin setiap webhook iSpring yang berhasil divalidasi tertulis ke disk secara atomic, sehingga tidak ada job yang hilang akibat crash di tengah penulisan.

#### Acceptance Criteria

1. WHEN `ISpring_Webhook_Handler` selesai memvalidasi sebuah submission, THE `Filesystem_Queue` SHALL menulis `Job_File` ke `Tmp_Dir` lalu memindahkannya ke `Pending_Dir` menggunakan `Atomic_Rename`.
2. THE `Filesystem_Queue` SHALL memberikan nama unik pada `Job_File` dalam format `<unix_nano_timestamp>-<tenant_id>-<no_id>-<8_char_random>.json` agar tidak ada collision pada burst.
3. IF penulisan ke `Tmp_Dir` gagal, THEN THE `Filesystem_Queue` SHALL mengembalikan error ke pemanggil tanpa membuat file di `Pending_Dir`.
4. IF `Atomic_Rename` dari `Tmp_Dir` ke `Pending_Dir` gagal, THEN THE `Filesystem_Queue` SHALL menghapus file sisa di `Tmp_Dir` dan mengembalikan error ke pemanggil.
5. THE `Job_File` SHALL berisi JSON yang valid dan dapat dibaca oleh manusia (indented 2 spasi) dengan field: `tenant_id`, `no_id`, `score`, `max_score`, `detail_xml`, `attempt_token`, `validasi`, `retry_count`, `last_error`, `enqueued_at`.
6. WHEN startup, THE `Filesystem_Queue` SHALL membuat `Pending_Dir`, `Processing_Dir`, `Done_Dir`, `Failed_Dir`, dan `Tmp_Dir` jika belum ada.

### Requirement 2: Atomic Dequeue dengan Pemindahan ke Processing

**User Story:** Sebagai pengembang, saya ingin worker memindahkan file ke `processing/` sebelum memprosesnya, sehingga state in-flight terlihat dari File Explorer dan dapat dipulihkan jika worker mati.

#### Acceptance Criteria

1. WHEN `Submission_Worker` melakukan dequeue, THE `Filesystem_Queue` SHALL memilih satu `Job_File` dari `Pending_Dir` dengan urutan FIFO berdasarkan `enqueued_at`.
2. THE `Filesystem_Queue` SHALL memindahkan `Job_File` terpilih dari `Pending_Dir` ke `Processing_Dir` menggunakan `Atomic_Rename` sebelum mengembalikannya ke pemanggil.
3. WHEN `Pending_Dir` tidak berisi `Job_File` apapun, THE `Filesystem_Queue` SHALL mengembalikan `(nil, nil)` ke pemanggil dalam waktu kurang dari 50 milidetik.
4. IF `Atomic_Rename` dari `Pending_Dir` ke `Processing_Dir` gagal karena file sudah dipindahkan oleh worker lain, THEN THE `Filesystem_Queue` SHALL melanjutkan ke kandidat berikutnya tanpa mengembalikan error.
5. WHEN `Submission_Worker` selesai memproses sebuah `Job_File` dengan sukses, THE `Filesystem_Queue` SHALL memindahkan file dari `Processing_Dir` ke `Done_Dir` menggunakan `Atomic_Rename`.

### Requirement 3: Retry dan Dead Letter

**User Story:** Sebagai admin sekolah, saya ingin kegagalan transient (DB locked sesaat) di-retry otomatis, tetapi kegagalan permanen pindah ke `failed/` agar saya bisa membukanya dengan Notepad untuk investigasi.

#### Acceptance Criteria

1. WHEN `Submission_Processor` mengembalikan error untuk sebuah `Submission_Job` dan `retry_count` kurang dari `Max_Retries`, THE `Filesystem_Queue` SHALL menambah `retry_count`, mengisi `last_error`, dan memindahkan `Job_File` kembali ke `Pending_Dir` dengan `enqueued_at` yang baru.
2. WHEN `Submission_Processor` mengembalikan error untuk sebuah `Submission_Job` dan `retry_count` mencapai `Max_Retries`, THE `Filesystem_Queue` SHALL memindahkan `Job_File` ke `Failed_Dir` dengan `last_error` terisi.
3. THE `Filesystem_Queue` SHALL menerapkan jeda exponential backoff antara retry dengan basis 1 detik dan batas atas 30 detik (1, 2, 4, 8, 16, 30, 30 detik).
4. WHEN `Job_File` dipindahkan ke `Failed_Dir`, THE `Filesystem_Queue` SHALL memastikan field `last_error` di dalam file berisi pesan error terakhir dan field `retry_count` mencerminkan jumlah retry yang sudah dilakukan.
5. THE `Filesystem_Queue` SHALL menulis ulang `Job_File` (dengan `retry_count` dan `last_error` baru) menggunakan pola tulis-ke-tmp-lalu-rename agar metadata retry tidak hilang akibat crash di tengah update.

### Requirement 4: Validasi Sinkron Anti-Cheat di Handler

**User Story:** Sebagai operator iSpring, saya ingin handler webhook mengembalikan 4xx untuk submission yang tidak valid, sehingga saya bisa membedakan submission yang ditolak dari yang berhasil tanpa menunggu worker.

#### Acceptance Criteria

1. WHEN `ISpring_Webhook_Handler` menerima request, THE `ISpring_Webhook_Handler` SHALL menjalankan `Anti_Cheat_Validation` sebelum memanggil enqueue.
2. IF `cek_login` aktif untuk pasangan (`tenant_id`, `no_id`) tidak ditemukan, THEN THE `ISpring_Webhook_Handler` SHALL mengembalikan HTTP 403 dengan body `"active session not found"` dan tidak melakukan enqueue.
3. IF `attempt_token` dari request tidak cocok dengan `attempt_token` di `cek_login`, THEN THE `ISpring_Webhook_Handler` SHALL mengembalikan HTTP 403 dengan body `"invalid attempt token"` dan tidak melakukan enqueue.
4. IF `detail_xml` dari request tidak kosong dan gagal di-parse oleh `ispringparser.ParseDetailedResults`, THEN THE `ISpring_Webhook_Handler` SHALL mengembalikan HTTP 400 dengan body `"Invalid iSpring detailed results XML"` dan tidak melakukan enqueue.
5. IF `no_id` dari request kosong (baik `sid` maupun `USER_NAME`), THEN THE `ISpring_Webhook_Handler` SHALL mengembalikan HTTP 400 dengan body `"Missing student identifier (sid / USER_NAME)"` dan tidak melakukan enqueue.
6. WHEN `Anti_Cheat_Validation` lulus, THE `ISpring_Webhook_Handler` SHALL melakukan enqueue ke `Filesystem_Queue` dan mengembalikan HTTP 200 dengan body `"Result received successfully"`.
7. THE `Anti_Cheat_Validation` SHALL menyelesaikan ketiga pemeriksaan (sesi, token, XML) dengan paling banyak satu query SELECT ke tabel `cek_login` agar overhead per request di bawah 5 milidetik pada DB beban ringan.

### Requirement 5: Penulisan Hasil Atomic dalam Satu Transaksi

**User Story:** Sebagai guru yang melihat nilai siswa, saya ingin hasil ujian yang muncul lengkap (skor utama dan semua detail per soal) atau tidak muncul sama sekali, sehingga grading tidak salah karena data partial.

#### Acceptance Criteria

1. WHEN `Submission_Processor` memproses sebuah `Submission_Job`, THE `Submission_Processor` SHALL menjalankan INSERT/UPSERT `hasil_tes`, INSERT N `hasil_tes_detail`, dan DELETE `cek_login` di dalam satu transaksi DB tunggal.
2. IF salah satu dari INSERT/UPSERT `hasil_tes`, INSERT `hasil_tes_detail`, atau DELETE `cek_login` gagal, THEN THE `Submission_Processor` SHALL melakukan rollback transaksi dan mengembalikan error sehingga tidak ada perubahan parsial yang ter-commit.
3. WHEN `detail_xml` berhasil di-parse menjadi N pertanyaan, THE `Submission_Processor` SHALL memanggil INSERT `hasil_tes_detail` sebanyak N kali di dalam transaksi yang sama dan membiarkan transaksi gagal secara alami jika salah satu INSERT mengembalikan error (tanpa pemeriksaan jumlah baris pasca-commit).
4. WHEN sebuah `Submission_Job` dengan `Validasi_Key` yang sudah ada di `hasil_tes` diproses lagi, THE `Submission_Processor` SHALL melakukan UPSERT pada `hasil_tes` (ON CONFLICT DO UPDATE) dan menggantikan baris `hasil_tes_detail` lama dengan yang baru di dalam transaksi yang sama.
5. IF transaksi gagal karena `database is locked` (SQLITE_BUSY), THEN THE `Submission_Processor` SHALL mengembalikan error agar `Filesystem_Queue` melakukan retry sesuai Requirement 3.

### Requirement 6: Panic Recovery di Worker

**User Story:** Sebagai admin sekolah, saya ingin satu submission yang menyebabkan panic tidak mematikan worker secara senyap, sehingga submission lainnya tetap diproses.

#### Acceptance Criteria

1. WHEN `Submission_Worker` memanggil `Submission_Processor`, THE `Submission_Worker` SHALL membungkus pemanggilan tersebut dengan `defer recover()`.
2. IF `Submission_Processor` panic saat memproses sebuah `Submission_Job`, THEN THE `Submission_Worker` SHALL mencatat panic message dan stack trace ke log, memindahkan `Job_File` kembali ke `Pending_Dir` (sebagai retry) atau ke `Failed_Dir` (jika `retry_count` mencapai `Max_Retries`), dan melanjutkan loop dequeue tanpa berhenti.
3. WHEN `Submission_Worker` memulihkan dari panic, THE `Submission_Worker` SHALL menambah `retry_count` dan mengisi `last_error` dengan teks panic yang di-recover.
4. THE `Submission_Worker` SHALL tetap menjalankan loop dequeue selama context induk belum dibatalkan, terlepas dari berapa banyak panic yang sudah di-recover.
5. IF context induk dibatalkan saat `Submission_Worker` sedang memproses sebuah `Submission_Job`, THEN THE `Submission_Worker` SHALL berhenti pada iterasi berikutnya dan meninggalkan `Job_File` saat ini di `Processing_Dir` untuk dipulihkan oleh recovery startup pada Requirement 7.
6. IF logging panic di `Submission_Worker` gagal (misal stderr ditutup), THEN THE `Submission_Worker` SHALL melanjutkan pemrosesan submission berikutnya tanpa berhenti.

### Requirement 7: Crash Recovery saat Startup

**User Story:** Sebagai admin sekolah yang harus mematikan paksa server saat listrik mati, saya ingin server saat dijalankan kembali otomatis melanjutkan semua submission yang belum selesai, sehingga tidak ada nilai siswa yang hilang.

#### Acceptance Criteria

1. WHEN `Aether_CBT` startup, THE `Filesystem_Queue` SHALL memindai `Processing_Dir` untuk file yang `mtime`-nya lebih lama dari `Stuck_Threshold`.
2. WHEN `Filesystem_Queue` menemukan file di `Processing_Dir` yang `mtime`-nya melewati `Stuck_Threshold`, THE `Filesystem_Queue` SHALL memindahkan file tersebut ke `Pending_Dir` menggunakan `Atomic_Rename`.
3. THE `Aether_CBT` SHALL menyelesaikan pemindaian recovery startup `Filesystem_Queue` sebelum HTTP server mulai menerima request HTTP apapun (bukan hanya webhook iSpring), dengan menunggu pemindaian sampai selesai tanpa batas waktu.
4. WHEN `Filesystem_Queue` melakukan pemindahan recovery, THE `Filesystem_Queue` SHALL mencatat ke log jumlah file yang dipromosi dan path masing-masing untuk audit.
5. WHERE file di `Tmp_Dir` ditemukan saat startup (sisa enqueue yang gagal di-rename), THE `Filesystem_Queue` SHALL menghapus file tersebut karena dianggap tidak valid.

### Requirement 8: Migrasi dari Tabel submission_queue Lama

**User Story:** Sebagai admin sekolah yang upgrade dari versi sebelumnya, saya ingin job pending di tabel `submission_queue` lama tidak hilang, sehingga submission yang masuk sebelum upgrade tetap diproses.

#### Acceptance Criteria

1. WHEN `Aether_CBT` startup dan tabel `submission_queue` ada di DB, THE `Filesystem_Queue` SHALL membaca semua baris dengan status `pending` atau `processing` dari tabel tersebut.
2. WHEN `Filesystem_Queue` membaca baris dari tabel `submission_queue` lama, THE `Filesystem_Queue` SHALL menulis setiap baris sebagai `Job_File` di `Pending_Dir` mengikuti format JSON Requirement 1.
3. WHEN seluruh baris `pending` dan `processing` dari tabel `submission_queue` sudah berhasil dipindahkan ke `Pending_Dir`, THE `Filesystem_Queue` SHALL menghapus baris-baris tersebut dari tabel `submission_queue`.
4. IF migrasi sebuah baris `submission_queue` gagal (misal write file gagal), THEN THE `Filesystem_Queue` SHALL membatalkan migrasi keseluruhan, mempertahankan baris asli di tabel, dan mencatat error ke log untuk intervensi manual.
5. THE `Filesystem_Queue` SHALL menulis log info berisi jumlah baris yang berhasil dimigrasi dan jumlah baris yang dilewati (status selain pending/processing).

### Requirement 9: Konfigurasi via Environment Variable

**User Story:** Sebagai operator yang men-deploy ke berbagai sekolah, saya ingin lokasi direktori queue dapat dikonfigurasi tanpa rebuild, sehingga saya bisa menempatkannya di drive terpisah jika perlu.

#### Acceptance Criteria

1. WHEN `Aether_CBT` startup, THE `Filesystem_Queue` SHALL membaca path `Queue_Root` dari environment variable `QUEUE_DIR`.
2. WHERE environment variable `QUEUE_DIR` tidak diset atau kosong, THE `Filesystem_Queue` SHALL menggunakan path default `data/queue/` relatif terhadap working directory.
3. IF `Queue_Root` tidak dapat dibuat atau tidak dapat ditulis (misal permission denied), THEN THE `Aether_CBT` SHALL menolak startup dengan exit code non-zero dan mencatat error yang menyebutkan path yang gagal.
4. THE `Filesystem_Queue` SHALL menempatkan `Tmp_Dir` di volume yang sama dengan `Pending_Dir`, `Processing_Dir`, `Done_Dir`, dan `Failed_Dir` agar `Atomic_Rename` valid sebagai operasi atomic.

### Requirement 10: Format Job File Stabil dan Pretty-Printable

**User Story:** Sebagai admin sekolah, saya ingin membuka file di `failed/` dengan Notepad dan langsung memahami isinya tanpa SQL, sehingga saya bisa menentukan apakah perlu retry manual atau menghubungi pengembang.

#### Acceptance Criteria

1. THE `Filesystem_Queue` SHALL menulis `Job_File` sebagai JSON dengan indent 2 spasi dan field dalam urutan tetap: `validasi`, `tenant_id`, `no_id`, `score`, `max_score`, `attempt_token`, `enqueued_at`, `retry_count`, `last_error`, `detail_xml`.
2. THE `Job_File` SHALL menggunakan format ISO 8601 UTC untuk field `enqueued_at` (contoh `2026-05-26T07:30:00Z`).
3. THE `Filesystem_Queue` SHALL men-serialize `detail_xml` sebagai string JSON dengan escape karakter yang benar (newline, quote, dll) sehingga `Job_File` tetap valid JSON walaupun XML berisi karakter khusus.
4. THE `Filesystem_Queue` SHALL menyediakan parser/serializer pasangan (`MarshalJob` dan `UnmarshalJob`) untuk `Submission_Job`.
5. FOR ALL `Submission_Job` valid, marshal-then-unmarshal SHALL menghasilkan struktur yang setara dengan input asli (round-trip property).

### Requirement 11: Pretty Printer untuk Pemeliharaan

**User Story:** Sebagai pengembang yang men-debug submission yang gagal, saya ingin alat untuk meng-encode dan men-decode `Job_File` secara konsisten, sehingga saya bisa membuat fixture test dan memvalidasi format.

#### Acceptance Criteria

1. THE `Filesystem_Queue` SHALL menyediakan fungsi `MarshalJob(job *SubmissionJob) ([]byte, error)` yang menghasilkan representasi byte JSON pretty-printed sesuai Requirement 10.
2. THE `Filesystem_Queue` SHALL menyediakan fungsi `UnmarshalJob(data []byte) (*SubmissionJob, error)` yang menerima byte JSON dan menghasilkan struktur `SubmissionJob`.
3. IF `UnmarshalJob` menerima JSON yang tidak valid atau missing field wajib (`tenant_id`, `no_id`, `validasi`, `enqueued_at`), THEN THE `UnmarshalJob` SHALL mengembalikan error deskriptif yang menyebutkan field yang bermasalah.
4. FOR ALL `Submission_Job` valid, `UnmarshalJob(MarshalJob(job))` SHALL menghasilkan struktur yang setara dengan `job` asli (round-trip property).
5. FOR ALL `Job_File` byte yang dihasilkan oleh `MarshalJob`, parsing dengan parser JSON standar Go (`encoding/json`) SHALL berhasil tanpa error.

### Requirement 12: Endpoint Debug Queue Berbasis Filesystem

**User Story:** Sebagai admin sekolah yang ingin memantau status burst, saya ingin endpoint `/api/debug/queue` melaporkan jumlah file di setiap direktori, sehingga saya bisa memverifikasi semua siswa selesai.

#### Acceptance Criteria

1. WHEN admin melakukan GET ke `Debug_Queue_Endpoint`, THE `Debug_Queue_Endpoint` SHALL mengembalikan JSON dengan field `pending_count`, `processing_count`, `failed_count`, dan `done_count`.
2. THE `Debug_Queue_Endpoint` SHALL menghitung tiap field dengan menjalankan `os.ReadDir` pada `Pending_Dir`, `Processing_Dir`, `Failed_Dir`, dan `Done_Dir` masing-masing dan menjumlahkan entri yang berakhiran `.json`.
3. WHEN `Debug_Queue_Endpoint` menerima request dan total jumlah file di seluruh direktori queue kurang dari atau sama dengan 10000, THE `Debug_Queue_Endpoint` SHALL mengembalikan respons dalam waktu kurang dari 200 milidetik.
4. WHERE total jumlah file di seluruh direktori queue melebihi 10000, THE `Debug_Queue_Endpoint` SHALL tetap mengembalikan respons yang akurat tanpa batas waktu 200 milidetik.
5. IF salah satu direktori tidak dapat dibaca saat sebuah GET request diproses (misal terhapus saat runtime), THEN THE `Debug_Queue_Endpoint` SHALL mengembalikan HTTP 500 dengan body JSON `{"error": "<deskripsi>"}` tanpa crash.
6. THE `Debug_Queue_Endpoint` SHALL memeriksa keterbacaan direktori hanya pada saat memproses GET request, tanpa monitoring proaktif di luar handler.

### Requirement 13: Throughput dan E2E Success Target

**User Story:** Sebagai operator ujian dengan kelas hingga 500 siswa, saya ingin semua nilai tersimpan benar setelah burst submit, sehingga ujian tidak perlu diulang.

#### Acceptance Criteria

1. WHEN harness `tests/load/verify_e2e.go` melakukan burst 500 submission concurrent ke `ISpring_Webhook_Handler` dengan sesi `cek_login` valid, THE `Aether_CBT` SHALL menyimpan 500 baris di `hasil_tes` (E2E success 100%) dalam waktu drain kurang dari 5 menit.
2. THE `Submission_Worker` SHALL memproses minimal 3 job per detik dalam kondisi DB tanpa kontensi eksternal, diukur sebagai rata-rata sepanjang drain 500 job.
3. WHEN harness menjalankan burst 50, 100, atau 200 submission concurrent, THE `Aether_CBT` SHALL menyimpan jumlah baris yang setara di `hasil_tes` dengan 0 file di `Failed_Dir` di setiap skala.
4. WHEN harness menjalankan burst dengan ukuran apapun hingga 500, THE `Filesystem_Queue` SHALL TIDAK meninggalkan file di `Processing_Dir` lebih dari `Stuck_Threshold` setelah drain selesai.
5. WHILE burst 500 sedang masuk dan `Submission_Worker` mengosongkan queue, THE `ISpring_Webhook_Handler` SHALL mempertahankan latency P95 di bawah 100 milidetik agar handler tidak terhambat oleh aktivitas worker.

### Requirement 14: Idempotency pada Retry Webhook

**User Story:** Sebagai pengelola integrasi iSpring yang mungkin di-retry oleh client, saya ingin retry webhook tidak menggandakan baris `hasil_tes`, sehingga laporan nilai tetap akurat.

#### Acceptance Criteria

1. WHEN `ISpring_Webhook_Handler` menerima dua webhook dengan `Validasi_Key` yang sama, THE `Submission_Processor` SHALL menjalankan UPSERT pada `hasil_tes` sehingga hanya ada satu baris dengan `Validasi_Key` tersebut.
2. THE `hasil_tes` SHALL memiliki constraint UNIQUE pada pasangan (`tenant_id`, `validasi`) yang dipertahankan oleh skema DB existing.
3. WHEN UPSERT terjadi pada submission duplikat, THE `Submission_Processor` SHALL menggantikan seluruh baris `hasil_tes_detail` lama dengan yang baru di dalam transaksi yang sama.
4. FOR ALL pasangan webhook duplikat dengan payload identik, jumlah baris di `hasil_tes` setelah pemrosesan kedua SHALL sama dengan jumlah setelah pemrosesan pertama.

### Requirement 15: Test Webhook Mengikuti Kontrak Baru

**User Story:** Sebagai pengembang, saya ingin test suite webhook (`TestISpringWebhookSuccess`, `TestISpringWebhookForbidden`, `TestISpringWebhookRejectsMissingAttemptToken`, `TestISpringGracePeriod`) mencerminkan kontrak handler baru, sehingga regresi terdeteksi sebelum deploy.

#### Acceptance Criteria

1. WHEN test fixture menyiapkan `Aether_CBT`, THE test fixture SHALL menyediakan instance `Filesystem_Queue` yang bekerja di direktori sementara dan terhubung ke `ISpring_Webhook_Handler`.
2. WHEN `TestISpringWebhookSuccess` dijalankan dengan sesi `cek_login` valid dan `attempt_token` cocok, THE test SHALL memverifikasi handler mengembalikan HTTP 200 dan baris `hasil_tes` muncul setelah worker memproses job.
3. WHEN `TestISpringWebhookForbidden` dijalankan tanpa sesi `cek_login` aktif, THE test SHALL memverifikasi handler mengembalikan HTTP 403 dengan body `"active session not found"` dan tidak ada `Job_File` di `Pending_Dir`.
4. WHEN `TestISpringWebhookRejectsMissingAttemptToken` dijalankan dengan `attempt_token` kosong atau tidak cocok, THE test SHALL memverifikasi handler mengembalikan HTTP 403 dengan body `"invalid attempt token"`.
5. WHEN `TestISpringGracePeriod` dijalankan dengan sesi yang sudah melewati `durasi_menit + 5 menit`, THE test SHALL memverifikasi `Submission_Processor` menolak job dan job berakhir di `Failed_Dir` setelah `Max_Retries` habis.

### Requirement 16: Tidak Memutuskan Fungsionalitas Existing

**User Story:** Sebagai admin sekolah, saya ingin login peserta, exam start, progress save, dan pelaporan infraction tetap berjalan tanpa perubahan visible, sehingga upgrade ini tidak mengganggu hari ujian.

#### Acceptance Criteria

1. THE `Aether_CBT` SHALL mempertahankan semua endpoint API existing (selain `/api/ispring/webhook` dan `/api/debug/queue`) dengan kontrak yang identik dengan sebelum upgrade.
2. THE `Aether_CBT` SHALL tetap memakai SQLite WAL untuk seluruh tabel selain `submission_queue` (yang digantikan oleh `Filesystem_Queue`).
3. WHEN `Aether_CBT` startup dengan `Filesystem_Queue` aktif, THE `Aether_CBT` SHALL menjalankan migrasi DB existing tanpa error dan tabel `submission_queue` lama TIDAK dihapus dari skema agar Requirement 8 dapat menjalankan migrasi data.
4. THE `Aether_CBT` SHALL tetap dijalankan sebagai single binary `aether-cbt.exe` tanpa dependensi service eksternal tambahan (Postgres, Redis, broker pesan, dsb.).

### Requirement 17: Single Worker dan Batch Transaction Optimization

**User Story:** Sebagai pengembang yang harus mempertahankan simplicity, saya ingin sistem menggunakan tepat satu worker goroutine sehingga tidak ada concurrent write contention pada SQLite, sementara throughput tetap memadai untuk burst 500 siswa via optimasi transaksi.

#### Acceptance Criteria

1. THE `Aether_CBT` SHALL menjalankan tepat satu instance `Submission_Worker` per proses server.
2. THE `Submission_Worker` SHALL TIDAK memunculkan goroutine pekerja paralel tambahan untuk memproses `Submission_Job`.
3. WHERE optimasi throughput dibutuhkan untuk memenuhi Requirement 13, THE `Submission_Worker` MAY mengelompokkan beberapa `Submission_Job` ke dalam satu transaksi DB tunggal hingga maksimum yang ditentukan dalam fase Design, dengan syarat semua job dalam batch yang sama dimuat secara berurutan dari `Pending_Dir`.
4. IF batching diadopsi dan satu job dalam batch gagal diproses, THEN THE `Submission_Worker` SHALL melakukan rollback transaksi dan memindahkan setiap job dalam batch tersebut kembali ke `Pending_Dir` dengan increment `retry_count` dan `last_error` masing-masing sesuai Requirement 3.
5. THE `Submission_Worker` SHALL TIDAK memerlukan koordinasi inter-process (file lock, named mutex, advisory lock DB) karena `Aether_CBT` dijalankan sebagai single binary single proses.
