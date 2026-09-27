# Laporan Audit Mendalam - Aether CBT

| | |
|---|---|
| **Repositori** | `D:\Projects\Aether-CBT` (`github.com/saroel01/Aether-CBT`, visibility: **public**) |
| **Tanggal audit** | 26 September 2026 |
| **Commit baseline** | `f6b05ff` - *feat(ui): comprehensive UI/UX redesign and enterprise resilience hardening* |
| **Ruang lingkup** | Backend Go 1.25 + Fiber v2 + SQLite (WAL), frontend SvelteKit 2 / Svelte 5 / Vite 5 / Tailwind 3, queue filesystem, integrasi iSpring, Docker/build/deploy, dokumentasi operasional |
| **Perubahan kode** | **Nol.** Audit read-only. Tidak ada file source yang dimodifikasi. |
| **Status working tree** | 3 file belum di-commit: `web/src/app.css`, `web/src/app.html`, `web/src/routes/+page.svelte`. Build `web/build` yang releasing tidak tentu sama dengan `src`. |

---

## 1. Ringkasan Eksekutif

Kualitas kode ini **jauh di atas rata-rata**: 58 file test Go (termasuk property test `pgregory.net/rapid`), queue filesystem dengan fsync + atomic rename + recovery, tenant isolation yang konsisten di level query, score verification lintas batch dengan SAVEPOINT per job, dan dokumentasi operasional yang sangat detail. Semua itu benar-benar bekerja: `go build`, `go vet`, dan `go test -count=1 ./...` lulus seluruhnya.

Masalahnya bukan di kualitas implementasi, melainkan di **area yang tidak pernah diuji** dan **asumsi yang tidak pernah ditagih**. Audit ini menemukan **5 temuan P0, 15 temuan P1, 21 temuan P2, dan 12 temuan P3**. Empat di antaranya adalah kegagalan aktif, bukan risiko teoretis:

1. **Skor ujian sepenuhnya dikendalikan siswa.** Field `dr` (XML detail) bersifat opsional; ketika ada, isinya juga dibuat oleh siswa; `skor_maks` diambil apa adanya dari `tp`. Rantai ini membatalkan seluruh nilai penilaian.
2. **Hasil ujian bisa ditimpa kapan saja, dan nilai esai yang sudah dinilai guru bisa terhapus**, karena processor bersifat last-writer-wins dan memakai strategi hapus-lalu-sisip pada tabel detail.
3. **Kunci JWT produksi tertanam di `Dockerfile` yang dipublikasikan**, dan image berjalan dalam mode `development` sehingga semua request tanpa header tenant jatuh ke tenant 1.
4. **Backup tidak pernah diverifikasi**: `integrity_check` dijalankan pada database sumber, bukan pada file backup yang baru ditulis.

Yang membedakan laporan ini dari sekadar daftar temuan statis: setiap butir P0 dan P1 menyertakan pemicu yang dapat dieksekusi, dampak, lokasi perbaikan, dan bentuk regression test. Test suite yang sekarang hijau **tidak akan menangkap satu pun dari 5 P0**. Itu sendiri adalah temuan arsitektural: tidak ada satu pun test negatif untuk otorisasi lintas-ruang, webhook tanpa `dr`, atau idempotensi processor.

| Severity | Jumlah | Ringkasan |
|---|---:|---|
| **P0** | 5 | Integritas skor, immutabilitas hasil, kunci JWT publik, akun default, backup tak terverifikasi |
| **P1** | 15 | Otorisasi lintas-ruang, identitas supervisor ambigu, DoS (login, body, retry), hasil hilang diam-diam, multi-tenant rusak, migration tanpa version tracking, tool ops dengan DSN salah, restore dan clean destruktif, exit code, hygiene Docker |
| **P2** | 21 | Jalur legacy tanpa batas waktu, reset password via CSV, IDOR, ekspor terpotong, generator password non-acak, UI misleading, iframe sandbox, harness load menulis ke DB produksi, env var inkonsisten, observability |
| **P3** | 12 | 76 file belum gofmt, dead code, nol test frontend, nol CI, 14+3 vulnerability npm, test suite lambat, dokumentasi bertentangan dengan kode |

**Rekomendasi eksekutif:** jangan deploy apa pun sebelum P0 nomor 1 sampai 5 beres. Untuk platform penilaian, P0 nomor 1 (skor) adalah yang paling merusak secara akademik dan paling sulit diremediasi belakangan, karena keputusan "hapus data yang sudah terlanjur masuk" harus diambil kemudian.

---

## 2. Metodologi dan Bukti Verifikasi

Audit dilakukan dengan membaca implementasi, test, dan dokumentasi secara berpasangan, memetakan trust boundary, lalu menjalankan seluruh gate yang tersedia di repo.

### 2.1 Command yang dijalankan dan hasilnya

| Command | Hasil | Interpretasi |
|---|---|---|
| `go build ./...` | exit 0, tanpa output | Kompilasi bersih |
| `go vet ./...` | exit 0, tanpa output | Tidak ada temuan vet |
| `go test ./...` (cached) | semua `ok` | - |
| `go test -count=1 ./...` | semua `ok`; `internal/api/handlers` **414,570 detik** | Suite benar, tapi satu package memakan 7 menit; CI perlu cache dan paralelisme |
| `go test -race ./internal/submission/...` | **gagal dijalankan**: `cgo: C compiler "gcc" not found` | Race detector belum pernah dijalankan; sinkronisasi worker dan queue belum pernah diuji |
| `gofmt -l .` | **76 file tidak ter-format** | Termasuk `cmd/server/main.go`, seluruh handler inti, seluruh `internal/submission` |
| `npm run build` (web) | sukses, dengan **warning a11y** | `web/src/lib/components/ui/Table.svelte:12` - `a11y_no_noninteractive_tabindex` |
| `npm audit` (web) | **14 vulnerability**: 2 critical, 4 high, 6 moderate, 2 low | `shell-quote` (critical), `SvelteKit`, `browserslist`, `nanoid`, `postcss`, `vite`/`esbuild`, `cookie`, `devalue` |
| `npm audit` (root) | **3 vulnerability**: 2 critical, 1 high | `shell-quote` via `concurrently`, `js-yaml` |
| `Test-Path .dockerignore` | `False` | Build context tidak dikontrol |
| `git ls-files --error-unmatch .env` | tidak tracked | `.env` aman |
| `git ls-files opencode.json .opencode` | tidak tracked | Konfigurasi lokal saja |
| GitHub API `repos/saroel01/Aether-CBT` | `"private": false` | Semua hardcoded secret di bawah sudah terbaca publik |

Tooling keamanan yang **tidak tersedia** di mesin ini: `govulncheck`, `gosec`, `golangci-lint`, `staticcheck`, `svelte-check`. Temuan yang biasanya tertangkap tool tersebut (hardcoded credential, dependency OWASP) harus dicari manual.

### 2.2 Trust boundary yang dipetakan

```
Browser (SPA, token JWT di localStorage)
   |  X-Tenant-ID (dikontrol klien, selalu dikirim) + Authorization: Bearer
   v
Fiber: CORS -> SecurityHeaders -> TenantMiddleware -> AuthMiddleware -> RequireRoles
   |
   +- PUBLIK: /api/auth/{login,student-login,supervisor-login}, /api/qrcode
   +- PUBLIK: /api/ispring/webhook        <- attempt_token = otorisasi utama
   +- PUBLIK: /api/exam/content/*         <- cookie aether_exam = capability
   v
Handlers -> Repository (tenant-scoped) -> SQLite (WAL, FK on, busy_timeout 5000)
   |
   v
Queue filesystem (pending/processing/done/failed) -> Worker -> Processor (1 tx per batch + SAVEPOINT per job)
   |
   v
Filesystem: data/soal/{tenant_slug}/{uuid}/  <- konten iSpring + shim yang di-inject saat diserve
```

---

## 3. Peta Sistem Ringkas

| Komponen | Lokasi | Catatan |
|---|---|---|
| Entry server | `cmd/server/main.go:28` | load `.env` manual, config, DB, migration, legacy migrator, queue, worker, Fiber, static plus SPA, graceful shutdown |
| Config | `internal/config/config.go:39-106` | `JWT_SECRET` wajib non-kosong; `CORS_ALLOWED_ORIGINS` wajib di production; body limit mengikuti `SOAL_UPLOAD_MAX_BYTES` |
| Route | `cmd/server/main.go:157-284` | 4 route auth publik, qrcode, webhook, content, sekitar 35 protected |
| Middleware | `internal/api/middleware/{tenant,auth,role,security_headers}.go` | tenant dari header, query, form, slug, subdomain, lalu fallback dev |
| DB | `internal/db/sqlite.go:114-168` | DSN `file:` + `_pragma=` (sifatnya load-bearing), `VerifyPragmas` menggagalkan startup bila pragma tidak aktif |
| Migration | `internal/db/migrate.go:32-113` | 31 file `.sql`, tanpa tabel version, per statement, tanpa transaksi |
| Queue | `internal/submission/fsqueue.go` | 6 direktori, fsync plus atomic rename, retry exponential lewat nama file, recovery saat startup |
| Worker | `internal/submission/worker.go:72-183` | batch 5 per 100 ms, kontrak error per job, panic recovery |
| Processor | `internal/submission/processor.go:37-291` | 1 transaksi LevelSerializable plus SAVEPOINT per job, UPSERT hasil, ganti detail, hapus `cek_login` |
| iSpring | `internal/ispring/{parser,score}.go` | `encoding/xml` (tanpa XXE), total diabaikan |
| Paket soal | `internal/soalpkg/{storage,serve,shim,strip_launcher}.go` plus `assets/ispring-shim.js` | anti zip-slip dan zip-bomb, shim di-inject setelah `<head>` |
| Frontend | `web/src/routes/**` (23 route), `web/src/lib/**` | SPA murni (`ssr=false`), seluruh guard hanya di sisi klien |
| Frontend terbesar | `web/src/routes/student/exam/+page.svelte` (970 baris) | 6 interval, 5 timeout, 5 listener window, iframe iSpring plus shim |

