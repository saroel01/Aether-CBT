# Handoff — Exam Scheduling & iSpring Delivery

**Status: 7 / 16 tasks complete, all on `main`, `go build/vet/test ./...` green.**
Last updated: 2026-06-14. Read this top-to-bottom before continuing.

This file is a working handoff for the agent picking up the exam-scheduling spec.
The source of truth for *what* to build is the spec trio in this folder
(`requirements.md`, `design.md`, `tasks.md`); this file captures *how* the work has
been done so far, the conventions you MUST follow, and exactly where to resume.

---

## 0. Resume in 4 steps

1. **Toolchain prep.** Go is NOT on PATH. Prepend it in every shell:
   ```bash
   export PATH="/c/Program Files/Go/bin:$PATH"   # go 1.26.4, module go 1.25.0
   ```
   Git identity is set repo-local (`Syahrul Hamdi <saroel.hamdi@gmail.com>`); if a fresh
   clone drops it, re-run `git config user.name "Syahrul Hamdi" && git config user.email "saroel.hamdi@gmail.com"`.
2. **Read the spec trio** (`requirements.md`, `design.md`, `tasks.md`) — tasks 1–7 are
   `[x]`, tasks 8–16 are `[ ]`.
3. **Confirm green** before touching anything:
   ```bash
   go build ./... && go vet ./... && go test ./...
   ```
4. **Resume at Task 8** (content serving) — see §4. It is the critical path
   (`1→2→4→8→14→15`) and unblocked.

---

## 1. What's done (commits on `main`, newest last)

| Commit | Task | Notes |
|---|---|---|
| `823699b` | foundation scaffold (1.1–1.7, 5.1–5.3) | migrations 020–025, config fields, pool wiring |
| `bf7a60b` | **1** + **5** remediation (1.8–1.10, 5.4–5.5) | per-statement migration runner; single-source pool; dead opt-out removed |
| `a9386be` | **2** models + repositories | ~41 repo tests |
| `83e47c6` | **3** scheduling service | 6 unit + 3 property (rapid) tests |
| `f2b6cf4` | **4** `internal/soalpkg` | storage/serve/shim, 14 security tests |
| `2e583b1` | (fix) Fiber BodyLimit | raised to upload cap so 15–20 MB zips work |
| `dafbd81` | **6** admin handlers + routes | 14 handler tests |
| `29c26d7` | **7** student session flow | 8 handler tests, legacy fallback retained |

Module: `github.com/saroel01/aether-cbt`. Backend stack: Go/Fiber + SQLite WAL + modernc
driver + SvelteKit frontend (`web/`, not yet touched).

---

## 2. Architecture & conventions (follow these)

### Layering (anti god-file, Req 16.1–16.3)
```
handlers (thin: parse, map errors, respond)  → internal/api/handlers
service   (cross-entity rules)               → internal/service
repository(tenant-scoped data access)        → internal/repository
soalpkg   (filesystem storage/serve/shim)    → internal/soalpkg
db        (migrations, pool, global *sql.DB) → internal/db
```

### Repository pattern (IMPORTANT — differs from the old `TenantRepository`)
New repositories **receive `*sql.DB`** (production wires `db.DB`; tests inject a per-test
DB). They are structs with a `db *sql.DB` field + `NewXxxRepository(db *sql.DB)`
constructor. Do **not** use the package-global `db.DB` inside repos. This is deliberate
(Req 16.7: tests must not mutate global state).

```go
repo := repository.NewExamRepository(db.DB)      // production
repo := repository.NewExamRepository(testDB)     // tests
```

### Tests
- **Shared migrated DB:** `testutil.NewMigratedDB(t) (*sql.DB, func())` — opens a temp
  SQLite, runs the real migrations via `db.RunMigrations`, returns a cleanup. Resolves the
  migrations dir via `runtime.Caller` (works from any package depth).
- **Shared seeders:** `testutil.SeedTenant/SeedKelas/SeedMapel/SeedRuang/SeedPeserta/
  SeedSoalPackage/SeedExam/SeedExamSession`. `internal/repository` tests wrap these as
  unexported `seedX` (see `repository/seed_test.go`).
- **Handler integration tests:** the harness in `handlers/admin_test.go`:
  `newAdminTestApp(t, role)` builds a Fiber app whose Locals (tenant_id/role/user_id) are
  injected by a test middleware (no JWT), plus the real `middleware.RequireRoles`. Helpers:
  `doJSON(...)`, `decodeJSON(...)`, `newMultipartUpload(...)`. **Set `db.DB` is done inside
  the harness** (handler code uses the global `db.DB`).
