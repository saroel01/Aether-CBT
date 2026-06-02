# Design Document

Penjadwalan Ujian Detail & Pengiriman Konten iSpring — Aether CBT

## Overview

Dokumen ini merancang penambahan dua kapabilitas yang saling bergantung di atas fondasi Aether CBT yang sudah ada (Go/Fiber + SQLite WAL + SvelteKit, multi-tenant, JWT + role middleware, filesystem submission queue):

1. **Penjadwalan ujian detail** — tingkatan kelas, definisi ujian, dan sesi (gelombang) berjendela waktu dengan token per-sesi, lengkap dengan penegakan di sisi server.
2. **Pengiriman konten iSpring** — unggah paket HTML5 hasil ekspor QuizMaker, penyimpanan terisolasi per tenant, penautan ke ujian, penyajian terotorisasi, dan shim klien yang mengalihkan pengiriman hasil ke webhook internal tanpa URL hardcoded.

Design menjaga prinsip yang sudah berlaku di repo: migrasi idempoten (lihat `internal/db/migrate.go`), akses data melalui lapisan repository (pola `internal/repository`), pemisahan handler HTTP dari logika data (mencegah god-file), keandalan pengiriman hasil via queue (`internal/submission`), dan isolasi tenant ketat.

### Fakta kode yang menjadi dasar design (terverifikasi)