---

## 4. Temuan P0 - Hentikan Deploy

### P0-1 - Skor ujian sepenuhnya dikendalikan siswa; `skor_maks` tidak pernah diderivasi

**Bukti**

- `internal/api/handlers/ispring.go:39-43` membaca `dr`; validasi hanya di `ispring.go:94-98` dengan penjaga `if detailXML != ""`, sehingga `dr` boleh kosong.
- `internal/submission/processor.go:179-186` membuat `detailReport`; `processor.go:192-207` menempatkan **seluruh** verifikasi skor di dalam `if detailReport != nil`.
- `processor.go:193` membuang nilai balik kedua dari `DerivedScore` (`internal/ispring/score.go:6-15` mengembalikan `awarded, max`).
- `processor.go:239` menulis `job.Score` dan `job.MaxScore` apa adanya; keduanya berasal dari form `sp` dan `tp` (`ispring.go:37-38`).
- Sumber `awardedPoints` dan `maxPoints` adalah atribut XML yang dikirim klien (`internal/ispring/parser.go:66-67`). Tidak ada answer key di seluruh repo.

**Pemicu A: nilai 100 tanpa `dr`**

```
POST /api/student/start     {"peserta_id":<id sendiri>,"session_id":S}   -> attempt_token
POST /api/ispring/webhook   attempt_token=..&sid=<no_id>&sp=100&tp=100   (tanpa field dr)
-> "Result received successfully" -> hasil_tes.skor = 100, skor_maks = 100, status = submitted
```

**Pemicu B: `skor_maks` dikontrol**

Kirim `dr` valid berisi satu soal dengan `awardedPoints="1" maxPoints="1"`, disertai `sp=1&tp=1000000`. `ScoresConsistent(1, 1, 0.05)` lolos, `skor=1` tersimpan, `skor_maks=1000000`. Semua rasio dan peringkat di `csv_utility.go:238-245` serta `item_analysis.go:61-78` menjadi tidak bermakna.

**Pemicu C: keracunan tipe kolom**

`hasil_tes.skor` bertipe `REAL` (`migrations/010:7`) tetapi menerima string. Nilai non-numerik akan disimpan sebagai TEXT oleh SQLite dan menggagalkan `rows.Scan(&skorNull)` bertipe `sql.NullFloat64` di `supervisor.go:187`, `csv_utility.go:232`, dan seluruh ekspor.

**Catatan:** guard yang ada saat ini adalah teater. `ScoresConsistent` (`score.go:20-25`) hanya membandingkan dua nilai yang sama-sama dikendalikan penyerang, sehingga tidak dapat mendeteksi pemalsuan.

**Dampak:** penilaian dihancurkan. Setiap siswa dapat mengirim nilai apa pun. Ini juga membatalkan klaim di `README.md:24-25` bahwa skor diverifikasi server.

**Perbaikan**

1. Jadikan `dr` wajib: hapus penjaga `if detailXML != ""` di `ispring.go:94-98`, kembalikan 400 bila kosong.
2. Tolak di `processor.go` bila `detailReport == nil`; jangan pernah menulis baris hasil tanpa XML.
3. Pakai nilai balik kedua `DerivedScore` untuk `skor_maks`; abaikan `tp` sepenuhnya; tolak `derivedMax` yang tidak positif.
4. **Perbaikan struktural (sangat disarankan):** saat paket iSpring diunggah (`soal_package_handler.go:40`), ekstrak answer key beserta `max_points` per `question_id` ke tabel `soal_answer_key`, lalu grade di server. Tanpa ini, skor tetap client-asserted dan hanya bisa dilabeli demikian di UI dan laporan.

**Regression test** (`internal/submission/processor_test.go`)

```go
job := &SubmissionJob{TenantID: 1, NoID: "p1", Score: "100", MaxScore: "100",
                       AttemptToken: tok, Validasi: "1_p1_9"}
if err := p.Process(ctx, job); err == nil { t.Fatal("accepted XML-less submission") }

job2 := /* dr valid, satu soal awarded=1 max=1 */, MaxScore: "1000000"
_ = p.Process(ctx, job2)
// assert hasil_tes.skor_maks == 1.0, bukan 1000000
```

---

### P0-2 - Hasil ujian dapat ditimpa ulang; nilai esai yang sudah dinilai guru dapat terhapus

**Bukti**

- `internal/submission/processor.go:109-140`: bila `cek_login` tidak ada (`sql.ErrNoRows`) tetapi `hasil_tes` untuk `job.Validasi` ada, processor tetap mengeset `requiresGraceCheck = false` (`:114-120`) lalu **melanjutkan proses normal**.
- `processor.go:224-239`: `ON CONFLICT(tenant_id, validasi) DO UPDATE SET skor, skor_maks, detail_xml`. Index unik `idx_hasil_tes_unique_validasi` (`migrations/017:37-38`) membuat overwrite ini deterministik.
- `processor.go:255-259`: `DELETE FROM hasil_tes_detail WHERE hasil_tes_id = ?`. Strategi replace ini menghapus semua baris.
- `processor.go:262-279`: hanya menyisipkan ulang apa yang ada di XML penyerang.
- `processor.go:283-288`: `DELETE FROM cek_login ... attempt_token = ?` menghapus baris sesi setelah commit, dan itulah yang membuka jalan bagi fallback di atas.
- `internal/api/handlers/essay_grading.go:148-176`: penilaian guru menulis `awarded_points` lalu **menghitung ulang `hasil_tes.skor`** dari jumlah detail.
- `internal/api/handlers/student_exam.go:204-211`: jalur legacy `mapel_id` **tidak memeriksa jendela waktu, status, maupun package**, dan me-reset `login_time = CURRENT_TIMESTAMP` pada setiap pemanggilan.
- `internal/api/handlers/ispring.go:89-92`: `validasi` berbentuk `tenant_noID_mapelID` (legacy) atau `tenant_noID_sessionID`; jalur legacy menghasilkan `validasi` yang sama.

**Pemicu A: overwrite skor.** Kirim dua POST webhook dengan `attempt_token` yang sama sebelum worker drain. Keduanya dijawab `200` (`ispring.go:65-67` masih menemukan `cek_login`). Worker memproses FIFO, sehingga job kedua menimpa job pertama.

**Pemicu B: penghapusan nilai guru.** Guru menilai esai, lalu siswa memakai jalur legacy untuk mendapat `attempt_token` baru. Submission baru mendarat pada baris `hasil_tes` yang sama, menghapus seluruh `hasil_tes_detail`, dan mengembalikan `skor` yang diklaim siswa sendiri. Grace period juga dilewati karena `requiresGraceCheck=false`.

**Dampak:** hasil mutable setelah kejadian; penilaian manusia hilang tanpa jejak audit. Bagi platform CBT ini setara dengan mengizinkan siswa mengubah nilai.

**Perbaikan**

1. Processor harus immutable setelah commit: pada fallback `ErrNoRows`, tulis ke tabel audit `hasil_tes_replay` atau kembalikan error terminal, jangan proses ulang. Idempotensi berarti payload identik adalah no-op, bukan penulis terakhir menang.
2. Hapus jalur legacy `mapel_id`, atau gate dengan flag `LEGACY_MAPEL_PATH=false` (default) **dan** periksa jendela waktu.
3. Tolak UPSERT bila `exam_session_id` sudah terisi dan attempt owner berbeda.
4. Simpan hash payload pada `hasil_tes` agar replay dengan payload identik terdeteksi secara deterministik.

**Regression test.** Proses job A, lalu proses job B dengan `Validasi` sama dan `Score:"100"`. Assert: job B error, `hasil_tes.skor` tidak berubah, dan `COUNT(hasil_tes_detail)` tetap sama setelah penilaian guru.

---

### P0-3 - Kunci JWT produksi tertanam di Dockerfile yang dipublikasikan; image berjalan dalam mode development

**Bukti**

- `Dockerfile:61` - `ENV JWT_SECRET=supersecurejwtkey2026`.
- `internal/config/config.go:64-66` - hanya menolak secret yang **kosong**; tidak ada cek panjang, entropi, maupun denylist.
- `internal/utils/auth.go:15-20,34-39` - dipakai apa adanya sebagai kunci HMAC; `internal/api/middleware/auth.go:28` memakainya untuk verifikasi.
- `.env.example:7` (tracked) - kunci kedua yang dikenal publik: `aether-cbt-secret-key-change-in-production`.
- `Dockerfile:59-61` tidak menyetel `ENV`, sehingga `internal/config/config.go:48` memakai default `development`.
- Akibatnya: `internal/api/middleware/tenant.go:101-105` mengembalikan **tenant 1 untuk setiap request tanpa `X-Tenant-ID`**, sehingga cabang production `tenant.go:107-111` yang mengembalikan 400 mati. Guard CORS production `cmd/server/main.go:136-142` juga mati dan jatuh ke origin localhost.
- `DEPLOYMENT_LINUX_VPS.md:63` dan `DEPLOYMENT_COOLIFY.md:43` mengklaim aplikasi menolak start jika secret kosong atau lemah, dan bahwa variabel tersebut WAJIB. **Tidak ada mekanisme weak-check di kode.**
- GitHub API: repo `private: false`, sehingga kedua kunci di atas sudah terbaca publik.

**Pemicu.** Sign `{user_id:1, tenant_id:1, role:"admin", exp:<masa depan>}` dengan HS256 memakai string `supersecurejwtkey2026`, lalu kirim sebagai `Authorization: Bearer`. Akses admin penuh tenant 1 pada setiap deployment yang mengikuti image default.

**Dampak.** Bypass autentikasi total pada platform multi-tenant; data seluruh sekolah dapat dieksfiltrasi dan dimodifikasi.

**Perbaikan**

1. Hapus `Dockerfile:61` sepenuhnya.
2. `config.Load` menolak secret yang kosong, pendek (kurang dari 32 byte), atau ada dalam denylist nilai yang pernah dipublikasikan.
3. Set `ENV=production` di image, atau jadikan `APP_ENV` tanpa default `development` untuk build rilis.
4. Rotasi `JWT_SECRET` di setiap instance yang mungkin pernah memakai default. Token lama tetap valid sampai `exp` (24 jam) habis, jadi rotasi perlu segera.
5. Ubah `.env.example:7` menjadi `JWT_SECRET=` disertai perintah generate.

