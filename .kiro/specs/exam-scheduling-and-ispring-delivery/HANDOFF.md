# Handoff — Exam Scheduling & iSpring Delivery

**Status: 11 / 16 tasks complete, all on `main`, `go build/vet/test ./...` green.**
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
2. **Read the spec trio** (`requirements.md`, `design.md`, `tasks.md`) — tasks 1–11 are
   `[x]`, tasks 12–16 are `[ ]`.
3. **Confirm green** before touching anything:
   ```bash
   go build ./... && go vet ./... && go test ./...
   ```
4. **Resume at Task 12** (legacy data migration) — see §4. Backend tasks 1–11 are done;
   remaining is `12 → 13 → 14 → 15 → 16`.

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
| `3fc101b` | **8** content serving + shim | `content_session_service.Authorize`, `ServeExamContent` (`GET /api/exam/content/*`), TenantMiddleware content-path exemption, migration 026 (unique `content_token`); 6 service + 12 handler + 2 middleware + 1 migration test |
| `4fc786f` | **9** anti-cheat | session-based `RecordInfraction` + lock at `cfg.AntiCheatLockThreshold`; `UpdateStudentProgress` rejects locked (Property 11); 4 handler tests |
| `b73b2eb` | **10** webhook `validasi` | `validasi` = `tenant_noID_sessionID` (legacy mapel fallback); processor scopes cek_login lookup+delete by `attempt_token` (right-session cleanup); property + cleanup tests |
| `dc351c5` | **11** supervisor monitoring | `GetRoomStatus`/SSE per-student `Status` (not_logged_in/in_progress/locked/submitted) + `session_id` scope; `ResetStudentSession` session-targeted; 4 handler tests |

### Task 8 notes (read before Task 9/14)
- **Content serving derives the tenant from the cookie token, NOT the request.** The iSpring
  player loads sub-assets via plain HTML tags with no `Authorization` **and** no tenant
  header, so `GET /api/exam/content/*` is registered **outside** the Bearer `AuthMiddleware`
  group (next to the webhook, `main.go`) and `TenantMiddleware` early-returns for that exact
  path (`p == "/api/exam/content" || HasPrefix "/api/exam/content/"` — precise, not a loose
  `HasPrefix`, so a future `/api/exam/content*` route is not silently exempted). The token
  is an unguessable per-session capability; migration **026** makes it a unique partial index
  so the token→`cek_login` mapping is 1:1 at the data layer.
- `ContentSessionService.Authorize(token)` chain: `GetByContentToken` → lock check →
  `exam_session.GetByID` → `enterable` window → `exam.GetByID` → `soal_package.GetByID` →
  `peserta.no_id` (for the shim `SID`). All scoped to the token's tenant. Errors:
  `ErrContentUnauthorized`(→401), `ErrContentLocked`/`ErrContentWindowClosed`(→403),
  `ErrContentPackageMissing`(→404), mapped in `mapContentError`; serve errors in
  `mapServeError` (`ErrPathTraversal`→400, not-exist→404).
- **Shim is injected only on the entry** (`ServeIndexWithShim`); assets stream verbatim via
  `ServeContent`. `contentTypeFor` hard-codes iSpring MIME types (host-registry-independent
  for Windows dev) with `mime.TypeByExtension` fallback.
- **Deferred to Task 15** (per design AD-4 / HANDOFF §3.7): streaming the entry/asset instead
  of buffering through fasthttp's `BodyWriter`, and caching the 5-query auth chain per asset.
  Functionally correct now; revisit under the ~500-participant load test.

### Tasks 9–11 notes (read before Task 12+)
- **Anti-cheat (9):** `RecordInfraction` is session-based — `cek_login_repo.IncrementInfraction`
  then `Lock` when `count >= antiCheatLockThreshold` (package var, wired via
  `SetAntiCheatLockThreshold(cfg.AntiCheatLockThreshold)` in `main.go`, default 3). Returns
  `{infraction_count, locked}`. A student may only record their own infractions (owner check,
  mirrors `StartExamSession`). `UpdateStudentProgress` rejects a locked session (403) —
  Property 11 now enforced on start + content serve + progress.
- **Webhook `validasi` (10):** `ISpringWebhook` builds `validasi = tenant_noID_sessionID`
  (falls back to `tenant_noID_mapelID` when `cek_login.session_id` is NULL, for legacy
  results). The active session is matched by `(no_id, attempt_token)` in the JOIN (not a
  separate constant-time compare) so a peserta with multiple sessions resolves the right one.
  The processor scopes its cek_login lookup AND post-result DELETE by `attempt_token`, so a
  sibling session survives (Req 11.3). The `hasil_tes(tenant_id, validasi)` unique index is
  unchanged. Test schemas in `ispring_test.go`/`features_test.go` now include `session_id`.
