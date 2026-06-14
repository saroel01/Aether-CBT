# Implementation Plan

## Overview

Rencana ini menerjemahkan design menjadi langkah implementasi yang inkremental dan dapat diuji. Setiap langkah membangun di atas langkah sebelumnya, mengutamakan validasi awal, dan tidak meninggalkan kode yatim. Setiap task menyertakan acuan _Requirements_.

Konvensi penyelesaian (gerbang kualitas, Requirement 16.5): setiap task backend dianggap selesai hanya bila `go build ./...`, `go vet ./...`, dan `go test ./...` lulus; task frontend bila `npm run build` (di `web/`) lulus.

## Tasks

- [x] 1. Pondasi skema database (migrasi idempoten) — _selesai (1.8–1.10 remediasi selesai)_
  - [x] 1.1 Tambah migrasi `020_alter_kelas_tingkat.sql` (kolom `tingkat` + indeks), mengikuti pola idempoten `RunMigrations`
    - Tulis SQL `ALTER TABLE` + `CREATE INDEX IF NOT EXISTS`
    - _Requirements: 1.2, 14.1_
  - [x] 1.2 Tambah migrasi `021_create_soal_package.sql`
    - _Requirements: 3.6, 14.1, 15.1_
  - [x] 1.3 Tambah migrasi `022_create_exam.sql`
    - _Requirements: 2.1, 14.1, 15.1_
  - [x] 1.4 Tambah migrasi `023_create_exam_session.sql` (+ indeks token)
    - _Requirements: 4.1, 14.1, 15.1_
  - [x] 1.5 Tambah migrasi `024_create_session_kelas_ruang.sql` (relasi kelas & ruang)
    - _Requirements: 5.1, 5.2, 15.1_
  - [x] 1.6 Tambah migrasi `025_alter_cek_login_session.sql` (`session_id`, `locked`, `content_token`, indeks unik sesi). CATATAN: drop indeks unik lama berbasis mapel sengaja ditunda ke task 7.3 (saat `StartExamSession` dikonversi ke basis sesi) agar alur "mulai ujian" tidak rusak di tengah transisi.
    - _Requirements: 7.2, 10.2, 14.1_
  - [x] 1.7 Perluas `internal/db/migrate_test.go`: verifikasi semua migrasi sukses pada DB kosong & rerun idempoten; verifikasi kolom/indeks baru ada
    - _Requirements: 14.1, 14.5_
  - [x] 1.8 _[Remediasi code-review]_ Percanggih `RunMigrations` (`internal/db/migrate.go`): pecah tiap berkas migrasi per-pernyataan (`;`) dan eksekusi satu per satu; tetap menelan error idempotensi ("duplicate column name"/"already exists") tetapi **per-pernyataan**, bukan per-berkas. Tujuan: migrasi yang diterapkan sebagian dapat menyembuhkan diri pada rerun (lihat design AD-8).
    - _Requirements: 14.1, 14.6_
  - [x] 1.9 _[Remediasi code-review]_ Perkuat `TestRunMigrationsIsIdempotentOnRerun`: selain menegaskan `RunMigrations` mengembalikan nil pada rerun, juga verifikasi keberadaan semua objek 020–025 (tabel/kolom/indeks, termasuk `idx_cek_login_unique_session` & `idx_cek_login_content_token`) setelah run ke-2 dan ke-3 menggunakan helper `objectExists`/`tableHasColumn` yang sudah ada.
    - _Requirements: 14.1, 14.5, 14.7_
  - [x] 1.10 _[Remediasi code-review]_ Refaktor helper migrasi (`runMigrationsInTempDB` dkk.) agar tidak bermutasi pada package-global `DB` dan tidak memakai `os.Chdir` (gunakan DB per-test + path absolut ke direktori migrasi, mis. `RunMigrations(dir string)`), atau dokumentasikan batasan non-paralel secara eksplisit agar tes konkuren tidak saling merusak.
    - _Requirements: 16.4, 16.7_