**Regression test.** `config.Load()` dengan `JWT_SECRET=supersecurejwtkey2026` harus gagal. Tambahkan tabel test berisi semua nilai default yang pernah dipublikasikan.

---

### P0-4 - Setiap instalasi mendapat akun `admin` dengan password `admin123` yang berfungsi

**Bukti**

- `internal/db/migrations/004_create_admin_user.sql:5` - `INSERT OR IGNORE INTO users (tenant_id, username, password_hash, role, full_name, is_active) VALUES (1, 'admin', '$2a$14$ZWg8M9q80U7P9MaoOFunseFWwQFM2nQsamPDBneEtxrUkIMdpwuMm', 'admin', 'System Administrator', TRUE)`.
- `cmd/server/main.go:70` menjalankan `db.RunMigrations` tanpa syarat di setiap boot.
- `cmd/createadmin/main.go:25-42` - password `admin123`, memakai `INSERT OR REPLACE` yang berarti delete lalu insert, sehingga mengganti `id` dan me-reset password tanpa interaksi, lalu mencetak password ke stdout.
- `cmd/seed/main.go:50-52` - kredensial ruang `ruang123`; `internal/api/handlers/student.go:110-112` dan `csv_utility.go:31,102` - password siswa `siswa123`.
- **Verifikasi runtime:** `bcrypt.CompareHashAndPassword` terhadap literal tersebut cocok untuk `admin123` (match, err nil) dan tidak cocok untuk `password` maupun `admin`.
- `docs/credential-rotation.md:53` hanya memberi wearer "jangan pakai admin123 di produksi". Tidak ada kode yang memaksa rotasi, dan migration justru menyuntik ulang kredensial itu bila terhapus.

**Pemicu.** `POST /api/auth/login {"username":"admin","password":"admin123"}` di sekolah mana pun yang belum mengubahnya.

**Perbaikan**

1. **Jangan edit 004**, karena sudah pernah dijalankan di lapangan. Tambah migrasi baru, misalnya `033`, yang menghapus baris admin hasil seed dengan penanda yang jelas (hash persis atau cap waktu migrasi).
2. Gate pembuatan admin pertama lewat env `SETUP_ADMIN_PASSWORD` atau CLI oneshot yang menolak jalan bila DB sudah punya admin.
3. `cmd/seed` menolak jalan bila `ENV=production`.
4. `createadmin` memakai `INSERT` yang gagal bila sudah ada, ditambah flag `--force` eksplisit, dan tidak mencetak password ke log produksi.

**Regression test.** Setelah `RunMigrations` pada DB kosong, `SELECT COUNT(*) FROM users WHERE role='admin'` harus `0`.

---

### P0-5 - Backup tidak pernah diverifikasi, dan backup yang baik bisa dihapus

**Bukti** (`scripts/backup.go`)

- `:42` - `dsn := *dbPath + "?_journal_mode=WAL&_foreign_keys=on"`, lalu `sql.Open` pada **database sumber**.
- `:51` - `VACUUM INTO '<backupFile>'` menulis file backup.
- `:58-59` - `db.QueryRow("PRAGMA integrity_check;")` dijalankan pada **handle yang sama**, yaitu **database sumber**. Tidak pernah ada `sql.Open` kedua pada `backupFile`.
- `:65-68` - bila `integrity` bukan `ok`, script **menghapus file backup** lewat `os.Remove(backupFile)`. Sumber yang korup berarti backup dihapus.
- `docs/backup-restore.md:47` mengklaim dilakukan `integrity_check` pada file backup, dan `:56` menjanjikan status integrity `ok`. Klaim `:45` bahwa WAL dibuka dengan benar juga salah, karena DSN-nya diabaikan driver (lihat P1-15).
- `//go:build ignore` berarti file ini **tidak pernah dikompilasi** oleh `go build ./...`, `go vet ./...`, maupun `go test ./...`.

**Verifikasi.** Reviewer kedua menjalankan ulang urutan persis script pada salinan DB, lalu menimpa file backup dengan 4 KB `0xDE`. Script melaporkan `integrity = "ok"`, sementara `integrity_check` pada file backup sebenarnya mengembalikan "file is not a database".

**Dampak.** Setiap backup yang dihasilkan tool tidak tervalidasi. Restore point sekolah bisa berupa file terpotong atau corrupt tanpa satu pun sinyal. Karena `docs/backup-restore.md:47` menyatakan hal sebaliknya, operator believe sudah aman.

**Perbaikan**

1. Buka `*sql.DB` kedua **pada `backupFile`**, jalankan `integrity_check` dan `foreign_key_check` di sana.
2. Hanya cetak "Backup berhasil" setelah keduanya `ok`, dan **jangan pernah menghapus file backup** karena kegagalan di sisi sumber.
3. Pakai `db.DSN()` (`internal/db/sqlite.go:70-72`) agar pragma benar dan konsisten dengan aplikasi.
4. Tambahkan `scripts/backup_test.go`, atau hapus tag `ignore`, agar file masuk permukaan CI.

**Verifikasi.** Korupikan backup yang dihasilkan, jalankan script asli, lalu assert exit 1 dan tidak ada pesan sukses.

---

## 5. Temuan P1

### P1-6 - Supervisor dapat me-reset sesi siswa di ruang lain

`internal/api/handlers/supervisor.go:274-314`. `ResetStudentSession` hanya memeriksa `role`Supervisor atau admin dan `req.PesertaID > 0`, lalu menjalankan:

```go
// session_id > 0
DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ? AND session_id = ?
// tanpa session_id: menghapus SEMUA sesi peserta tersebut
DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ?
```

**Tidak ada predikat `ruang_id`.** Bandingkan handler saudara `UnlockStudentSession` yang **memilikinya** di `supervisor.go:336-346`, dan `RecordInfraction` di `anticheat_handler.go:58-61`.

**Pemicu.** Supervisor ruang 1 mengirim `POST /api/supervisor/reset {"peserta_id":<id siswa ruang 2>}`, dijawab `200 OK`. Tanpa `session_id`, seluruh `cek_login` siswa tersebut dihapus.

**Dampak.** Batas antar-ruang ditembus; proctor bisa membutakan monitor ruang lain dan menghapus status lock-nya. Perlu dicatat bahwa `GET /supervisor/room-status` untuk supervisor **sudah** ter-scope ke ruangnya (`supervisor.go:246`), sehingga ini inkonsistensi internal, bukan keputusan desain.

**Perbaikan.** Ekstrak satu helper `assertSupervisorOwnsPeserta(c, pesertaID) error` yang dipakai `ResetStudentSession`, `UnlockStudentSession`, dan `RecordInfraction`; tambahkan `AND ruang_id = ?` dan kembalikan 403 bila count 0.

**Regression test.** Seed dua ruang; token supervisor dengan `user_id=1`; `POST /api/supervisor/reset {"peserta_id":<id ruang-2>}` harus menghasilkan 403 **dan** baris `cek_login` ruang-2 harus masih ada.

---

### P1-7 - Supervisor dari tabel `users` beroperasi pada ruang yang dipilih secara numerik

`internal/db/migrations/002_create_users.sql:8` mendefinisikan `users.ruang_id`, dan `internal/models/user.go:11` mengekspos `RuangID`. Namun:

- `internal/api/handlers/user.go:74-77` - `INSERT INTO users (tenant_id, username, password_hash, role, full_name)`: `ruang_id` **tidak pernah diisi**, dan tidak pernah dibaca di mana pun (diverifikasi lewat grep).
- `user.go:61` - `allowedRoles := {"admin","supervisor"}`, sehingga `POST /api/users` **boleh** membuat supervisor di tabel `users`.
- Dua jalur login menghasilkan `role="supervisor"` dengan **urutan ID yang berbeda**, karena `002:3` dan `migrations/006:3` keduanya `INTEGER PRIMARY KEY AUTOINCREMENT`:
  - `supervisor.go:53` - `GenerateToken(id, tenantID, "supervisor")` dengan `id` berupa **`ruang.id`**.
  - `handlers/auth.go:53` - `GenerateToken(user.ID, ...)` dengan `user.ID` berupa **`users.id`**.
- Semua consumer memperlakukan `user_id` sebagai id ruang: `supervisor.go:246`, `supervisor_sse.go:18`, `anticheat_handler.go:56`, `supervisor.go:337`.

**Pemicu.** Admin membuat baris `users` dengan `role:"supervisor"`, sehingga `users.id = 3`. Bila `ruang.id = 3` adalah ruang lain, akun tersebut mendapat monitor real-time ruang 3, dapat mengunci siswanya (`anticheat_handler.go:56-64`), dan membukanya kembali (`supervisor.go:337-346`).

**Efek samping yang lebih luas.** `handlers/me.go:19-23` mencari baris di `users` berdasarkan `user_id`; untuk supervisor berbasis `ruang`, baris itu tidak ada, sehingga `GET /api/me` selalu 404 bagi supervisor.

**Perbaikan.** Hapus `supervisor` dari allowlist `CreateUser`, karena supervisor adalah kredensial ruang, bukan akun user. Alternatifnya, mewajibkan `ruang_id` dan membacanya dari DB. Jangka panjang: masukkan `room_id` ke claim JWT dan assert non-nol untuk role supervisor di `middleware/auth.go`.

---

### P1-8 - Tidak ada rate limit pada tiga endpoint login, dengan bcrypt cost 14

`cmd/server/main.go:161-165` mendaftarkan tiga route auth publik **tanpa limiter**; hanya webhook yang punya (`:177-184`). `internal/utils/auth.go:43` memakai `bcrypt.GenerateFromPassword([]byte(password), 14)` yang kira-kira 1,5 detik CPU per percobaan, dipanggil di `auth.go:48`, `supervisor.go:48`, dan `exam.go:53`.