- **Supervisor (11):** `fetchRoomStatus(tenantID, ruangID, sessionID)` (in `supervisor.go`) is
  shared by REST + SSE; each `LiveStudentStatus` now carries `Status`, `Locked`, `SessionID`.
  **GOTCHA:** the SSE stream-writer closure runs after fasthttp recycles the `Ctx` — capture
  every request value (`tenantID`, `ruangID`, `sessionID`) as a local BEFORE
  `SetBodyStreamWriter`; never touch `c` inside the closure. `ResetStudentSession` targets one
  session when `session_id` is given (row removal clears the lock), else legacy all-sessions
  reset.

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

## 4. What's next — start with Task 12 (legacy data migration)

Tasks 8–11 are **DONE** (content serving, anti-cheat, webhook `validasi`, supervisor
monitoring) — see the notes in §1. The backend is feature-complete for the session model;
what remains is **data migration (12)**, the two **SvelteKit frontends (13, 14)**, the
**load test (15)**, and **docs (16)**.

### Task 12 — Legacy data migration (resume here)
- A Go util run after `RunMigrations` (in `cmd/server/main.go`, after the migrations call):
  for each tenant that has `settings.token`/`is_exam_active` but **no** `exam_session`,
  create one `exam` + one `exam_session` "legacy" from those settings, idempotent
  ("only if absent"). Pick a placeholder mapel deterministically (or create one) — if that
  can't be done unambiguously, document the limitation. This makes the legacy fallback in
  `StudentLogin`/`StartExamSession`/`GetRemainingTime`/`UpdateStudentProgress` unnecessary;
  once verified, the legacy paths + the old mapel-based `idx_cek_login_unique_exam_session`
  index (migration 025 note) can be dropped.
- Test: rerun doesn't duplicate; an old install still logs in during transition (Req 14.3, 14.4).

### Task 13 — Admin SvelteKit UI (`web/`)
- Use `apiUrl`/`authHeaders` from the existing client; **no hardcoded URLs/tokens**. Pages:
  class tingkat (13.1), soal-package upload/list/delete (13.2), exam create/edit + link
  package (13.3), exam-session window/token/classes/rooms/effective status (13.4).
- Skill: `frontend-design:frontend-design` for the UI; gate is `npm run build` (Req 16.5).

### Task 14 — Student SvelteKit UI (`web/`)
- Replace `generateQuestions()`/fake XML in `web/src/routes/student/exam/+page.svelte` with an
  `<iframe src="/api/exam/content/index.html">` (same-origin; the content cookie is sent).
- Keep debounced progress (`POST /api/student/progress`) + infraction
  (`POST /api/student/infraction`); react to `locked:true` / 403 by showing the locked state.
- Skill: `frontend-design:frontend-design`. Gate: `npm run build`, no hardcode URL/token.

### Task 15 — Scale + shim verification
- `tests/load/` → ~500 participants (login/start/progress/submit); verify no lost results
  (Property 10) and acceptable content-serve latency. **Revisit the deferred Task 8 items
  here**: stream entry/assets (AD-4) instead of buffering via `BodyWriter`, and cache the
  5-query content auth chain.
- Shim runtime verification (15.2) needs a **complete** iSpring fixture with `data/player.js`
  (current `contoh_soal/KIMIA_XII_UAS_2025` lacks it). Drive it headless with the
  `chrome-devtools-mcp:chrome-devtools` skill, or document as manual verification pre-launch.
- Use `verify`/`run` skills to launch the app and confirm the iframe + shim end-to-end.

### Task 16 — Docs + final gate
- Update `docs/Database_Schema.md`, `docs/Technical_Architecture.md`, README, deployment guide
  (incl. the iSpring "Send quiz result to server" export instruction from §3.6, and kiosk mode).
- Final gate: `go build/vet/test ./...` + `npm run build` (Req 16.5).

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
  service/content_session_service.go  Authorize(contentToken): token→session→exam→package, lock/window; WithContentClock
  soalpkg/                 storage.go (Store/RemovePackage), serve.go (ResolvePath/ServeContent/ServeIndexWithShim),
                           shim.go (InjectionHTML), assets/ispring-shim.js (embedded)
  api/handlers/            grade/soal_package/exam/exam_session_handler.go (admin),
                           exam.go (StudentLogin), student_exam.go (Start/Remaining),
                           anticheat_handler.go (RecordInfraction/UpdateStudentProgress),
                           student_session_handler.go (MySessions, content cookie),
                           content_serving_handler.go (ServeExamContent + contentTypeFor/mapContentError/mapServeError)
  testutil/                NewMigratedDB + Seed* helpers
.kiro/specs/exam-scheduling-and-ispring-delivery/  requirements.md, design.md, tasks.md, HANDOFF.md (this file)
```

## 6. Where to look for examples
- A thin handler delegating to repo + service + mapping errors: `exam_session_handler.go`.
- A cookie-authorized handler with error→HTTP mapping + shim injection: `content_serving_handler.go`.
- The dual-path (session vs legacy) pattern: `student_exam.go` `StartExamSession`.
- A property test: `service/scheduling_property_test.go`.
- A security-critical package with tests: `internal/soalpkg/*_test.go`.

Good luck — the foundation is solid and well-tested; pick up at Task 12.