- [x] 2. Model & lapisan repository (tenant-scoped, anti god-file)
  - [x] 2.1 Tambah struct model di `internal/models` (SoalPackage, Exam, ExamSession, relasi, perluasan sesi aktif)
    - _Requirements: 2.1, 3.6, 4.1, 16.1_
  - [x] 2.2 Implementasi `internal/repository/soal_package_repo.go` + test CRUD tenant-scoped
    - _Requirements: 3.9, 15.2, 16.1, 16.4_
  - [x] 2.3 Implementasi `internal/repository/exam_repo.go` + test (validasi referensi mapel/paket, soft delete, tolak hapus bila ada sesi terjadwal/aktif)
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 2.6, 16.1, 16.4_
  - [x] 2.4 Implementasi `internal/repository/exam_session_repo.go` + test (CRUD, relasi kelas/ruang, cek token tumpang tindih, lookup token aktif)
    - _Requirements: 4.1, 4.6, 4.7, 15.2, 16.1, 16.4_
  - [x] 2.5 Implementasi `internal/repository/grade_repo.go` + test (set/list tingkat)
    - _Requirements: 1.1, 1.4, 16.1, 16.4_
  - [x] 2.6 Implementasi `internal/repository/cek_login_repo.go` + test (start/lock/unlock/progress/lookup berbasis session_id)
    - _Requirements: 7.1, 7.2, 10.1, 10.2, 16.1, 16.4_

- [ ] 3. Lapisan service penjadwalan
  - [ ] 3.1 `internal/service/scheduling_service.go`: status efektif sesi (dari waktu server), validasi jendela waktu, keunikan token tumpang tindih, transisi status (tolak terjadwal/aktif tanpa paket)
    - _Requirements: 2.5, 4.2, 4.3, 4.4, 4.5, 4.8_
  - [ ] 3.2 Kelayakan peserta (keanggotaan kelas/ruang, server-side) + test menyeluruh termasuk batas waktu tepat
    - _Requirements: 5.1, 5.2, 5.3, 5.4_
  - [ ] 3.3 Property-based test untuk Property 5, 6, 8 (`pgregory.net/rapid`)
    - _Requirements: 4.4, 4.5, 6.1, 6.3, 7.5_

- [ ] 4. Penyimpanan & penyajian paket iSpring (`internal/soalpkg`)
  - [ ] 4.1 `storage.go`: ekstraksi ZIP aman (anti zip-slip), validasi `index.html`, batas ukuran/jumlah berkas, deteksi versi best-effort, penulisan ke `data/soal/{slug}/{uuid}/`, checksum, cleanup saat gagal
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.6a, 3.7, 15.3_
  - [ ] 4.2 Test `storage` termasuk Property 2 & 3 (zip-slip, non-ZIP, tanpa index.html, cleanup, isolasi tenant) memakai header fixture `contoh_soal/KIMIA_XII_UAS_2025`
    - _Requirements: 3.3, 3.7, 15.3, 16.4_
  - [ ] 4.3 `serve.go`: resolusi path aman (anti traversal) + streaming berkas
    - _Requirements: 8.1, 8.3, 8.4, 8.6, 13.3_
  - [ ] 4.4 `shim.go` + `assets/ispring-shim.js` (go:embed): konten shim & logika titik sisip pada `index.html` (tanpa mengubah berkas disk)
    - _Requirements: 9.1, 9.2, 9.3, 9.5_
  - [ ] 4.5 Test `serve`/`shim`: Property 4 & 12 (penyajian dalam direktori, injeksi hanya pada index.html stream)
    - _Requirements: 8.3, 9.5, 16.4_

- [x] 5. Konfigurasi & connection pool (skala) — _selesai (5.4–5.5 remediasi selesai)_
  - [x] 5.1 Tambah field konfigurasi baru di `internal/config/config.go` (DB pool, batas upload, ambang lock, cookie secure)
    - _Requirements: 3.2, 10.6, 13.1_
  - [x] 5.2 Set `SetMaxOpenConns/SetMaxIdleConns/SetConnMaxLifetime` di `internal/db/sqlite.go` (`Connect`) dari konfigurasi
    - _Requirements: 13.1_
  - [x] 5.3 Perbarui `.env.example` dengan variabel baru + nilai aman
    - _Requirements: 13.1, 10.6_
  - [x] 5.4 _[Remediasi code-review]_ Tetapkan satu sumber kebenaran untuk default pool (25/10/30m): jadikan `db.DefaultPoolConfig()` kanonik dan `config.Load()` membaca nilai darinya (atau sebaliknya), lalu hapus literal default yang terduplikasi antar paket.
    - _Requirements: 13.1, 13.7_
  - [x] 5.5 _[Remediasi code-review]_ Selesaikan opt-out mati di `applyPoolConfig`: hapus ketiga cabang `if pool.X > 0` beserta komentar yang menyesatkan (nilai selalu positif sehingga ketiga batas wajib selalu diterapkan), ATAU bila 0 bermakna "tak terbatas/matikan", sediakan parser env khusus pool yang mengizinkan 0 secara konsisten.
    - _Requirements: 13.1, 16.6_