**Pemicu.** Sekitar 20 koneksi paralel ke `/api/auth/supervisor-login` dengan tebakan password akan menyenuh seluruh core CPU, dan semua login siswa sah ikut antre. Sekaligus menjadi oracle brute force tanpa throttle, dengan `no_id` siswa yang umumnya berupa angka 7 digit berurutan.

**Catatan positif.** Respons seragam `401 Invalid credentials` (`auth.go:45,49` dan `exam.go:53-55`) membuat enumerasi username sulit. Yang hilang hanya throttling.

**Perbaikan**

```go
loginLimiter := limiter.New(limiter.Config{Max: 10, Expiration: time.Minute,
    KeyGenerator: func(c *fiber.Ctx) string { return c.IP() + "|" + fmt.Sprint(c.Locals("tenant_id")) }})
auth.Post("/login", loginLimiter, handlers.Login)   // plus student-login dan supervisor-login
```

Tambahkan counter per `no_id` dengan lockout, dan pindahkan cost bcrypt ke 10 sampai 12 secara config-driven untuk windows ujian.

**Regression test.** Attempt login ke-11 dalam menit dari satu IP harus menghasilkan 429.

---

### P1-9 - `BodyLimit` 100 MB berlaku global termasuk webhook publik, dan tidak ada timeout

`cmd/server/main.go:125-132` - `fiber.Config{BodyLimit: int(cfg.SoalUploadMaxBytes)}` dengan default `100*1024*1024` (`internal/config/config.go:56`). Komentar di `main.go:127-130` sendiri mengakui belum ada limit per route. `ReadTimeout`, `WriteTimeout`, dan `IdleTimeout` **tidak diatur**, sehingga nil berarti tanpa batas.

`internal/api/handlers/ispring.go:29-43` memanggil `c.FormValue(...)` pada route publik tersebut. fasthttp harus mem-buffer seluruh body, membuat salinan ter-parse, dan mengembalikan string, sehingga sekitar tiga kali ukuran body per koneksi. Guard `attempt_token` baru berjalan di `ispring.go:46`, yaitu **setelah** body ter-buffer.

**Pemicu.** Dua puluh POST paralel ke `/api/ispring/webhook` dengan body urlencoded 100 MB menghasilkan sekitar 6 GB RSS pada VPS 2 sampai 4 GB, sehingga OOM kill di tengah ujian. Tidak butuh kredensial apa pun.

**Perbaikan**

1. `BodyLimit` global menjadi 1 MB.
2. Terapkan 100 MB hanya pada `POST /api/admin/soal-packages/upload` dan `/api/admin/students/import-csv` lewat `bodylimit.New(...)` per route.
3. Set `ReadTimeout`, `WriteTimeout`, dan `IdleTimeout`.
4. Tambahkan batas konkurensi pada route webhook.

**Regression test.** Webhook dengan body 2 MB harus menghasilkan 413.

---

### P1-10 - Rate limit webhook 100 per menit per IP, ditambah retry klien tanpa backoff, sehingga hasil ujian hilang massal

**Bukti**

- `cmd/server/main.go:171-184` - `limiter.New(limiter.Config{Max: 100, Expiration: time.Minute, ...})` tanpa `KeyGenerator`. Terverifikasi pada source `fiber@v2.52.5/middleware/limiter/config.go`, yang punya default `KeyGenerator: func(c *fiber.Ctx) string { return c.IP() }`.
- Artinya **seluruh siswa di balik satu IP publik atau NAT berbagi kuota 100 per menit**. Bukti empiris ada di `tests/load/E2E_RESULTS.md:95,119,148`, di mana semua run skala 200 dan 500 harus menaikkan `WEBHOOK_RATE_LIMIT_PER_MIN=100000`. `tests/load/README.md:16` mencatat "Rate Limiter: Dinonaktifkan untuk pengujian (dinaikkan ke 100000/min)".
- `web/src/routes/student/exam/+page.svelte:284-323` - `retrySubmission()` memakai interval tetap **3,5 detik tanpa backoff dan tanpa batas attempt**. Respons `!res.ok` untuk kode 403, 409, 429, atau 500 **tidak menghentikan retry**, hanya lolos ke jadwal berikutnya (`:319-322`).

**Dampak**

1. Saat 300 siswa mengirim bersamaan di akhir ujian, sebagian besar mendapat 429 dan hasil mereka hilang.
2. Satu siswa dengan sesi rusak, misalnya 403 karena token terkunci, akan **retry tanpa henti** tiap 3,5 detik, memakai kuota IP yang sama dan memblokir siswa lain di sekolah yang sama.
3. Ini bukan masalah teoretis: dokumentasi load test sudah harus mematikan limiter agar test lulus.

**Perbaikan**

1. Pakai `KeyGenerator` custom dengan kunci primer `attempt_token` yang sudah unik per attempt, plus kuota global per tenant, dan simpan penghitung per token dengan TTL.
2. Naikkan default ke nilai yang realistis untuk ukuran sekolah, misalnya 1000 per menit per token, dan **dokumentasikan** lewat runbook, bukan lewat env yang tidak disebut di mana pun. Verified: `WEBHOOK_RATE_LIMIT_PER_MIN` tidak muncul di lima panduan ops mana pun.
3. Sisi klien: exponential backoff plus jitter, batas attempt, dan **berhenti** pada 403 dan 409 karena tidak akan pernah sukses.

---

### P1-11 - Hasil ujian hilang diam-diam: UI menyatakan tersimpan aman sebelum server mengonfirmasi

**Bukti**

- `internal/soalpkg/assets/ispring-shim.js:113` (XHR), `:128` (fetch), `:140` (beacon), dan `:163` (form) memanggil `notifyParent('aether_result', {})` **tepat setelah mengirim, sebelum mengetahui respons**. Tidak ada handler respons sama sekali.
- `web/src/routes/student/exam/+page.svelte:546-568` - `onIframeMessage` menerima `aether_result` dan langsung menetapkan `submitted = true` serta `showResultModal = true`.
- `exam/+page.svelte:682-696` (`handleTimeExpired`) dan `:702-716` (`endExamEarly`) menetapkan `submitted = true` setelah mengirim `postMessage`. Flag `submissionPending` hanya di-set bila `!isOnline` (`:692-695` dan `:709-711`).
- Konsekuensinya: webhook membalas 429, 403, atau 500 yang sangat mungkin terjadi (lihat P1-10), tetapi UI tetap menampilkan "Sesi Ujian Selesai" dengan klaim hasil telah tercatat, `flushProgress` dihentikan (`:571`), dan overlay retry tidak pernah muncul. Payload tersimpan di `localStorage` (`saveSubmissionPayload`, `:254-266`) tapi **tidak pernah dikirim ulang**.
- `endExamEarly` juga menganggap sukses sebelum mengetahui apakah shim berhasil membentuk payload. Shim bisa jatuh ke `notifyParent('aether_result', {forced:true, empty:true})` di `ispring-shim.js:205`, yang berarti **tidak ada payload yang dikirim**.

**Dampak.** Kehilangan hasil tanpa satu pun sinyal ke siswa maupun guru. Ini justru skenario yang paling ditakuti di platform ujian.

**Perbaikan**

1. **Handshake:** shim hanya mengirim `aether_result` setelah respons webhook 2xx, dengan hook pada `XMLHttpRequest.onload` atau `fetch().then`, dan sertakan `ok:false` bila gagal agar parent masuk mode retry.
2. `endExamEarly` dan expiry tidak boleh menetapkan `submitted=true`; tunggu konfirmasi, dengan timeout yang jatuh ke `submissionPending`.
3. `retrySubmission`: backoff exponential plus jitter, batas attempt, dan berhenti pada 4xx selain 408 dan 429.
4. `checkPendingSubmission` (`:269-282`) saat ini menetapkan `submitted = true` sebelum mengetahui hasil; ubah agar tidak mengklaim sukses.

---

### P1-12 - Multi-tenant berbasis subdomain tidak berfungsi pada build default

`web/src/lib/api.ts:22-27` (`getTenantID`) dan `:33-43` (`authHeaders`) mengirim `X-Tenant-ID` pada **setiap** request, dengan default `'1'`. `aether_tenant_id` tidak pernah ditulis di mana pun, dan `VITE_TENANT_ID` tidak diset saat build Docker. `internal/api/middleware/tenant.go:51-64` memberi **prioritas 1** pada header tersebut, sebelum lookup slug (`:66-81`) dan subdomain (`:83-97`).

**Dampak.** Pada deployment subdomain, yaitu kontrak yang didokumentasikan di `tenant.go:24` dan `DEPLOYMENT_LINUX_VPS.md`, seluruh login siswa, pengawas, dan admin diarahkan ke tenant 1. Murid tenant 2 tidak akan bisa login sama sekali; bila `no_id` bertabrakan, ia bahkan bisa login ke tenant yang salah. Mekanisme multi-tenant yang diklaim di `README.md:15` praktis hanya bekerja untuk satu tenant per build.

**Perbaikan.** Jangan kirim `X-Tenant-ID` bila tidak diset eksplisit; biarkan server me-resolve dari subdomain; atau kirim `X-Tenant-Slug` yang dibaca dari `location.hostname`. Tambahkan indikator tenant di UI hasil login, dan hapus fallback `tenant_id: 1` di `student/login/+page.svelte:59-64`.

---

### P1-13 - Tidak ada `middleware.Recover()`, dan ada 72 type assertion `c.Locals` tanpa check

`cmd/server/main.go:125-184` tidak mendaftarkan `middleware.Recover()`. Dokumentasi fasthttp eksplisit menyatakan tidak ada panic recovery di level server, sehingga satu panic akan menghentikan seluruh proses. Sementara itu ada sekitar 72 asersi satu-nilai `c.Locals("tenant_id").(int)`, `("role").(string)`, dan `("user_id").(int)` di seluruh handler, misalnya `supervisor.go:246,275,318,337`, `me.go:14-16,41-42`, `student_exam.go:17,47,119-120,222-223,230`, dan `ispring.go:117`.

