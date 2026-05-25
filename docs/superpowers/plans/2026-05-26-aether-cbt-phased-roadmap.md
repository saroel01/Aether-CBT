# Aether CBT Phased Roadmap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:**  
Menyusun dan mendokumentasikan rencana bertahap (Phased Roadmap) yang realistis untuk membawa Aether CBT dari kondisi Hardened MVP menjadi sistem yang siap digunakan untuk ujian nyata di sekolah.

**Architecture:**  
Roadmap disusun berdasarkan analisis risiko tertinggi terlebih dahulu, dengan pemisahan yang jelas antara fase pra-pilot, pasca-pilot, dan jangka panjang. Setiap fase memiliki gate yang terukur dan deliverables yang konkret. Rencana ini memanfaatkan skill global Superpowers untuk meningkatkan kualitas perencanaan dan eksekusi.

**Tech Stack:**  
Go (Fiber), SQLite (WAL), Svelte 5 + SvelteKit, Tailwind, iSpring QuizMaker

**Tanggal Penyusunan:** 26 Mei 2026  
**Status:** Draft Awal

---

## Latar Belakang

Setelah menyelesaikan Phase 0 (Security Hardening) yang mencakup perbaikan JWT, penghapusan secret lemah, CORS allow-list, webhook protection, dan penguatan tenant isolation, proyek membutuhkan rencana yang terstruktur untuk menyelesaikan sisa gap yang masih ada sesuai HANDOFF.md.

Rencana ini disusun dengan bantuan skill `superpowers-writing-plans` dan `superpowers-executing-plans`.

---

## Current State (Mei 2026)

**Sudah Selesai (Phase 0):**
- JWT dengan validasi algoritma + secret wajib
- CORS menggunakan allow-list
- Webhook dilindungi rate limiting + body limit
- Tenant validation lebih ketat di produksi
- Dokumentasi dan skill sudah diperbarui

**Masih Terbuka (Sisa Gap):**
- Load testing
- Credential rotation
- Login rate limiting penuh
- Legacy password handling
- Frontend dependency upgrade

---

## Rencana Bertahap

### Phase 1: Pre-Pilot Readiness (Paling Kritis)

**Tujuan:** Membuat sistem cukup aman dan andal untuk menjalankan ujian pertama di sekolah.

**Target:** Selesai sebelum ujian pilot pertama.

#### Task 1.1: Real iSpring QuizMaker Fixtures

**Status:** ✅ Completed (2026-05-25)

**Files:**
- Created: `tests/fixtures/ispring/kimia-xii-uas-2025-real-20260525.xml` (real production XML from published quiz)
- Modified: `internal/ispring/parser_test.go` (added `TestParseDetailedResultsHandlesRealProductionFixture`)
- Test: `internal/ispring/parser_test.go`

**Achieved:**
- Captured authentic iSpring v2 XML (45 questions, all 5 core types: MultipleChoice, MultipleResponse, True/False, Matching, Sequence) via capture server + manual browser submission from the real published UAS Kimia quiz.
- Parser passes cleanly on the fixture (no changes needed).
- Test asserts version="2", summary (21.9%, passed=false), passingPercent=25, and question volume.

- [x] **Step 1:** Obtained real XML via published quiz (practical alternative to direct school samples)
- [x] **Step 2:** Simpan fixture ke folder `tests/fixtures/ispring/`
- [x] **Step 3:** Added comprehensive real-fixture test exercising production data
- [x] **Step 4:** Parser runs with zero errors on real fixture
- [x] **Step 5:** Documented via roadmap update + fixture itself (see also docs/how-to-get-real-ispring-xml.md)

#### Task 1.2: Backup & Restore Procedure

**Status:** ✅ Completed (2026-05-25)

**Files:**
- Created: `scripts/backup.go` (core backup tool menggunakan VACUUM INTO)
- Created: `scripts/backup.ps1` (wrapper PowerShell)
- Created: `scripts/restore.ps1`
- Created: `docs/backup-restore.md`

**Achieved:**
- Backup menggunakan metode `VACUUM INTO` (paling aman untuk WAL + modernc driver).
- Script backup otomatis melakukan integrity check.
- Script restore dengan konfirmasi keras + pembersihan WAL/SHM.
- 2 siklus penuh backup → hapus data → restore berhasil.
- Dokumentasi lengkap dalam bahasa Indonesia.

- [x] **Step 1:** Riset cara backup SQLite WAL yang aman (modernc + VACUUM INTO)
- [x] **Step 2:** Buat skrip backup yang menangani `.db` + WAL dengan benar
- [x] **Step 3:** Buat prosedur restore lengkap + verifikasi integritas
- [x] **Step 4:** Dokumentasikan prosedur di `docs/backup-restore.md`
- [x] **Step 5:** Lakukan uji coba backup → hapus data → restore (2x berhasil)

#### Task 1.3: Credential Rotation Workflow

**Status:** ✅ Completed (2026-05-25)