- [ ] 6. Handler admin (tipis) + routing
  - [ ] 6.1 Handler tingkat kelas (`PUT /api/classes/:id/tingkat`) + wiring route + test
    - _Requirements: 1.1, 1.5, 16.2_
  - [ ] 6.2 Handler paket soal (upload multipart, list, delete tak-tertaut) + wiring + test (403 untuk non-admin)
    - _Requirements: 3.1, 3.8, 3.9, 3.10, 16.2_
  - [ ] 6.3 Handler exam (list/create/update/delete) + wiring + test
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 16.2_
  - [ ] 6.4 Handler exam-session (list/create/update/delete, tautkan kelas/ruang) + wiring + test (tolak entitas lintas-tenant)
    - _Requirements: 4.1, 4.6, 4.7, 5.1, 5.2, 16.2_

- [ ] 7. Alur siswa berbasis sesi
  - [ ] 7.1 Perluas `StudentLogin` (`exam.go`): validasi token terhadap sesi efektif; bedakan "belum dimulai"/"berakhir"; sertakan sesi yang berhak; pertahankan bcrypt/plaintext
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_
  - [ ] 7.2 Endpoint `GET /api/student/my-sessions` (sesi yang dapat dimasuki sekarang) + test
    - _Requirements: 5.3, 6.4_
  - [ ] 7.3 Perluas `StartExamSession`: referensi `session_id`, terbitkan `attempt_token`, set cookie konten `aether_exam`, tolak di luar jendela/locked
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 8.2_
  - [ ] 7.4 Perluas `GetRemainingTime`: `min(durasi, akhir_sesi − now)`, clamp 0 + test (Property 6)
    - _Requirements: 7.5_
  - [ ] 7.5 Debounce/idempotensi progres: ubah `UpdateStudentProgress` jadi UPSERT ringan; kurangi frekuensi (kontrak server)
    - _Requirements: 13.2_

- [ ] 8. Penyajian konten terotorisasi + cookie sesi konten
  - [ ] 8.1 `content_session_service.go`: terbitkan/validasi `content_token`, penegakan tenant + jendela waktu + lock
    - _Requirements: 8.1, 8.2, 8.5, 10.3, 15.5_
  - [ ] 8.2 Handler `GET /api/exam/content/*` (validasi cookie → sesi aktif → stream + shim) + wiring (di luar AuthMiddleware Bearer)
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6_
  - [ ] 8.3 Test integrasi penyajian: pemilik sah vs bukan pemilik (403), di luar jendela (403), locked (403), traversal (400/404)
    - _Requirements: 8.2, 8.3, 10.3, 16.4_

- [ ] 9. Anti-cheat server-enforced
  - [ ] 9.1 Perluas `RecordInfraction`: increment di server, set `locked` saat ambang tercapai (konfigurasi)
    - _Requirements: 10.1, 10.2, 10.6_
  - [ ] 9.2 Tegakkan `locked` di start (7.3), penyajian konten (8.2), dan progres; pengawas unlock via reset
    - _Requirements: 10.3, 10.4_
  - [ ] 9.3 Test Property 11 (penolakan saat locked di semua jalur; grace-period existing tetap lulus)
    - _Requirements: 10.3, 10.5, 16.4_