**Status saat ini.** Reviewer menelusuri seluruh asersi dan **tidak menemukan panic yang dapat dipicu dari luar** pada kode hari ini: `TenantMiddleware` selalu mengeset `tenant_id` pada route non-exempt, dan `AuthMiddleware` mengeset ketiga claim sebelum handler protected jalan. Jadi ini **gap ketahanan, bukan crash yang terkonfirmasi**. Namun `csv_utility.go:97-101` mengindeks `record[0..4]` dengan penjaga hanya pada header (`:68`), yang aman semata karena `encoding/csv` menurunkan `FieldsPerRecord` dari record pertama dan error pada mismatch (`:92-94`). Menghapus jalur error tersebut akan menghidupkan panic remote.

**Perbaikan.** Tambahkan `app.Use(middleware.Recover())` sebagai middleware pertama, konversi seluruh asersi ke bentuk dua nilai, dan beri komentar penjaga untuk asumsi `FieldsPerRecord`.

---

### P1-14 - Migration tanpa tabel version, tanpa transaksi, dengan error ditelan berdasarkan substring

`internal/db/migrate.go:93-100` mengeksekusi **setiap** statement **tanpa** `BEGIN` dan `COMMIT`, bandingkan `internal/db/peserta_fk_repair.go:135-162` yang benar memakai satu `sql.Tx`. Polanya diketahui, hanya tidak dipakai di runner. `:158-171` menelan setiap error yang mengandung `duplicate column name` atau `already exists`. Tidak ada `schema_migrations` di seluruh repo. File 010, 011, dan 014 masih membawa anotasi mati `-- +goose Up`, indikasi alat yang tidak dipakai.

**Dampak**

1. Crash di tengah file meninggalkan migrasi setengah terpasang. Boot berikutnya menjalankannya ulang dan **bisa menelan gagalnya lagi**, sehingga `migrate.go:102` tetap mencatat "All migrations applied successfully" di atas skema yang tidak lengkap.
2. Pencocokan substring pesan driver adalah kontrak tidak stabil lintas upgrade driver.
3. Karena tidak ada version tracking, **tidak ada jejak** apakah migrasi data 027 pernah dijalankan. `027_password_hash_migration.sql:20` hanya berisi `SELECT 1;`, sedangkan rehash-nya ada di `cmd/migratepasswords` di luar `RunMigrations`.
4. Setiap boot menulis ke DB live: `migrations/017:8-38` menjalankan dua `DELETE` plus `CREATE UNIQUE INDEX`, dan `029_kelas_mapel_tenant.sql` menjalankan ulang pekerjaan full-table tiap start. Diverifikasi lewat `EXPLAIN QUERY PLAN`: `SCAN cek_login`, `SCAN hasil_tes`, `SCAN kelas_mapel`.

**Perbaikan.** Tabel `schema_migrations(version TEXT PRIMARY KEY, checksum TEXT, applied_at TEXT)`; satu transaksi per file; insert version di transaksi yang sama; skip yang sudah applied; gagal pada checksum drift; dan klasifikasikan error idempoten berdasarkan kode extended SQLite, bukan teks pesan.

---

### P1-15 - Empat tool operasional memakai DSN yang diabaikan driver

`internal/db/sqlite.go:35-43` mendokumentasikan perangkap ini panjang-lebar dan memperbaikinya untuk aplikasi lewat prefix `file:` plus `_pragma=`. Tooling tidak pernah dimigrasi:

| Tool | Lokasi | DSN |
|---|---|---|
| `scripts/backup.go` | `:42` | `*dbPath + "?_journal_mode=WAL&_foreign_keys=on"` |
| `scripts/reset_queue.go` | `:16` | `"data/cbt_aether.db?_journal_mode=WAL&_busy_timeout=5000"` |
| `scripts/inspect_queue.go` | pola sama | idem |
| `tests/load/dataprep.go` | `:30` | `dbPath + "?_journal_mode=WAL&_busy_timeout=5000"` |

**Diverifikasi terhadap `modernc.org/sqlite v1.50.1`:**

```
produksi (file: prefix)  journal_mode=wal     foreign_keys=1  busy_timeout=5000
backup.go:42             journal_mode=delete foreign_keys=0  busy_timeout=0
INSERT FK yang invalid pada DSN backup.go-style: err=<nil>   (FK enforcement OFF)
```

**Dampak.** Tool backup berjalan tanpa `busy_timeout`, sehingga `VACUUM INTO` di `:51`, yang merupakan satu-satunya mode online yang diizinkan `docs/backup-restore.md:104`, bisa gagal terhadap server yang hidup. Harness load prepping data dengan `busy_timeout=0` terhadap DB hidup **dan membuang seluruh error Exec** (`dataprep.go:55-58`), sehingga `SQLITE_BUSY` yang kontesten tidak terlihat dan run dilanjutkan dengan fixture setengah jadi.

**Perbaikan.** Pakai `db.DSN()` yang sudah diekspor di mana pun, dan tambahkan test yang mengassert `db.VerifyPragmas` lolos pada koneksi masing-masing tool. Verifikasi: `grep -n 'sql.Open("sqlite",' | grep -v 'file:'` harus nol hasil.

---

### P1-16 - `scripts/reset_queue.go` menghapus data tanpa filter tenant, tanpa konfirmasi, dan selalu melaporkan sukses

- `//go:build ignore` berarti **tidak pernah dikompilasi** oleh `go build`, `go vet`, maupun `go test ./...`; file hanya type-check bila dipanggil eksplisit.
- `:16` DB path hardcoded, tanpa flag `-db`, tanpa konfirmasi, tanpa guard environment.
- `:22-24` `DELETE FROM submission_queue` dan `DELETE FROM failed_submissions` **tanpa filter tenant**, sehingga mengosongkan antrean seluruh sekolah.
- `:25-43` penghapusan peserta memakai tebakan prefix `E2E%`, `WB%`, `LT%`, `LB%`, `SB%`, `DX%`, dan `FC%`, yaitu prefix yang **sangat mungkin** dipakai nomor peserta asli.
- `:47-57` setiap kegagalan hanya `warn:` ke stderr, lalu `:58` tetap mencetak `Reset complete.` dan exit 0.

**Dampak.** Kehilangan submission, arsip dead-letter, dan peserta nyata, semuanya dengan exit code sukses.

**Perbaikan.** Minta flag `-i` atau konfirmasi eksplisit; sediakan `-db`; scope semua `DELETE` ke satu tenant; ganti tebakan prefix dengan marker khusus peserta uji; `os.Exit(1)` pada error pertama; buang tag `ignore` atau tambahkan test yang meng-build tiap file.

---

### P1-17 - `scripts/restore.ps1` membuang transaksi yang belum checkpoint

- `:55` `Copy-Item $Database $oldBackup -Force` hanya menyalin file utama.
- `:68-78` kemudian **menghapus** `.db-wal` dan `.db-shm`.
- Transaksi yang sudah committed tetapi belum checkpointed hanya ada di `-wal`; menghapusnya berarti safety copy kehilangan semua transaksi sejak checkpoint terakhir.
- `:26` `Read-Host` membuat script tidak dapat dipakai dari cron atau CI. Pada host non-interaktif, return kosong membuat `:28-31` mencetak "Restore dibatalkan" dan **`exit 0`**, yaitu no-op yang dianggap sukses.
- Tidak ada `integrity_check` pada file hasil restore, dan migrasi tidak dijalankan.

**Perbaikan.** Snapshot `.db`, `-wal`, dan `-shm` bersama, atau `VACUUM INTO` dulu; tambahkan `-Force`; exit non-nol bila konfirmasi tidak ada; verifikasi file hasil restore sebelum menyatakan sukses; jalankan migrasi setelah restore.

---

### P1-18 - Kegagalan bind port keluar dengan exit code 0

`cmd/server/main.go:323-346`. `app.Listen` gagal, goroutine mengirim ke `serverErr`, `select` mengambilnya, lalu `log.Printf("Server failed to start or listen error: %v", err)` jatuh ke `log.Println("Aether CBT shutdown complete")`. `main()` return normal sehingga **exit 0**. Semua kegagalan startup lain di file yang sama memakai `log.Fatalf` (`:65,72,83,98,107,110`), jadi ini inkonsistensi, bukan pilihan desain.

**Dampak.** `restart: on-failure`, status deploy Coolify, dan gate CI mana pun yang membaca exit status akan membaca instance mati sebagai deploy sukses.

**Perbaikan.** `log.Fatalf("server listen: %v", err)` pada cabang `serverErr`. **Verifikasi:** tahan port dengan instance pertama, jalankan instance kedua, assert exit 1.

---

### P1-19 - `make clean` dan `npm run clean` menghapus database produksi

`Makefile:13-14` - `clean: rm -rf bin/ data/cbt_aether.db`, yang diiklankan di `Makefile:26` sebagai "Clean build artifacts". `package.json:15` - `"clean": "rm -rf web/.svelte-kit web/node_modules bin data/cbt_aether.db"`. Keduanya **tidak** menghapus `data/queue`, sehingga tersisa job file yang `peserta` dan `cek_login`-nya sudah hilang, yang berakhir retry lalu dead-letter.

**Perbaikan.** Hapus path DB dari setiap target clean. Bila memang perlu reset, gate di balik konfirmasi dan flag `-db`, serta tambahkan `backups/` ke `.gitignore` karena saat ini tidak di-ignore sehingga backup bisa ter-commit.

---

### P1-20 - Dockerfile tidak reproducible dan tidak aman secara operasional

- **Tidak ada `.dockerignore`** (verified `Test-Path .dockerignore` menghasilkan `False`), sementara `Dockerfile:12` menjalankan `COPY web/ ./`. Artinya `web/node_modules` yang berisi 130 entri dan dibangun di Windows, beserta `web/build`, ikut tersalin **menimpa** hasil `npm ci` di `Dockerfile:9`. Build tidak reproducible dan berpotensi rusak.
- `Dockerfile:20` memakai `golang:1.22-alpine` sedangkan `go.mod:3` menyatakan `go 1.25.0`. Dengan `GOTOOLCHAIN=auto` sebagai default, builder 1.22 akan mengunduh toolchain go1.25.0 saat `go mod download` (`:28`). Ini butuh jaringan, lambat, dan bertentangan dengan base yang dipin. Di lingkungan air-gapped build gagal.
- Tanpa `HEALTHCHECK`, tanpa `USER` (root, dengan `/app/data` world-readable di `:53`), dan tanpa `VOLUME` sehingga data hilang saat container diganti.
- `Dockerfile:35` `CGO_ENABLED=0` sudah benar dan konsisten dengan `modernc.org/sqlite` yang pure Go.