**Files:**
- Created: `docs/credential-rotation.md` (prosedur lengkap + checklist)
- Created: `scripts/generate-password.ps1` (rekomendasi untuk Windows)
- Created: `scripts/generate-password.go` (alternatif portabel)
- Updated: `USAGE_GUIDE.md` dan `HANDOFF.md`

**Achieved:**
- Prosedur rotasi yang jelas untuk Admin, Ruang, Siswa, Global Token, dan JWT_SECRET.
- Tool generator password yang aman (menggunakan kriptografi).
- **Production-ready**: Standalone executable (`cmd/password-generator`) + instruksi ringkas untuk admin sekolah yang hanya punya build.
- Peringatan kuat di dokumentasi utama.
- Checklist rotasi sebelum pilot/deployment.

- [x] **Step 1:** Buat prosedur wajib rotasi password sebelum setiap deployment/pilot
- [x] **Step 2:** Buat script/tools sederhana untuk generate password baru
- [x] **Step 3:** Update dokumentasi (USAGE_GUIDE.md dan HANDOFF.md)
- [x] **Step 4:** Tambahkan referensi prosedur di dokumentasi (proses deployment disarankan mengikuti checklist di docs/credential-rotation.md)

#### Task 1.4: Basic Load Testing (Expanded Scope)

**Status:** ✅ Completed (2026-05-25)

**Scope Expansion (Full Mode):**  
Menguji **semua endpoint koneksi masal** siswa dan pengawas, dilakukan secara bertahap dari 100 → 300 → 500 concurrent users.

**Files:**
- `tests/load/main.go` — CLI entry point
- `tests/load/metrics.go` — Metrics collector (p50/p95/p99, throughput, error classification)
- `tests/load/client.go` — Reusable HTTP client (login, start, webhook, progress, etc.)
- `tests/load/dataprep.go` — Database data preparation & cleanup
- `tests/load/scenario_priority.go` — Login, Start Exam, Submission Burst
- `tests/load/scenario_during.go` — During-Exam Mixed Load
- `tests/load/scenario_full.go` — Full Exam Cycle
- `tests/load/README.md` — Complete results & analysis

**Key Findings:**
- Login (READ+WRITE): PASS hingga 500 concurrent (2300 req/s, P95=376ms)
- During-Exam (READ-heavy): PASS at 100 concurrent (99% success, P95=20ms)
- Webhook Submission (WRITE-heavy): **FAIL** above ~5-10 concurrent (0.4-2.5% success)
- SQLite WAL single-writer lock adalah bottleneck fatal untuk concurrent writes

**Bug Fixed:** Webhook route auth middleware leak (moved route before protected group)

**Recommendation:** Migrasi ke PostgreSQL untuk production. SQLite hanya cocok untuk pilot kecil (< 50 siswa).

- [x] **Step 1:** Identifikasi semua endpoint koneksi masal + tentukan skenario realistis
- [x] **Step 2:** Bangun framework load test yang fleksibel (multiple scenarios + phased concurrency)
- [x] **Step 3:** Jalankan uji beban bertahap (100 → 300 → 500) + ukur SQLite WAL behavior
- [x] **Step 4:** Dokumentasikan hasil lengkap, bottleneck, dan batas praktis
- [x] **Step 5:** Berikan rekomendasi arsitektur & tuning untuk produksi (termasuk kemungkinan migrasi DB)

---

### Phase 2: Post-Pilot Hardening

**Tujuan:** Memperkuat sistem setelah mendapatkan pengalaman nyata dari ujian pertama.

**Target:** 1–3 bulan setelah ujian pilot pertama.

- [ ] Login rate limiting untuk seluruh endpoint sensitif
- [ ] Monitoring dan alerting dasar
- [ ] Legacy Password Rotation Plan
- [ ] Perbaikan dokumentasi berdasarkan pengalaman lapangan

---

### Phase 3: Long-term Maturity

**Tujuan:** Meningkatkan kualitas jangka panjang dan kesiapan skala lebih besar.

- [ ] Frontend Dependency Upgrade Plan (npm audit)
- [ ] Automated Backup Solution
- [ ] Comprehensive Load & Stress Testing
- [ ] Advanced Observability (jika diperlukan)

---

## Rekomendasi Penggunaan Skill Global

Saat mengeksekusi roadmap ini, disarankan menggunakan skill berikut:

| Fase / Aktivitas                     | Skill Global yang Direkomendasikan                  |
|--------------------------------------|-----------------------------------------------------|
| Menyusun atau merevisi roadmap       | `superpowers-writing-plans`                         |
| Menjalankan fase demi fase           | `superpowers-executing-plans`                       |
| Debugging isu kompleks               | `superpowers-systematic-debugging`                  |
| Verifikasi sebelum selesai           | `superpowers-verification-before-completion`        |
| Pekerjaan besar & paralel            | `superpowers-subagent-driven-development`           |

---

## Lampiran

- Lihat `HANDOFF.md` untuk daftar gap asli.
- Lihat `docs/superpowers/plans/2026-05-25-production-hardening.md` untuk pekerjaan sebelumnya.
- Semua skill global tersedia di `.opencode/skills/superpowers-*`

---

**End of Plan**