- [ ] 10. Webhook & kunci validasi berbasis sesi
  - [ ] 10.1 Ubah pembentukan `validasi` ke `tenant_id_noID_sessionID` di handler webhook & processor; pastikan UPSERT `hasil_tes` tetap benar
    - _Requirements: 14.2_
  - [ ] 10.2 Perbarui/penyesuaian test webhook & processor (idempotensi kiriman ulang, Property 9)
    - _Requirements: 14.2, 16.4_
  - [ ] 10.3 Verifikasi `cek_login` dihapus per sesi yang tepat setelah hasil masuk (tidak memengaruhi sesi lain)
    - _Requirements: 11.3_

- [ ] 11. Pemantauan pengawas berbasis sesi
  - [ ] 11.1 Perluas `GetRoomStatus`/SSE agar berbasis `session_id` + status (belum login/mengerjakan/terkunci/terkirim) per sesi
    - _Requirements: 11.1, 11.2, 11.5_
  - [ ] 11.2 Perluas `ResetStudentSession` agar menargetkan (tenant+peserta+session) dan membuka lock
    - _Requirements: 10.4, 11.3_
  - [ ] 11.3 Test akses role (student ditolak) + isolasi tenant
    - _Requirements: 11.4, 11.5, 16.4_

- [ ] 12. Migrasi data warisan (kompatibilitas)
  - [ ] 12.1 Util migrasi data di Go (dipanggil setelah `RunMigrations`): buat exam+session warisan dari `settings` bila tenant belum punya sesi (penjagaan idempoten "hanya bila belum ada")
    - _Requirements: 14.3_
  - [ ] 12.2 Test: rerun tidak menduplikasi; instalasi lama tetap dapat login pada masa transisi
    - _Requirements: 14.3, 14.4, 16.4_

- [ ] 13. Frontend admin (SvelteKit)
  - [ ] 13.1 UI tingkat pada manajemen kelas (pakai `apiUrl`/`authHeaders`)
    - _Requirements: 12.1, 12.5_
  - [ ] 13.2 UI manajemen paket soal: upload ZIP (progress), list, delete; umpan balik sukses/gagal
    - _Requirements: 12.4, 12.5, 12.6_
  - [ ] 13.3 UI manajemen ujian: buat/sunting + tautkan paket
    - _Requirements: 12.2, 12.5_
  - [ ] 13.4 UI manajemen sesi: jendela waktu, token (auto-generate), kelas, ruang, status efektif
    - _Requirements: 12.3, 12.5, 12.6_

- [ ] 14. Frontend siswa (SvelteKit) — konten nyata
  - [ ] 14.1 Ganti `generateQuestions()`/XML tiruan di `student/exam/+page.svelte` dengan embed konten dari `/api/exam/content/...` (iframe same-origin)
    - _Requirements: 8.1, 9.1_
  - [ ] 14.2 Sesuaikan alur login/pilih-sesi/start agar menyimpan konteks sesi & memuat konten; pertahankan pelaporan progres (debounced) & infraction
    - _Requirements: 6.4, 7.3, 10.1, 13.2_
  - [ ] 14.3 Pastikan `npm run build` lulus dan tidak ada hardcode URL/token
    - _Requirements: 12.5, 16.5_

- [ ] 15. Pengujian skala & verifikasi shim
  - [ ] 15.1 Sediakan paket iSpring lengkap (termasuk `data/player.js`) sebagai fixture uji runtime; dokumentasikan jika hanya verifikasi manual yang memungkinkan
    - _Requirements: 9.6_
  - [ ] 15.2 Uji shim (headless/puppeteer bila fixture lengkap tersedia): konten dimuat, hasil dialihkan ke `/api/ispring/webhook` dengan `attempt_token`/`tenant_id`/`sid`
    - _Requirements: 9.1, 9.2, 9.3, 9.4_
  - [ ] 15.3 Perluas `tests/load/` ke skenario ~500 peserta (login/start/progress/submit) + verifikasi tanpa kehilangan hasil (Property 10)
    - _Requirements: 13.1, 13.4, 13.5, 13.6_

- [ ] 16. Gerbang kualitas akhir & dokumentasi
  - [ ] 16.1 Jalankan `go build ./...`, `go vet ./...`, `go test ./...`, dan `npm run build`; perbaiki temuan
    - _Requirements: 16.5_
  - [ ] 16.2 Perbarui dokumentasi (`docs/Database_Schema.md`, `docs/Technical_Architecture.md`, `README`, panduan deployment) agar konsisten dengan model sesi & penyajian konten; catat panduan mode kiosk (deployment)
    - _Requirements: 14.2, 14.3_