- `internal/db/migrate.go` menjalankan ulang **semua** berkas `.sql` pada setiap startup secara leksikal dan **tidak** memiliki tabel pelacak migrasi. Idempotensi dijamin oleh konvensi (`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `INSERT OR IGNORE`) dan oleh penelan error untuk `duplicate column name`/`already exists`/`SQLITE_BUSY`. **Semua migrasi baru wajib mengikuti pola ini.**
- `internal/db/sqlite.go` membuka SQLite dengan `_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000` namun **tidak** memanggil `SetMaxOpenConns/SetMaxIdleConns` pada jalur server (hanya di test/benchmark). Ini titik perbaikan untuk Requirement 13.
- `internal/api/middleware/auth.go` menaruh `user_id`, `tenant_id`, `role` ke `c.Locals` dari klaim JWT. `middleware.RequireRoles(...)` menegakkan akses berbasis peran. `TenantMiddleware` mengisi `tenant_id` dari header/slug/subdomain dan menolak di produksi bila tak ada.
- Webhook `POST /api/ispring/webhook` (publik, di luar grup terproteksi) memvalidasi sesi aktif + `attempt_token` (403), memvalidasi XML via `internal/ispring`, lalu `Enqueue` ke filesystem queue; worker memproses batch dalam satu transaksi dan menulis `hasil_tes` + `hasil_tes_detail`, lalu menghapus `cek_login`.
- `validasi` saat ini berformat `tenant_id_noID_mapelID` dan dipakai sebagai kunci UPSERT pada indeks unik `hasil_tes(tenant_id, validasi)`.
- `cek_login` memiliki indeks unik `(tenant_id, peserta_id, mapel_id)` (migrasi 017), kolom `attempt_token` (018), `mapel_id`, `tab_switch_count` (012), `answered_count`, `total_questions` (015).
- Frontend siswa (`web/src/routes/student/exam/+page.svelte`) saat ini meng-generate soal hardcoded dan XML iSpring tiruan; ini akan digantikan oleh penyajian paket nyata.
- Sampel fixture `contoh_soal/KIMIA_XII_UAS_2025 (Published)` **tidak lengkap**: hanya `index.html` (≈1.5 MB, memuat data quiz base64 dan memanggil `QuizPlayer.start` dari `data/player.js`), `ismplayer.html`, `metainfo.xml`, `preview.png`. Folder `data/` (termasuk `player.js`) yang dirujuk `metainfo.xml` tidak ada. Implikasi pada strategi pengujian dijelaskan di bagian Testing.

---

## Architecture

### Diagram alur tingkat tinggi

```
Admin (JWT admin)                          Siswa (JWT student)
  │                                          │
  │ kelola tingkat/ujian/sesi/paket          │ login (token sesi) ─► JWT student
  ▼                                          ▼
┌───────────────────────────┐        ┌──────────────────────────────┐
│ Admin API (terproteksi)   │        │ Student API (terproteksi)    │
│  - GradeLevel/Exam/Session│        │  - validasi sesi efektif     │
│  - Soal package upload    │        │  - StartSession ► attempt_tkn│
│    & linking              │        │    + content session cookie  │
└───────────┬───────────────┘        └───────────────┬──────────────┘
            │                                         │
            ▼                                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ Service layer (logika penjadwalan, otorisasi sesi, paket)         │
└───────────┬───────────────────────────────┬───────────────────────┘
            │                                │
            ▼                                ▼
┌───────────────────────────┐     ┌─────────────────────────────────┐
│ Repository layer          │     │ Package storage (filesystem)    │
│ (SQLite, tenant-scoped)   │     │ data/soal/{tenant_slug}/{uuid}/ │
└───────────────────────────┘     └─────────────────────────────────┘
            │                                │
            ▼                                ▼ (penyajian + shim injeksi)
┌───────────────────────────┐     ┌─────────────────────────────────┐
│ SQLite (WAL)              │     │ Player iSpring di browser siswa │
└───────────────────────────┘     └───────────────┬─────────────────┘
                                                   │ POST hasil (di-shim)
                                                   ▼
                                   POST /api/ispring/webhook ─► queue ─► worker ─► hasil_tes
```

### Keputusan arsitektur utama (dan alasannya)

**AD-1. Model sesi menggantikan token global lama (dengan migrasi data).**
Token global `settings.token` + `is_exam_active` digantikan oleh token per-sesi pada `exam_session`. Untuk kompatibilitas mundur (Requirement 14.3), migrasi data membuat satu `exam` + satu `exam_session` "warisan" dari konfigurasi `settings` yang ada per tenant sehingga instalasi lama tidak langsung rusak. Kolom `settings.token`/`is_exam_active` **tidak dihapus** (dipertahankan untuk pembacaan transisi dan menghindari kerusakan migrasi idempoten), tetapi tidak lagi menjadi sumber kebenaran untuk login. Disetujui pengguna.

**AD-2. Otorisasi penyajian konten memakai cookie sesi konten, bukan Bearer JWT.**
Player iSpring memuat sub-aset (`data/player.js`, font, gambar) lewat tag HTML biasa yang **tidak** mengirim header `Authorization`. Karena itu penyajian konten tidak bisa bergantung pada `AuthMiddleware`. Sebagai gantinya:
- Endpoint terotorisasi (Bearer JWT siswa) `POST /api/student/start` mengeset **cookie sesi konten** `aether_exam` (HttpOnly, SameSite=Strict, Secure bila HTTPS, Path=`/api/exam/content`) yang berisi token konten acak yang dipetakan ke `cek_login` (sesi aktif).
- Endpoint penyajian `GET /api/exam/content/*` memvalidasi cookie → sesi aktif → tenant → jendela waktu → tidak terkunci, lalu melakukan streaming berkas.
Ini menyelesaikan masalah sub-aset tanpa menaruh rahasia di URL (lebih aman daripada token-in-path), dan tetap kompatibel LAN/online karena cookie dikirim same-origin.

**AD-3. Shim disuntikkan saat penyajian, hanya pada `index.html` (entry), tanpa mengubah berkas di disk.**
Saat `index.html` paket di-stream, server menyisipkan blok `<script>` shim sebelum `<script src="data/player.js...">`. Shim mencegat lapisan jaringan browser (`XMLHttpRequest.open/send`, `fetch`, `navigator.sendBeacon`, dan submit `<form>`) dan mengarahkan ulang setiap pengiriman yang membawa field hasil iSpring (`dr`/`sp`/`tp`) ke `POST /api/ispring/webhook` relatif same-origin, sambil menyuntikkan `attempt_token`, `tenant_id`, dan `sid`. Berkas paket di disk tetap utuh (audit). Karena `index.html` bisa berukuran besar (≈1.5 MB), injeksi dilakukan secara streaming dengan pemindaian penanda penyisipan, bukan memuat seluruh berkas lalu mengganti string di memori untuk setiap request (lihat Performa).

**AD-4. Penyajian konten memakai streaming berkas (bukan baca penuh ke memori).**
Untuk sub-aset (non-index) gunakan `c.SendFile`/streaming Fiber. Untuk `index.html` gunakan penulisan bertahap: stream bagian sebelum titik sisip, tulis shim, stream sisanya. Ini memenuhi Requirement 8.6 dan 13.3.

**AD-5. Konfigurasi connection pool eksplisit untuk SQLite WAL.**
Tetapkan `SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime` pada `db.Connect`. Karena WAL mengizinkan banyak pembaca tetapi satu penulis pada satu waktu, dan `_busy_timeout=5000` sudah aktif, kita menetapkan pool yang memadai untuk pembacaan paralel sembari mengandalkan busy_timeout untuk serialisasi penulisan. Penulisan hasil sudah diserialkan oleh worker queue tunggal. Beban tulis progres dikurangi via debounce (AD-6). Nilai dapat dikonfigurasi via environment dengan default aman.

**AD-6. Progres siswa di-debounce di klien dan idempoten di server.**
`POST /api/student/progress` saat ini menulis DB tiap klik. Diubah: klien menahan (debounce) dan mengirim progres secara berkala (mis. saat pindah soal atau interval), server melakukan satu `UPDATE` ringan pada `cek_login`. Ini memangkas beban tulis pada skala 500 siswa (Requirement 13.2).

**AD-7. Akses data baru lewat repository + service; handler tipis (anti god-file).**
Entitas baru mendapat repository sendiri di `internal/repository`, logika lintas-entitas di `internal/service`, dan handler hanya mengurus HTTP. Penyimpanan/penyajian paket dipisah ke paket `internal/soalpkg`. Tidak ada satu berkas yang menggabungkan upload + serve + schedule + monitor.

---

## Components and Interfaces

### 1. Skema database (migrasi baru, idempoten)

Semua migrasi mengikuti pola `RunMigrations`. Penomoran melanjutkan urutan yang ada (terakhir `019_create_submission_queue.sql`).

#### `020_alter_kelas_tingkat.sql`
```sql
-- Tambah tingkatan pada kelas (idempoten: error "duplicate column name" ditelan RunMigrations)
ALTER TABLE kelas ADD COLUMN tingkat TEXT;
CREATE INDEX IF NOT EXISTS idx_kelas_tingkat ON kelas(tenant_id, tingkat);
```

#### `021_create_soal_package.sql`
```sql
CREATE TABLE IF NOT EXISTS soal_package (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    nama TEXT NOT NULL,
    package_uuid TEXT NOT NULL,          -- nama folder di data/soal/{slug}/{uuid}
    entry_path TEXT NOT NULL DEFAULT 'index.html',
    ispring_version TEXT,                -- best-effort dari komentar header; NULL bila tak terdeteksi
    total_size INTEGER NOT NULL DEFAULT 0,
    checksum TEXT,                       -- sha256 arsip terunggah (audit/dedup)
    uploaded_by INTEGER,                 -- users.id
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    UNIQUE(tenant_id, package_uuid)
);
CREATE INDEX IF NOT EXISTS idx_soal_package_tenant ON soal_package(tenant_id);
```

#### `022_create_exam.sql`
```sql
CREATE TABLE IF NOT EXISTS exam (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    mapel_id INTEGER NOT NULL,
    tingkat TEXT,
    soal_package_id INTEGER,             -- boleh NULL saat draft
    durasi_menit INTEGER NOT NULL DEFAULT 90,
    kkm REAL NOT NULL DEFAULT 0,
    shuffle_questions BOOLEAN NOT NULL DEFAULT FALSE,
    shuffle_answers BOOLEAN NOT NULL DEFAULT FALSE,
    nama TEXT,                           -- label tampil opsional (mis. "UAS Kimia XII")
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (mapel_id) REFERENCES mapel(id),
    FOREIGN KEY (soal_package_id) REFERENCES soal_package(id)
);
CREATE INDEX IF NOT EXISTS idx_exam_tenant ON exam(tenant_id);
CREATE INDEX IF NOT EXISTS idx_exam_mapel ON exam(tenant_id, mapel_id);
```

#### `023_create_exam_session.sql`
```sql
CREATE TABLE IF NOT EXISTS exam_session (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_id INTEGER NOT NULL,
    nama TEXT,                           -- label gelombang (mis. "Sesi 1")
    waktu_mulai DATETIME NOT NULL,
    waktu_selesai DATETIME NOT NULL,
    token TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK(status IN ('draft','terjadwal','aktif','selesai','dibatalkan')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (exam_id) REFERENCES exam(id)
);
CREATE INDEX IF NOT EXISTS idx_exam_session_tenant ON exam_session(tenant_id);
CREATE INDEX IF NOT EXISTS idx_exam_session_exam ON exam_session(exam_id);
-- Keunikan token diberlakukan di lapisan aplikasi terhadap sesi yang jendela waktunya
-- tumpang tindih (lihat catatan keunikan token). Indeks bantu pencarian token:
CREATE INDEX IF NOT EXISTS idx_exam_session_token ON exam_session(tenant_id, token);
```

> **Catatan keunikan token (Requirement 4.4):** keunikan absolut `(tenant_id, token)` terlalu kaku karena token sesi lama yang sudah `selesai` boleh dipakai ulang. Penegakan dilakukan di service: saat membuat/menyunting sesi, tolak bila ada sesi lain dengan token sama yang jendela waktunya tumpang tindih. Indeks `idx_exam_session_token` mempercepat pengecekan ini.

#### `024_create_session_kelas_ruang.sql`
```sql
CREATE TABLE IF NOT EXISTS exam_session_kelas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    kelas_id INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES exam_session(id) ON DELETE CASCADE,
    FOREIGN KEY (kelas_id) REFERENCES kelas(id),
    UNIQUE(session_id, kelas_id)
);
CREATE TABLE IF NOT EXISTS exam_session_ruang (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL,
    ruang_id INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES exam_session(id) ON DELETE CASCADE,
    FOREIGN KEY (ruang_id) REFERENCES ruang(id),
    UNIQUE(session_id, ruang_id)
);
CREATE INDEX IF NOT EXISTS idx_session_kelas_session ON exam_session_kelas(session_id);
CREATE INDEX IF NOT EXISTS idx_session_ruang_session ON exam_session_ruang(session_id);
```

#### `025_alter_cek_login_session.sql`
```sql
-- Tautkan sesi aktif ke exam_session dan tambahkan penguncian server-side + token konten
ALTER TABLE cek_login ADD COLUMN session_id INTEGER;
ALTER TABLE cek_login ADD COLUMN locked INTEGER NOT NULL DEFAULT 0;
ALTER TABLE cek_login ADD COLUMN content_token TEXT;
-- Indeks unik sesi-berbasis menggantikan basis mapel.
-- Index lama (017) di-drop agar dua sesi berbeda tidak terganjal kunci lama.
DROP INDEX IF EXISTS idx_cek_login_unique_exam_session;
CREATE UNIQUE INDEX IF NOT EXISTS idx_cek_login_unique_session
    ON cek_login(tenant_id, peserta_id, session_id);
CREATE INDEX IF NOT EXISTS idx_cek_login_content_token ON cek_login(content_token);
```

> **Catatan perubahan `validasi` (Requirement 14.2):** kunci `hasil_tes.validasi` berubah dari `tenant_id_noID_mapelID` menjadi `tenant_id_noID_sessionID`. Struktur indeks unik `hasil_tes(tenant_id, validasi)` **tidak berubah** (tetap satu hasil per kunci), hanya komposisi string-nya. Hasil lama tetap terbaca karena kolom dan indeks identik; hasil baru memakai format berbasis sesi. Tidak ada penghapusan kolom.

#### `026_seed_legacy_session.sql` (migrasi data kompatibilitas)
```sql
-- Best-effort: untuk tiap tenant yang punya settings token tapi belum punya exam_session,
-- buat satu exam + exam_session "warisan" agar instalasi lama tetap berfungsi.
-- Ditulis idempoten dengan INSERT ... WHERE NOT EXISTS. Detail diselesaikan saat implementasi
-- (mis. memilih mapel default/placeholder); jika tidak memungkinkan tanpa ambiguitas,
-- langkah ini menjadi util migrasi di kode, bukan SQL murni (lihat Tasks).
```

> Migrasi data warisan yang aman bisa jadi memerlukan logika (mis. memilih mapel) yang tidak elok sebagai SQL idempoten murni. Keputusan implementasi: jika tidak dapat dilakukan secara deterministik dan idempoten via SQL, pindahkan ke fungsi migrasi data di Go yang dipanggil saat startup setelah `RunMigrations`, dengan penjagaan "jalankan hanya bila belum ada sesi". Ini ditegaskan sebagai task tersendiri agar tidak menimbulkan tech debt tersembunyi.

### 2. Lapisan repository (`internal/repository`)

Tambahan repository (mengikuti pola `tenant_repo.go`, semua tenant-scoped):

- `grade_repo.go` — operasi `tingkat` pada kelas (update tingkat, list distinct tingkat).
- `soal_package_repo.go` — CRUD metadata `soal_package`.
- `exam_repo.go` — CRUD `exam` + validasi referensi mapel/paket.
- `exam_session_repo.go` — CRUD `exam_session`, relasi kelas/ruang, pencarian token aktif, cek tumpang tindih token.
- `cek_login_repo.go` — operasi sesi aktif berbasis `session_id` (start/lock/unlock/progress/lookup), menggantikan SQL tersebar.

Setiap repository menerima `*sql.DB` global (`db.DB`) konsisten dengan pola sekarang, mengembalikan model dari `internal/models`. Tiap kueri menyertakan filter `tenant_id`.

### 3. Lapisan service (`internal/service`)

- `scheduling_service.go` — aturan lintas-entitas: validasi jendela waktu, status efektif sesi (dihitung dari waktu server), keunikan token tumpang tindih, kelayakan peserta (keanggotaan kelas/ruang), transisi status sesi (tolak `terjadwal/aktif` bila paket belum tertaut).
- `content_session_service.go` — penerbitan/validasi `content_token` (cookie), penegakan jendela waktu + lock saat penyajian konten.

### 4. Paket penyimpanan & penyajian soal (`internal/soalpkg`)

- `storage.go` — ekstraksi ZIP aman (anti zip-slip), validasi `index.html`, deteksi versi best-effort, penulisan ke `data/soal/{slug}/{uuid}/`, perhitungan ukuran/checksum, pembersihan saat gagal, penghapusan paket.
- `serve.go` — resolusi path aman (mencegah traversal keluar direktori paket), streaming berkas, dan injeksi shim pada entry `index.html`.
- `shim.go` — penyediaan konten shim JS (di-embed via `go:embed` dari `internal/soalpkg/assets/ispring-shim.js`) dan logika titik sisip.

Validasi anti zip-slip: untuk setiap entri, `target := filepath.Join(dest, entry.Name)`; tolak bila `!strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), filepath.Clean(dest)+string(os.PathSeparator))`. Batas ukuran total dan jumlah berkas diberlakukan untuk mencegah zip bomb (Requirement 3.2).

### 5. Endpoint HTTP (handler tipis)

Admin (role `admin`/`superadmin`, di grup `protected`):

| Method | Path | Fungsi |
| --- | --- | --- |
| PUT | `/api/classes/:id/tingkat` | Set tingkat kelas |
| GET | `/api/admin/soal-packages` | Daftar paket tenant |
| POST | `/api/admin/soal-packages/upload` | Unggah ZIP iSpring (multipart) |
| DELETE | `/api/admin/soal-packages/:id` | Hapus paket tak tertaut |
| GET/POST | `/api/admin/exams` | List/buat definisi ujian |
| PUT/DELETE | `/api/admin/exams/:id` | Sunting/hapus ujian |
| GET/POST | `/api/admin/exam-sessions` | List/buat sesi |
| PUT/DELETE | `/api/admin/exam-sessions/:id` | Sunting/hapus sesi |
| POST | `/api/admin/exam-sessions/:id/classes` | Tautkan kelas |
| POST | `/api/admin/exam-sessions/:id/rooms` | Tautkan ruang |

Siswa & publik:

| Method | Path | Auth | Fungsi |
| --- | --- | --- | --- |
| POST | `/api/auth/student-login` | publik | Login + validasi token sesi (diperluas) |
| GET | `/api/student/my-sessions` | JWT student | Sesi yang berhak diikuti saat ini |
| POST | `/api/student/start` | JWT student | Mulai sesi, set `attempt_token` + cookie konten (diperluas) |
| POST | `/api/student/progress` | JWT student | Progres (debounced, idempoten) |
| POST | `/api/student/infraction` | JWT student | Catat pelanggaran; lock server bila ambang tercapai |
| GET | `/api/student/remaining-time` | JWT student | Sisa waktu = min(durasi, akhir_sesi − now) |
| GET | `/api/exam/content/*` | cookie konten | Penyajian paket iSpring + shim |
| POST | `/api/ispring/webhook` | publik (rate-limited) | Penerimaan hasil (tetap, validasi sesi+attempt_token) |

Pengawas (role `supervisor`/`admin`): endpoint monitoring/reset yang ada diperluas agar berbasis `session_id` (Requirement 11).

### 6. Shim klien iSpring (`internal/soalpkg/assets/ispring-shim.js`)

Disuntikkan ke `index.html` saat penyajian. Tanggung jawab:

1. Membaca konteks dari variabel global yang ditulis server di atas tag shim:
   `window.__AETHER__ = { webhook: "/api/ispring/webhook", attemptToken, tenantId, sid };`
2. Override `XMLHttpRequest.prototype.open/send`, `window.fetch`, `navigator.sendBeacon`, dan intersepsi `HTMLFormElement.prototype.submit` + event `submit`.
3. Deteksi payload hasil iSpring: bila body (urlencoded/FormData) memuat kunci hasil (`dr`/`sp`/`tp`), alihkan tujuan ke `window.__AETHER__.webhook` (relatif) dan tambahkan/timpa `attempt_token`, `tenant_id`, `sid`.
4. Teruskan status gagal/sukses agar perilaku konsisten dengan webhook+queue (Requirement 9.4).

> **Risiko terverifikasi:** detail internal `player.js` bersifat proprietary dan berbeda antar versi; fixture sampel tidak menyertakan `player.js`. Karena itu shim sengaja mencegat **lapisan jaringan browser** (bukan internal player), sehingga tahan terhadap variasi versi. Verifikasi penuh tetap memerlukan paket lengkap nyata (lihat Testing & Tasks).

### 7. Perubahan konfigurasi (environment)

| Variabel | Default | Fungsi |
| --- | --- | --- |
| `DB_MAX_OPEN_CONNS` | mis. 1 untuk penulisan aman + pool baca terpisah, atau nilai moderat (final di Tasks) | Batas koneksi pool |
| `DB_MAX_IDLE_CONNS` | sama/atau lebih kecil | Idle pool |
| `SOAL_UPLOAD_MAX_BYTES` | mis. 100 MB | Batas ukuran ZIP |
| `SOAL_PACKAGE_MAX_FILES` | mis. 5000 | Anti zip bomb |
| `ANTICHEAT_LOCK_THRESHOLD` | 3 | Ambang lock pelanggaran |
| `CONTENT_COOKIE_SECURE` | auto/true di produksi | Flag Secure cookie konten |

Dimuat lewat `internal/config/config.go` (mengikuti pola `getEnv`).

---

## Data Models

Struct Go baru di `internal/models` (ringkas; field mengikuti kolom migrasi):

- `GradeLevelInfo` (atau perluasan model kelas dengan `Tingkat *string`).
- `SoalPackage` { ID, TenantID, Nama, PackageUUID, EntryPath, IspringVersion *string, TotalSize, Checksum, UploadedBy *int, timestamps, DeletedAt }.
- `Exam` { ID, TenantID, MapelID, Tingkat *string, SoalPackageID *int, DurasiMenit, KKM, ShuffleQuestions, ShuffleAnswers, Nama *string, timestamps, DeletedAt }.
- `ExamSession` { ID, TenantID, ExamID, Nama *string, WaktuMulai, WaktuSelesai, Token, Status, timestamps, DeletedAt }.
- `ExamSessionKelas`, `ExamSessionRuang` (relasi).
- Perluasan representasi sesi aktif (`cek_login`) dengan `SessionID *int`, `Locked bool`, `ContentToken *string`.

Status efektif sesi dihitung (bukan disimpan) oleh service:
```
efektif_dapat_dimasuki = (status ∈ {terjadwal, aktif})
                         AND (now ∈ [waktu_mulai, waktu_selesai])
```

---

## Error Handling

- **Validasi input** dikembalikan sebagai HTTP 400 dengan pesan jelas (mengikuti `utils.ErrorResponse`).
- **Otorisasi**: 401 (tak login/token sesi salah), 403 (peran salah / bukan pemilik sesi / sesi terkunci / di luar jendela waktu).
- **Upload**: 413/400 untuk ukuran berlebih; 400 untuk ZIP tidak valid/zip-slip/tanpa `index.html`; cleanup parsial dijamin.
- **Penyajian konten**: 403 untuk sesi tidak valid; 404 untuk berkas tak ada; 400 untuk path traversal.
- **Penghapusan** ujian/paket yang masih tertaut: 409/400 dengan pesan alasan.
- **iSpring webhook**: perilaku 400/403/500 yang ada dipertahankan; perubahan hanya pada penurunan kunci `validasi` ke basis sesi.
- **Pembedaan waktu sesi** (Requirement 6.3): "sesi belum dimulai" vs "sesi telah berakhir" dikembalikan sebagai pesan berbeda.
- Seluruh error dilog tanpa membocorkan rahasia (tidak mencetak `attempt_token`/`content_token`/isi cookie).

---

## Testing Strategy

Mengikuti gaya pengujian repo yang ada (Go `testing`, test berdampingan dengan kode; properti/benchmark sudah dipakai di `internal/submission`).

### Unit & integrasi (Go)
- **Migrasi**: perluas `internal/db/migrate_test.go` — verifikasi seluruh migrasi (lama+baru) sukses pada DB kosong dan DB existing (rerun idempoten); verifikasi indeks unik `cek_login(tenant_id, peserta_id, session_id)` dan keberadaan kolom baru.
- **Repository**: test CRUD tenant-scoped untuk exam/session/package/grade; pastikan kebocoran lintas-tenant nihil.
- **Scheduling service**: status efektif (batas waktu tepat), penolakan token tumpang tindih, kelayakan peserta, penolakan transisi tanpa paket.
- **soalpkg/storage**: anti zip-slip (entri `../`), penolakan non-ZIP, penolakan tanpa `index.html`, batas ukuran/jumlah berkas, cleanup saat gagal, deteksi versi best-effort (pakai header `index.html` fixture KIMIA), isolasi path per tenant.
- **soalpkg/serve**: resolusi path aman, penolakan traversal, injeksi shim hanya pada `index.html` dan menghasilkan markup yang benar.
- **shim**: karena shim adalah JS, uji di sisi Go sebatas memastikan konten ter-embed dan titik sisip benar; logika JS diuji manual/headless (lihat di bawah).
- **Anti-cheat**: lock server pada ambang, penolakan start/serve saat terkunci, unlock oleh pengawas; grace-period existing tetap lulus.
- **Student auth flow**: perluas `student_auth_flow_test.go` untuk token berbasis sesi (di dalam/di luar jendela waktu, sesi belum dibuka/berakhir).
- **Webhook**: pastikan `validasi` berbasis sesi tetap UPSERT benar di `hasil_tes`; uji idempotensi kiriman ulang.

### Frontend (SvelteKit)
- `npm run build` wajib lulus (Requirement 16.5).
- Halaman ujian siswa beralih dari simulator hardcoded ke `<iframe>`/embed konten dari `/api/exam/content/...`; uji bahwa progres/infraction tetap terkirim.

### Verifikasi shim end-to-end (fixture)
- **Keterbatasan terverifikasi**: fixture `KIMIA_XII_UAS_2025` tidak lengkap (tanpa `data/player.js`), sehingga uji runtime penuh tidak mungkin hanya dari fixture ini. Task khusus: sediakan paket iSpring **lengkap** (ekspor ulang/di-zip utuh) di lokasi fixture pengujian, lalu jalankan uji headless (proyek sudah punya `puppeteer` sebagai devDependency) untuk memastikan: konten dimuat, dan pengiriman hasil dialihkan ke `/api/ispring/webhook` dengan `attempt_token`/`tenant_id`/`sid`. Bila paket lengkap belum tersedia, langkah ini didokumentasikan sebagai verifikasi manual wajib sebelum hari-H (sejalan dengan utang fixture iSpring yang sudah dicatat proyek).

### Uji beban (Requirement 13)
- Manfaatkan `tests/load/` untuk mensimulasikan ~500 peserta: burst login, start sesi, progres (setelah debounce), submit hasil. Verifikasi tidak ada kehilangan hasil (jaminan queue) dan latensi penyajian konten wajar.

### Gerbang kualitas (Requirement 16.5)
`go build ./...`, `go vet ./...`, `go test ./...`, dan `npm run build` harus lulus sebelum pekerjaan dianggap selesai.

---

## Correctness Properties

Invariant berikut harus selalu benar dan menjadi dasar pengujian (sebagian cocok untuk property-based testing dengan `pgregory.net/rapid` yang sudah dipakai di `internal/submission`):

### Property 1: Isolasi tenant pada entitas baru
Untuk setiap operasi baca/tulis pada entitas baru (exam, exam_session, soal_package, relasi), hasil yang dikembalikan/termodifikasi SHALL hanya milik `tenant_id` konteks permintaan. Tidak ada nilai input yang dapat menyebabkan kebocoran lintas-tenant.

**Validates: Requirements 15.1, 15.2**

### Property 2: Isolasi path penyimpanan paket
Path penyimpanan paket selalu berada di dalam `data/soal/{tenant_slug}/`; tidak ada `package_uuid` atau nama berkas yang dapat menghasilkan path di luar direktori tenant.

**Validates: Requirements 3.5, 15.3**

### Property 3: Ekstraksi ZIP aman dan bersih
Untuk sembarang isi arsip ZIP, ekstraksi tidak pernah menulis berkas di luar direktori tujuan paket (anti zip-slip), dan kegagalan di tengah selalu meninggalkan disk bersih (tidak ada paket parsial yang tercatat sebagai valid).

**Validates: Requirements 3.3, 3.7**

### Property 4: Penyajian berkas terbatas di direktori paket
Untuk sembarang path permintaan ke `/api/exam/content/*`, berkas yang disajikan selalu berada di dalam direktori paket sesi tersebut; permintaan yang resolusinya keluar direktori selalu ditolak.

**Validates: Requirements 8.3**

### Property 5: Kelayakan masuk sesi
Sebuah sesi dapat dimasuki jika dan hanya jika status ∈ {terjadwal, aktif} DAN waktu server ∈ [waktu_mulai, waktu_selesai]. Tidak ada jalur yang mengizinkan masuk di luar kondisi ini.

**Validates: Requirements 4.5, 6.1, 6.3, 7.3**

### Property 6: Sisa waktu terbatas dan non-negatif
Sisa waktu siswa selalu = `min(durasi_ujian, waktu_selesai_sesi − now)` dan tidak pernah negatif (di-clamp ke 0).

**Validates: Requirements 7.5**

### Property 7: Sesi aktif tunggal per peserta-sesi
Maksimal satu sesi aktif per `(tenant_id, peserta_id, session_id)` (dijamin indeks unik); upaya start ganda menghasilkan pembaruan idempoten, bukan duplikat.

**Validates: Requirements 7.1, 7.2**

### Property 8: Tidak ada token sesi tumpang tindih
Tidak ada dua sesi dengan token sama yang jendela waktunya tumpang tindih dalam satu tenant.

**Validates: Requirements 4.4**

### Property 9: UPSERT hasil idempoten
Untuk kunci `validasi = tenant_id_noID_sessionID`, pemrosesan hasil selalu UPSERT tepat satu baris `hasil_tes`; pengiriman ulang dengan kunci sama tidak pernah membuat baris ganda.

**Validates: Requirements 14.2**

### Property 10: Tidak ada kehilangan hasil
Hasil yang sudah diterima webhook tidak pernah hilang akibat lonjakan beban (dijamin rename atomic + retry + recovery pada filesystem queue yang ada).

**Validates: Requirements 13.4, 13.6**

### Property 11: Penguncian server otoritatif
Bila sesi aktif `locked = 1`, maka start/lanjut/penyajian konten selalu ditolak (403), terlepas dari state frontend; hanya aksi pengawas yang dapat membuka kunci.

**Validates: Requirements 10.2, 10.3, 10.4**

### Property 12: Berkas paket tidak berubah saat disajikan
Berkas paket di disk tidak pernah berubah akibat penyajian; injeksi shim hanya terjadi pada aliran keluaran entry `index.html`, bukan pada berkas tersimpan.

**Validates: Requirements 9.5**

---

## Catatan Migrasi & Kompatibilitas

- Semua migrasi baru idempoten dan aman di-rerun (pola `RunMigrations`).
- Tidak ada kolom/tabel existing yang dihapus. `settings.token`/`is_exam_active` dipertahankan untuk transisi.
- Perubahan kunci `validasi` ke basis sesi tidak mengubah struktur indeks `hasil_tes`; hasil lama tetap terbaca.
- Index unik `cek_login` berpindah dari basis mapel ke basis sesi via `DROP INDEX IF EXISTS` + `CREATE UNIQUE INDEX IF NOT EXISTS` (idempoten).
- Migrasi data sesi warisan dijalankan secara penjagaan "hanya bila belum ada sesi" agar tidak menduplikasi pada rerun.