**Perbaikan.** Tambahkan `.dockerignore` berisi `data/`, `web/node_modules`, `web/build`, `web/.svelte-kit`, `.git`, `*.db*`, `backups/`, dan `release/`. Samakan versi builder dengan `go.mod` atau set `GOTOOLCHAIN=local`. Tambahkan `HEALTHCHECK`, `USER` non-root dengan `chown /app/data`, dan deklarasikan `VOLUME /app/data`.

---

## 6. Temuan P2

| # | Temuan | Bukti | Dampak dan Perbaikan |
|---|---|---|---|
| P2-21 | Jalur legacy `mapel_id` me-reset `login_time` dan **tidak** memeriksa jendela waktu, status, maupun package | `student_exam.go:189-215`, bandingkan jalur sesi di `:146-187` | Siswa dapat memperpanjang ujian tanpa batas dengan loop `POST /api/student/start`. `GetRemainingTime:297-299` dan grace check `processor.go:168-176` sama-sama bersandar pada `login_time`. Hapus atau gate dengan flag plus enforce window. |
| P2-22 | Impor CSV **me-reset password siswa yang sudah ada** ke `siswa123` yang dipublikasikan | `csv_utility.go:156-165` (`DO UPDATE SET ... password = excluded.password`), default di `:31,102` | Re-import roster membuat semua siswa kembali punya password yang dikenal publik. Hapus `password` dari `DO UPDATE`; sediakan kolom opt-in `reset_passwords=true`. |
| P2-23 | Impor CSV tanpa batas baris dan tanpa batas password unik | `csv_utility.go:75-169`; cache hanya mengulang password (`passwordCache`, `:81,146-154`) | CSV 1.000 baris dengan 1.000 password unik menghasilkan sekitar 25 menit CPU bcrypt-14 **sambil memegang writer SQLite**, sehingga writer lain gagal pada `busy_timeout` 5 detik. Cap rows, misalnya 5.000, plus cap password unik per import. |
| P2-24 | `CreateStudent` memakai password default `siswa123` | `student.go:110-112` | Sama dengan P2-22. Wajibkan password eksplisit, atau generate acak dan kembalikan sekali. |
| P2-25 | IDOR: `GetAvailableMapels` menerima `peserta_id` milik siswa lain | `student_exam.go:46-63`; route `main.go:214` memakai `authenticatedExamUsers`, sehingga student juga boleh | Paksa `peserta_id = c.Locals("user_id")` untuk role student, seperti yang sudah benar dilakukan `GetRemainingTime:229-231`. |
| P2-26 | Supervisor tidak bisa mengganti password-nya sendiri | `main.go:254` - `protected.Put("/me", adminOnly, ...)`; `me.go:19-23` juga 404 untuk supervisor berbasis `ruang` | Proctor tidak bisa merotasi kredensialnya. Buka `PUT /me` untuk semua role, dan perbaiki resolusi identity. |
| P2-27 | JWT 24 jam tanpa revokasi, dengan minimum password 6 karakter | `utils/auth.go:59-70`; `me.go:84-94` | Perubahan password tidak meng-invalidasi token lama. Tambahkan claim `token_version` atau `password_changed_at` yang diperiksa di `middleware/auth.go`, dan naikkan minimum ke 8. |
| P2-28 | Ekspor hasil **terpotong diam-diam** pada 5.000 baris tanpa tiebreaker | `csv_utility.go:213` dan `:290` (`LIMIT 5000`, `ORDER BY k.nama_kelas, p.no_id`) | Sekolah dengan lebih dari 5.000 baris hasil atau esai kehilangan bagian tanpa indikator. Tambahkan pagination, atau pembatas eksplisit dengan header peringatan, plus tiebreaker deterministik. |
| P2-29 | Formula injection pada ekspor CSV | `csv_utility.go:247-256` untuk nama dan jawaban mentah; XLSX di `:372-380` aman | Esai siswa berisi `=cmd|...` dieksekusi saat guru membuka Excel. Tambahkan prefix `'` untuk sel yang diawali `=`, `+`, `-`, `@`, tab, atau CR. |
| P2-30 | Endpoint SSE tidak terbatas dan tidak dipakai frontend | `supervisor_sse.go:44-73` (loop 2 detik selamanya, tanpa batas koneksi maupun durasi, satu query per koneksi per 2 detik dengan pool 25) dibanding `supervisor/+page.svelte:88` yang polling 3 detik | Satu token supervisor bisa membuka ribuan stream dan menghabiskan pool koneksi. Cap durasi dan koneksi per `user_id`, atau hapus endpoint yang tidak dipakai. |
| P2-31 | `PasswordGenerator` tidak menghasilkan password acak | `web/src/lib/components/PasswordGenerator.svelte:24-27,33-35,38-41` memakai `Math.random()` untuk 4 karakter wajib, `sort(() => Math.random()-0.5)` untuk shuffle, serta modulo bias `array[i] % all.length` | Dipakai untuk membuat password siswa dan ruang di halaman admin. Ganti seluruhnya dengan `crypto.getRandomValues` plus rejection sampling, dan shuffle via Fisher-Yates tanpa bias. |
| P2-32 | UI menampilkan password ruang palsu untuk semua ruang | `web/src/routes/admin/rooms/+page.svelte:183-197` memakai `r.password \|\| 'ruang123'`, sementara `handlers/ruang.go:16-20` tidak menyertakan password dalam SELECT | Admin percaya semua ruang berpassword `ruang123`. Hapus kolom dan ikonnya, atau sediakan mekanisme reset password eksplisit. |
| P2-33 | Auto-login QR mati | `admin/students/print-cards/+page.svelte:52-54` hanya menyertakan `no_id` dan `token`, sedangkan `student/login/+page.svelte:27` mensyaratkan `urlNoId && urlPass && urlToken` | Memindai kartu tidak mengisi apa pun. Tentukan kontraknya: menyertakan password berarti risiko, atau isi no_id dan token saja lalu biarkan password diketik manual. |
| P2-34 | `sandbox` iframe tidak efektif, plus `postMessage('*')` tanpa cek origin | `web/src/routes/student/exam/+page.svelte:847` memakai `allow-scripts allow-same-origin`, `:541` mengirim ke `*`, handler di `:546-552`; cermin di `ispring-shim.js:66-69,207-212` | Paket iSpring yang diunggah admin bersifat semi-ternaque dan bisa menyentuh `parent.document` untuk mencuri `localStorage` berisi `aether_token`. Gunakan pengecekan `event.origin` dan sandbox tanpa `allow-same-origin` bila shim tidak butuh DOM. |
| P2-35 | CSP hanya `frame-ancestors`, dan token JWT disimpan di `localStorage` | `internal/api/middleware/security_headers.go:8-15`; `web/src/lib/api.ts:17-20,45-48` | Tidak ada `default-src` maupun `script-src`, dan tidak ada HSTS. Migrasi ke cookie `HttpOnly` dengan `SameSite=Strict` menghapus kelas risiko ini. |
| P2-36 | Harness load menulis ke DB produksi secara default | `tests/load/dataprep.go:93-103` menjalankan `UPDATE settings SET is_exam_active = 1, token = 'ujian2026'`, dengan default `-db data/cbt_aether.db` | `go run ./tests/load/` pada instalasi sekolah akan mengaktifkan ujian dan mengganti token bersama. Wajibkan `-allow-write` plus konfirmasi tenant, dan refuse saat `is_exam_active = 1`. |
| P2-37 | Tiga perintah memakai tiga nama env berbeda | Server memakai `DATABASE_URL` (`config.go:47`), `cmd/seed/main.go:12` hardcode `data/cbt_aether.db`, `cmd/migratepasswords` memakai `DATABASE_PATH` | `DATABASE_URL=... go run ./cmd/seed` menulis ke DB **lain** dan melaporkan sukses. Sammakan semua ke `DATABASE_URL` plus flag `-db`, dan cetak path absolut sebelum menulis. |
| P2-38 | `/api/health` hanya stub, namun dipakai sebagai gate verifikasi | `cmd/server/main.go:157-159` hanya mengembalikan `{"status":"ok"}`; dipakai di `docs/runbooks/queue-and-litestream.md:241,274`, `docs/production-readiness.md:46`, dan pre-flight `tests/load/client.go` | Hijau tidak berarti sistem berfungsi, dan itulah alasan P0-5 tidak terdeteksi. Pisahkan `/api/health` untuk liveness dari `/api/ready` yang memeriksa Ping, `VerifyPragmas`, queue writable, dan jumlah FK violation. |
| P2-39 | Observability minim | Hanya `GET /api/debug/queue` (`main.go:208`, admin-only, empat hitungan folder); sisanya `log.Printf` tanpa request-id; peringatan FK violation di `migrate.go:110,148` hanya dicetak sekali saat startup dan tidak disimpan | Radius mustahil. Ekspor queue depth dan failed count sebagai metrics, dan simpan ringkasan FK ke tabel `startup_warnings` yang diekspos di `/api/ready`. |
| P2-40 | Error `rows.Scan` diabaikan pada listing users | `handlers/user.go:38` memanggil `rows.Scan(...)` tanpa cek error | Baris rusak diam-diam menghasilkan entri dengan field nol. |
| P2-41 | `getSettingsForTenant` mengabaikan error insert seed | `handlers/settings_handler.go:67-70` memakai `_, _ = db.DB.Exec(...)` | Bila insert gagal, misalnya karena FK, token yang dikembalikan ke admin tidak pernah tersimpan, sehingga admin menyimpan token yang tidak berfungsi tanpa error. |

---

## 7. Temuan P3