- **Property tests:** `pgregory.net/rapid` (already a dep) — see
  `service/scheduling_property_test.go`.
- **TDD is the norm.** Watch a test fail (RED) before implementing (GREEN). Several real
  bugs were caught this way; don't skip it.

### Error sentinels (map these to HTTP in handlers)
- `repository`: `ErrNotFound` (→404), `ErrConflict` (→409), `ErrInvalidReference` (→400)
- `service`: `ErrInvalidWindow` (→400), `ErrTokenConflict` (→409), `ErrPackageRequired` (→400)
- `soalpkg`: `ErrNotZip`, `ErrMissingIndex`, `ErrTooLarge` (→413), `ErrTooManyFiles`,
  `ErrZipSlip` (all →400); `ErrPathTraversal` (→400/404)
- `handlers.soalStoreErrorToHTTP` maps the soalpkg set; `mapScheduleError` maps the service set.

### Commit conventions
- Commit verified units directly to `main` (user's established workflow). Conventional
  commits: `feat(scope): ...`, `fix(scope): ...`.
- End commit + PR bodies with `Co-Authored-By: Claude <noreply@anthropic.com>`.
- **Do not stage** `design.md`, `requirements.md`, or `opencode.json` — those carry the
  user's own uncommitted spec edits. Stage code + `tasks.md` only.

---

## 3. Key decisions & non-obvious gotchas

1. **Legacy fallback during transition (Req 6.6, AD-1).** The student flow
   (`StudentLogin`, `StartExamSession`, `GetRemainingTime`, `UpdateStudentProgress`) is
   session-based when `session_id`/session-token is provided, and falls back to the old
   `settings.token` / mapel-based path otherwise. **Do not remove the legacy path** until
   Task 12 (legacy data migration) creates exam_sessions from old `settings`, AND Task 10
   converts the webhook `validasi` key. Also: the old `idx_cek_login_unique_exam_session`
   (mapel-based) index is intentionally **not dropped** yet (migration 025 note) — drop it
   in a later migration once the webhook + StartExamSession are fully session-based.
2. **modernc scans naive SQLite datetimes as UTC.** Tests that build session windows around
   `time.Now()` MUST format in UTC (see `fmtTime` in `handlers/student_flow_test.go`),
   otherwise the "effective window" comparisons drift by the local TZ offset. Production is
   fine because `readSessionInput` requires RFC3339 (TZ-bearing) timestamps.
3. **`scheduling_service` has an injectable clock** (`WithClock(fn)`); default `time.Now`.
   Use it in tests for deterministic time logic.
4. **Migration runner is per-statement self-healing (AD-8).** `RunMigrations(db, dir)`
   splits each file on `;` (string-literal + comment aware) and swallows idempotency errors
   per-statement. New migrations must still be idempotent (`IF NOT EXISTS`).
5. **BodyLimit** is app-wide = `cfg.SoalUploadMaxBytes` (default 100 MB). A TODO in
   `soal_package_handler.go` notes tightening to a per-route `bodylimit` middleware so only
   the upload endpoint accepts the full size (do this if you revisit 6.2).
6. **iSpring export (operational — document in Task 16.2):** the teacher MUST enable
   "Send quiz result to server" in QuizMaker → Reporting at export (otherwise the player
   emits no POST and the shim has nothing to intercept). The server **address field can be
   any placeholder** — `assets/ispring-shim.js` overrides the destination at runtime to
   `/api/ispring/webhook` and appends `attempt_token`/`tenant_id`/`sid`. Result POST fields
   from iSpring: `dr` (XML), `sp`, `tp` (scores), `ps`/`psp`, `sid`, `qt`, … (the shim
   detects `dr`/`sp`/`tp`). Same package works on LAN and online.
7. **`ServeIndexWithShim` reads index.html into memory** for injection (correct, disk
   unchanged — Property 12). AD-4 asks for streaming injection at scale; defer the
   streaming optimization to Task 15 if needed.
8. **CekLogin.locked is declared `INTEGER`** (not BOOLEAN) — scanned as int→bool in
   `scanCekLogin`. `shuffle_*` are BOOLEAN and scan directly into bool.

---

## 4. What's next — start with Task 8 (critical path)

The critical path to real content delivery is `1→2→4→8→14→15`; tasks 1,2,4 are done, so
**Task 8 is the next unblocked critical-path item.**

### Task 8 — Content serving (`GET /api/exam/content/*`)
Builds on: Task 7's content cookie (`aether_exam`, set in `StartExamSession`) + Task 4's
`soalpkg` (`ServeContent`, `ServeIndexWithShim`, `ResolvePath`).
- **8.1 `content_session_service.go`** (`internal/service`): validate the content cookie →
  `cek_login_repo.GetByContentToken` → check tenant match, window (`NotEnterableReason`),
  and `locked`. Issue/validate is already half-done: `StartExamSession` sets the
  `content_token` via `cek_login_repo.SetContentToken` and writes the cookie.
- **8.2 Handler `GET /api/exam/content/*`**: registered **outside** the Bearer
  `AuthMiddleware` group (the iSpring player loads sub-assets via plain HTML tags with no
  `Authorization` header — AD-2). Validate cookie → resolve the exam's `soal_package`
  directory (`data/soal/{tenant_slug}/{package_uuid}/`) → stream via `soalpkg.ServeContent`
  for assets and `soalpkg.ServeIndexWithShim` for `index.html`. Set Content-Type by
  extension. Tenant slug via `handlers.tenantSlug(tenantID)`.
- **8.3 Integration tests**: owner vs non-owner (403), outside window (403), locked (403),
  traversal (400/404). Reuse `newAdminTestApp` with role="student".

### Then (wave 5): 9 → 10 → 12
- **Task 9 (anti-cheat):** `cek_login_repo` already has `Lock/Unlock/IsLocked/
  IncrementInfraction`. Wire `RecordInfraction` (anticheat_handler.go, still mapel-based)
  to increment + lock at `cfg.AntiCheatLockThreshold`; enforce `locked` in start (done) +
  content serve (8.2) + progress. Property 11.
- **Task 10 (webhook):** change `validasi` key to `tenant_id_noID_sessionID` in the webhook
  handler + processor; keep UPSERT on `hasil_tes(tenant_id, validasi)`. Verify `cek_login`
  cleanup targets the right session.
- **Task 12 (legacy data migration):** Go util run after `RunMigrations`; for each tenant
  with `settings.token` but no `exam_session`, create one exam + session from settings
  (idempotent "only if absent"). This makes the legacy fallback unnecessary.

### Later waves
- **11** supervisor monitoring (session-based `GetRoomStatus`/SSE + reset).
- **13** admin SvelteKit UI (uses `apiUrl`/`authHeaders`, no hardcoded URLs/tokens).
- **14** student SvelteKit UI — replace `generateQuestions()`/fake XML with an iframe to
  `/api/exam/content/...`. Keep debounced progress + infraction.
- **15** load test (~500 participants) in `tests/load/`; shim runtime verification needs a
  complete iSpring fixture (current `contoh_soal/KIMIA_XII_UAS_2025` lacks `data/player.js`).
- **16** docs (`Database_Schema.md`, `Technical_Architecture.md`, README, deployment incl.
  the iSpring export instruction from §3.6) + final gate.

### Dependency graph (from tasks.md)
```
wave 5: 8 (needs 4+7+5), 9 (needs 7), 10 (needs 7), 12 (needs 1+2+3)
wave 6: 11 (needs 10), 13 (needs 6), 14 (needs 8)
wave 7: 15 (needs 4+7+8+14)
wave 8: 16 (needs all)
```

---

## 5. Quick file map

```
internal/
  db/migrate.go            RunMigrations(db, dir) — per-statement, self-healing
  db/sqlite.go             Connect, DefaultPoolConfig (canonical pool source)
  config/config.go         Load() reads pool defaults from db.DefaultPoolConfig()
  models/                  SoalPackage, Exam, ExamSession(+relations), CekLogin, status consts
  repository/              grade/soal_package/exam/exam_session/cek_login repos + errors + scan helpers
  service/scheduling_service.go  effective status, window, token-overlap, eligibility, remaining; WithClock
  soalpkg/                 storage.go (Store/RemovePackage), serve.go (ResolvePath/ServeContent/ServeIndexWithShim),
                           shim.go (InjectionHTML), assets/ispring-shim.js (embedded)
  api/handlers/            grade/soal_package/exam/exam_session_handler.go (admin),
                           exam.go (StudentLogin), student_exam.go (Start/Remaining),
                           anticheat_handler.go (RecordInfraction/UpdateStudentProgress),
                           student_session_handler.go (MySessions, content cookie)
  testutil/                NewMigratedDB + Seed* helpers
.kiro/specs/exam-scheduling-and-ispring-delivery/  requirements.md, design.md, tasks.md, HANDOFF.md (this file)
```

## 6. Where to look for examples
- A thin handler delegating to repo + service + mapping errors: `exam_session_handler.go`.
- The dual-path (session vs legacy) pattern: `student_exam.go` `StartExamSession`.
- A property test: `service/scheduling_property_test.go`.
- A security-critical package with tests: `internal/soalpkg/*_test.go`.

Good luck — the foundation is solid and well-tested; pick up at Task 8.