## Task Dependency Graph

```json
{
  "waves": [
    { "wave": 1, "tasks": ["1", "5"], "rationale": "Pondasi skema DB dan konfigurasi/connection pool tidak bergantung pada apa pun dan memengaruhi semua task berikutnya." },
    { "wave": 2, "tasks": ["2"], "rationale": "Model & repository bergantung pada skema (1)." },
    { "wave": 3, "tasks": ["3", "4"], "rationale": "Service penjadwalan dan paket soal (storage/serve/shim) bergantung pada model/repository (2); keduanya independen satu sama lain." },
    { "wave": 4, "tasks": ["6", "7"], "rationale": "Handler admin bergantung pada service (3); alur siswa berbasis sesi bergantung pada repository/service (2,3)." },
    { "wave": 5, "tasks": ["8", "9", "10", "12"], "rationale": "Penyajian konten (8) butuh soalpkg(4)+siswa(7)+config(5); anti-cheat(9) & webhook(10) butuh alur siswa(7); migrasi warisan(12) butuh 1,2,3." },
    { "wave": 6, "tasks": ["11", "13", "14"], "rationale": "Monitoring pengawas(11) butuh webhook/sesi(10); frontend admin(13) butuh handler admin(6); frontend siswa(14) butuh penyajian konten(8)." },
    { "wave": 7, "tasks": ["15"], "rationale": "Uji skala & verifikasi shim butuh soalpkg(4), siswa(7), konten(8), frontend siswa(14)." },
    { "wave": 8, "tasks": ["16"], "rationale": "Gerbang kualitas akhir & dokumentasi bergantung pada seluruh task." }
  ]
}
```

Jalur kritis: 1 → 2 → 4 → 8 → 14 → 15 (pengiriman konten nyata end-to-end). Penjadwalan (3 → 6 → 13) dapat berjalan paralel setelah 2. Task 5 (pool/config) independen dan sebaiknya dikerjakan awal karena memengaruhi skala 8 & 15.

## Notes

- **Urutan migrasi**: penomoran melanjutkan dari `019_*` yang ada. Migrasi 020–025 wajib idempoten sesuai pola `RunMigrations` (lihat design). Migrasi data warisan (task 12) berupa util Go pasca-`RunMigrations`, bukan SQL, karena memerlukan logika deterministik berpenjagaan.
- **Fixture iSpring (terverifikasi tidak lengkap)**: `contoh_soal/KIMIA_XII_UAS_2025 (Published)` hanya memuat `index.html`, `ismplayer.html`, `metainfo.xml`, `preview.png` — tanpa `data/player.js`. Task 4.2 hanya memakai header `index.html` untuk deteksi versi; uji runtime shim (15.2) memerlukan paket lengkap (15.1) atau verifikasi manual terdokumentasi.
- **Anti god-file**: akses data baru selalu via repository (task 2); handler tetap tipis (task 6, 7, 8); penyimpanan/penyajian/shim dipisah ke `internal/soalpkg` (task 4). Tidak ada satu berkas yang menggabungkan upload+serve+schedule+monitor.
- **Kompatibilitas**: tidak ada kolom/tabel existing dihapus; `settings.token`/`is_exam_active` dipertahankan selama transisi (task 12). Perubahan kunci `validasi` tidak mengubah struktur indeks `hasil_tes`.
- **Gerbang kualitas** dijalankan per task dan final di task 16.1.
- **Remediasi code-review fondasi**: temuan review pada fondasi ditambahkan sebagai subtask 1.8–1.10 (runner migrasi per-pernyataan, verifikasi objek setelah rerun, helper tes paralel-aman) dan 5.4–5.5 (sumber tunggal default pool, hapus opt-out mati). Task 1 dan 5 kembali dibuka (`[ ]`) hingga remediasi selesai. Kerjakan **1.8–1.10 sebelum melanjutkan ke wave 2 (task 2)** karena peningkatan runner migrasi & pengujian idempotensi adalah prasyarat keandalan seluruh migrasi 020–025.