| # | Temuan | Bukti |
|---|---|---|
| P3-42 | 76 file Go tidak lulus `gofmt -l` | Termasuk `cmd/server/main.go`, seluruh handler inti, dan seluruh `internal/submission` |
| P3-43 | Nol test frontend dan nol tooling | Tidak ada `*.test.*`, `tsconfig.json`, ESLint, Prettier, `svelte-check`, maupun Playwright di `web/`; `web/package.json:5-9` hanya punya `dev`, `build`, `preview`; `puppeteer` di root `package.json:19` tidak dipakai script mana pun |
| P3-44 | Kompresi precompress dimatikan | `web/svelte.config.js:9-15` memakai `precompress: false`, dan Go tidak melayani header kompresi di `cmd/server/main.go` |
| P3-45 | `@sveltejs/adapter-auto` terpasang tapi tidak dipakai | `web/package.json:11` dibanding `svelte.config.js:1` |
| P3-46 | Dead code | `internal/submission/queue.go` (`InMemoryQueue`, `SQLiteQueue`, `BufferedSQLiteQueue` tidak dipakai di produksi), `web/src/lib/components/ui/Timer.svelte`, resource helper `api.ts:78-99` tanpa call site, dan `fmtTime` duplikat di `exam/+page.svelte:413-421` dibanding `web/src/lib/timer.ts:41` |
| P3-47 | Dark mode dikonfigurasi tapi tidak pernah diaktifkan | `web/tailwind.config.js:4` memakai `darkMode:'class'`, tidak ada toggle, sehingga varian `dark:` di `ui/Toast.svelte` dan `ui/ConfirmModal.svelte` adalah CSS mati |
| P3-48 | `svelte-ignore` membungkam a11y | `Modal.svelte:66` dan `admin/rooms/+page.svelte:188`. `Modal` tidak punya focus trap maupun focus restore, dan `aria-labelledby="modal-title"` bersifat statis sehingga duplikat id bila ada lebih dari satu modal |
| P3-49 | `localStorage.clear()` dipakai sebagai logout | `exam/+page.svelte:719` dan `student/select-subject/+page.svelte:114` menghapus seluruh storage origin |
| P3-50 | Font dari CDN eksternal | `web/src/app.html:7-9` adalah hard dependency ke `fonts.googleapis.com` untuk produk yang dirancang offline-first, sekaligus render-blocking |
| P3-51 | `favicon.png` tidak ada | `web/src/app.html:5` merujuk `%sveltekit.assets%/favicon.png`, tidak ada di `web/static/`, sehingga 404 dilayani SPA fallback sebagai HTML (`main.go:304-317`) |
| P3-52 | Guard role duplikat di tiga handler | `supervisor.go:248-250,278-280,321-323` mengulang guard yang sama. Menariknya guard room-scope **hanya** ada di `Unlock` (`:336-346`) dan `RecordInfraction` (`anticheat_handler.go:58-61`), dan itulah akar P1-6 |
| P3-53 | `opencode.json` project menunjuk path plugin versi lama | Path seperti `frontend-design/unknown`, `superpowers/5.1.0`, dan `atomic-agents/324399402b9b` sudah tidak ada di cache lokal (`6d5f79446b99`, `6.1.1`, `94220182f88d`). File ini untracked jadi tidak berdampak ke repo, tetapi membingungkan |

---

## 8. Testing, CI, dan Dependensi

### 8.1 Tidak ada CI sama sekali

Tidak ada direktori `.github/`. Padahal repo punya 58 file test Go, termasuk property test (`pgregory.net/rapid`) untuk queue, job, processor, scheduling, dan supervisor, plus golden test untuk ekspor. Gate yang ada hanya manual (`go test`, `npm run build`). Konsekuensinya, P0-1 sampai P0-5 semuanya lolos suite yang hijau.

### 8.2 Gap test yang menjadi rumah bagi temuan P0 dan P1

Tidak ada test yang mengassert hal-hal berikut:

1. `ResetStudentSession` **menolak** peserta di luar ruang supervisor (P1-6).
2. Webhook **menolak** submission tanpa `dr` (P0-1).
3. Processor **menolak** overwrite hasil yang sudah committed (P0-2).
4. `CreateUser` **menolak** role supervisor (P1-7).
5. Login **rate-limited** (P1-8) dan webhook **413** pada body besar (P1-9).
6. Klien menghentikan retry pada 403 dan 409 (P1-10, P1-11).

### 8.3 Slow test dan race yang belum pernah jalan

`internal/api/handlers` membutuhkan 414 detik karena bcrypt cost 14 dan property test. `go test -race` gagal dijalankan karena tidak ada gcc, sehingga kondisi balapan pada `FilesystemQueue.inFlight`, `worker.stopOnce`, dan `SubQueue.Close()` belum pernah diverifikasi. Tambahkan job Linux, atau Windows dengan MSYS2, untuk menjalankan `-race` pada `internal/submission`.

### 8.4 Dependensi

| Lokasi | Temuan |
|---|---|
| `web/` | 14 vulnerability: `shell-quote` critical via `concurrently` di root, `SvelteKit <=2.70.3` (prototype pollution di form remote function, DoS via header `Accept`), `browserslist` dan `nanoid` dan `postcss` high, `vite`/`esbuild` (exposur request pada dev server), `cookie`, `devalue` |
| root | 3 vulnerability: `shell-quote` critical dua kali, `js-yaml` high untuk merge-key DoS |
| Go | `golang.org/x/crypto v0.48.0`, `modernc.org/sqlite v1.50.1`, `excelize v2.10.1` terlihat mutakhir, tetapi `govulncheck` tidak tersedia untuk verifikasi |

Eksploitabilitas sebagian besar ada di jalur build dan dev, bukan runtime produksi. Prioritas: `npm audit fix` yang non-breaking, lalu update SvelteKit dan Vite mayor dengan pengujian ulang.

---

## 9. Dokumentasi yang Bertentangan dengan Kode

| Klaim di dokumen | Kenyataan |
|---|---|
| `docs/Database_Schema.md:355` membuat tabel `migrations` | Tabel **tidak ada** di 31 file `.sql` maupun di kode Go |
| `docs/Database_Schema.md:362-377` tabel `activity_logs` untuk audit | **Tidak ada** di mana pun |
| `docs/Database_Schema.md:338` menyatakan `idx_cek_login_unique_exam_session` di-drop pada migration 025 | `migrations/025:16-23` menyatakan indeks itu **sengaja tidak di-drop**, dan `internal/db/migrate_test.go` masih mengassert keberadaannya |
| `docs/backup-restore.md:45` "Membuka database dengan mode WAL yang benar" | DSN tool diabaikan driver sehingga `journal_mode=delete` (P1-15) |
| `docs/backup-restore.md:47,56` `integrity_check` pada file backup | Dijalankan pada DB **sumber** (P0-5) |
| `docs/backup-restore.md:117` troubleshooting "Hapus manual file `.db-wal` dan `.db-shm`" | Presisi **merusak data** yang persis dianalisis di P1-17 |
| `DEPLOYMENT_LINUX_VPS.md:63` aplikasi menolak start jika secret kosong atau lemah | Hanya cek kosong di `config.go:64-66` |
| `DEPLOYMENT_COOLIFY.md:43` `JWT_SECRET` WAJIB | Tidak ada mekanisme pemaksa, dan image punya default (P0-3) |
| `docs/runbooks/queue-and-litestream.md:22` login sebagai admin atau supervisor untuk `/api/debug/queue` | Route `adminOnly` saja (`main.go:208`) |
| `docs/runbooks/queue-and-litestream.md:243,257` restart server akan memindahkan `processing/` ke `pending/` | Hanya file yang **lebih tua dari** `QUEUE_STUCK_THRESHOLD_MIN` (default 5 menit, didokumentasikan sendiri di `:166`), sehingga restart tunggal tidak cukup |
| `tests/load/README.md:8` "GAGAL TOTAL untuk WRITE-HEAVY, migrasi ke PostgreSQL sangat direkomendasikan" | Dikontradiksi addendum `:260-268` dan `docs/production-readiness.md:9-11` |
| `tests/load/README.md:16` rate limiter dinonaktifkan ke 100000 per menit | Tidak ada panduan ops yang menyebut `WEBHOOK_RATE_LIMIT_PER_MIN`, sementara default produksi 100 per menit per IP (P1-10) |
| `tests/load/README.md:262` webhook me-rename ke `queued/` | Direktori sebenarnya `pending/` (`fsqueue.go:161`) |
| `docs/production-readiness.md:12-24` tabel latensi sebagai status terkini | Disusupi `tests/load/E2E_RESULTS.md:124-195` |
| `docs/credential-rotation.md:53` jangan pakai admin123 di produksi | Migration 004 **menyuntikkan ulang** kredensial itu di setiap instalasi (P0-4) |
| `README.md:24-25` klaim server-derived score verification | Verifikasi hanya konsisten-diri atas data kiriman siswa (P0-1) |
| `README.md:15` klaim Multi-Tenant yang Ketat | Header `X-Tenant-ID` dari klien selalu menang, dengan default tenant 1 (P1-12) |
| `docs/PRD.md:6,128,179` PWA, Service Worker, IndexedDB | Tidak ada `manifest.webmanifest`, `service-worker.ts`, maupun `static/`; offline diimplementasikan ad-hoc di `localStorage` |

---

## 10. Roadmap Perbaikan

### Fase 0 - Sebelum deploy apa pun (1 sampai 2 hari, risiko rendah, dampak sangat tinggi)

1. **Privatisasi repo** plus rotasi `JWT_SECRET`, password admin, ruang, dan siswa, serta token sesi di setiap instance yang mungkin pernah memakai default (P0-3, P0-4).
2. Hapus `Dockerfile:61`, tambahkan guard entropi dan denylist untuk `JWT_SECRET`, serta set `ENV=production` di image (P0-3).
3. Migrasi `033` menghapus seed admin dari 004, gate pembuatan admin, dan `seed` menolak `ENV=production` (P0-4).
4. `scripts/backup.go`: verifikasi pada file backup dengan `integrity_check` dan `foreign_key_check`, jangan hapus backup, dan pakai `db.DSN()` (P0-5, P1-15).
5. `main.go:333` memakai `log.Fatalf` pada listen failure (P1-18).
6. `scripts/restore.ps1`: salin `.db`, `-wal`, dan `-shm`, serta exit non-nol tanpa konfirmasi (P1-17).
7. Hapus DB dari target clean; `reset_queue.go` minta konfirmasi, filter tenant, dan exit non-nol (P1-16, P1-19).
8. Tambah `.dockerignore`, Healthcheck, USER, dan VOLUME (P1-20).

### Fase 1 - Integritas dan otorisasi (1 sampai 2 minggu)

9. Pipeline skor: `dr` wajib, `skor_maks` dari `DerivedScore`, processor immutable after commit plus tabel replay, dan hapus atau gate jalur legacy `mapel_id` (P0-1, P0-2, P2-21).
10. **Keputusan answer key:** cache key saat upload untuk grade di server. Tanpa ini skor tetap client-asserted.
11. Otorisasi: helper `assertSupervisorOwnsPeserta`, hapus `supervisor` dari `CreateUser`, dan buka `PUT /me` untuk semua role (P1-6, P1-7, P2-26).
12. Hardening endpoint: limiter di tiga login, `BodyLimit` 1 MB plus limit per route untuk upload, timeout Fiber, `middleware.Recover()`, dan rate limit webhook per token (P1-8, P1-9, P1-10, P1-13).
13. Kredensial siswa: hapus default `siswa123`, jangan reset password saat impor CSV, dan cap baris serta password unik (P2-22, P2-23, P2-24).
14. Handshake submission: konfirmasi 2xx sebelum menampilkan "Sesi Selesai", backoff sisi klien, dan hentikan retry pada 4xx (P1-11).
15. Perbaiki tenant resolution di frontend (P1-12) dan `PasswordGenerator` (P2-31).

### Fase 2 - Keandalan data dan engineering hygiene (2 sampai 3 minggu)

16. `schema_migrations` dengan version dan checksum, satu transaksi per file (P1-14).
17. Migrasikan semua tool ke `db.DSN()`, dan harness load tidak boleh menulis ke DB produksi tanpa flag (P1-15, P2-36).
18. `/api/ready` plus `HEALTHCHECK` plus observability untuk queue depth, failed count, dan FK warnings (P2-38, P2-39).
19. Sederhanakan env var: satu `DATABASE_URL` untuk semua perintah (P2-37).
20. `gofmt` 76 file dan hapus dead code (P3-42, P3-46).
21. Perbaiki ekspor dengan pagination, tiebreaker, dan proteksi formula injection (P2-28, P2-29).

### Fase 3 - Frontend, aksesibilitas, dan engineering debt (3 sampai 4 minggu)

22. Tooling frontend: `tsconfig.json`, `svelte-check`, Vitest, ESLint, dan Prettier, plus test untuk `lib/timer.ts` yang sudah dirancang injectable-clock tetapi nol test (P3-43).
23. Pecah `student/exam/+page.svelte` yang berisi 970 baris, 6 interval, dan 5 listener menjadi modul: `submissionClient`, `antiCheat`, `countdown`, `iframeHost` (P3-46).
24. Aksesibilitas: focus trap dan restore di `Modal`, hapus `tabindex` non-interaktif di `Table`, ganti `svelte-ignore` dengan handler keyboard yang benar, dan perbaiki semua warning build (P3-48).
25. Migrasi token ke cookie `HttpOnly`, CSP dengan `script-src`, dan HSTS (P2-35).
26. Batasi atau hapus endpoint SSE (P2-30).
27. Pertimbangkan PWA dan service worker bila PRD masih menuntut (lihat bagian 11).
28. Tambahkan test negatif untuk semua butir Fase 1 beserta job CI untuk gofmt, vet, test, build, svelte-check, audit, dan `-race` di Linux.

---

## 11. Keputusan yang Menunggu Ownersip

1. **Penilaian.** Cache answer key saat upload lalu grade di server dengan effort besar tapi anti-palsu nyata, atau menerima skor client-asserted dan melabelinya demikian di UI serta laporan? Rekomendasi: opsi pertama. Tanpa itu, skor tidak layak menjadi nilai akademik.
2. **Jalur legacy `mapel_id`.** Hapus, atau gate dengan `LEGACY_MAPEL_PATH=false` plus enforcement jendela waktu? Rekomendasi: hapus pada rilis berikutnya, karena jalur ini adalah biang P0-2.
3. **Multi-tenant.** Andalkan subdomain dengan menghapus default `X-Tenant-ID` di frontend, atau satu build per tenant lewat `VITE_TENANT_ID`? Rekomendasi: subdomain, karena klaim multi-tenant saat ini tidak terpenuhi.
4. **Rate limit webhook.** Kuota per `attempt_token` plus kuota global per tenant (rekomendasi), atau sekadar relaxasi default dan dokumentasi? Hanya dengan koordinat per token, NAT bersama menjadi aman.
5. **PWA dan offline.** Kejar Service Worker dan IndexedDB sesuai PRD, atau turunkan scope ke retry `localStorage` yang sekarang? Rekomendasi: turunkan scope. Retry ad-hoc sudah cukup untuk target sekolah, dan service worker menambah failure mode baru di hari-H.
6. **Retensi dan privasi.** Berapa lama `hasil_tes_detail`, `data/queue/done` yang saat ini 7 hari, dan `failed/` disimpan, serta siapa yang boleh menghapus? Saat ini tidak ada kontrol retensi selain `QUEUE_DONE_RETENTION_DAYS` dan tidak ada audit trail perubahan nilai.

---

## 12. Lampiran A - Yang Sudah Diverifikasi Aman, Jangan Diaudit Ulang

- **JWT algorithm confusion dan `alg:none`**: ditolak di `middleware/auth.go:25-27` dan `utils/auth.go:75-77` sebelum kunci dikembalikan, jadi penyerang tidak bisa memilih algoritma maupun kunci. Kekurangan validasi `iss` dan `aud` adalah gap hardening, bukan eksploitabel di sini.
- **Spoofing tenant via `X-Tenant-ID` pada route protected**: `TenantMiddleware` berjalan lebih dulu, tetapi `AuthMiddleware` **menimpa** `tenant_id` dari JWT di `auth.go:53`, dan urutan group middleware menjaminnya. Header tersebut memang selector yang disengaja pada tiga route login publik.
- **XSS lewat shim yang di-inject**: `internal/soalpkg/shim.go:32-43` memakai `json.Marshal` yang meng-escape `<`, `>`, dan `&`, sehingga `no_id` berisi `</script><script>` tidak dapat keluar dari blok `<script>`.
- **Path traversal dan zip-slip saat serve**: `soalpkg.ResolvePath` di `serve.go:17-23` plus `isWithin` di `storage.go:212-221` berjalan untuk setiap asset dan setiap entri zip. `filepath.Join` men-clean sebelum pengecekan, dan entry non-regular serta symlink ditolak di `storage.go:160-163`.
- **Cross-tenant content serving via cookie `aether_exam`**: seluruh rantai ter-scope ke tenant baris `cek_login` di `content_session_service.go:98,110,120,129-131`. Token 256-bit dari `crypto/rand` dan unik 1:1 lewat `migrations/026`.
- **XXE dan entity expansion**: `internal/ispring/parser.go` memakai `encoding/xml` Go yang tidak meresolv entity eksternal, ukuran dan kedalaman dibatasi, dan token 256-bit diperiksa lebih dulu di `ispring.go:46`.
- **SQL injection**: semua query memakai placeholder `?`. SQL yang dirakit dari string hanya berasal dari fragmen konstanta compile-time (`supervisor.go:95-141`, `me.go:103`, `fsqueue.go`) atau nama kolom tetap.
- **Parsing `X-Tenant-ID`**: `tenant.go:115-120` memakai `strconv.Atoi` setelah `TrimSpace`, sehingga ketat.
- **Short-row panic pada CSV**: aman semata karena `encoding/csv` menurunkan `FieldsPerRecord` dari record pertama di `csv_utility.go:92-94`. Rapuh, jadi perlu komentar penjaga.
- **Perbaikan FK `peserta`**: `internal/db/peserta_fk_repair.go` benar-benar transaksional, pragma-safe, idempoten, menolak column set tak terduga, dan punya test preservasi data.
- **Index hot path runtime**: `processor.go:95`, `:110`, `:159`, `:115`, dan `:255` dilayani index yang tepat, diverifikasi lewat `EXPLAIN QUERY PLAN`.
- **Durability queue**: fsync file sebelum rename, atomic rename, `syncDir`, backoff exponential di nama file, dead-letter dengan `.error.txt`, dan recovery saat startup. Defek P1-14 ada di lapisan migration, bukan di queue.

## 13. Lampiran B - Yang Tidak Bisa Diverifikasi pada Audit Ini

- Race detector, karena tidak ada gcc atau CGO.
- `govulncheck`, `gosec`, `golangci-lint`, `staticcheck`, dan `svelte-check`, karena tidak terpasang.
- Perilaku runtime produksi untuk Docker, Coolify, litestream, dan cron backup. Hanya analisis statis. `docs/litestream-config.example.yml` dan `DEPLOYMENT_LINUX_VPS.md` bagian 4 tidak bisa diverifikasi tanpa akses host.
- Perilaku browser untuk Lighthouse, accessibility tree, dan Core Web Vitals. Tidak ada Chrome DevTools MCP terpasang, sehingga temuan a11y di sini berasal dari warning compiler dan pembacaan kode.
- `npm audit` memakai lockfile lokal, sehingga angka advisory dapat berubah setelah advisory baru terbit.

---

*Laporan ini dibuat oleh audit read-only pada commit `f6b05ff`. Tidak ada file source yang diubah. Semua klaim P0 dan P1 disertai pemicu yang dapat dieksekusi atau argumen data-flow yang dapat ditelusuri pada `path:line`. Klaim P2 dan P3 sebaiknya di-spot-check sebelum dijadikan dasar perubahan skrip produksi.*
