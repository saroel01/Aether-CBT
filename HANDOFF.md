# Aether CBT - Handoff

**Status:** Hardened MVP, pending deployment evidence  
**Workspace:** `D:\Users\Saroel-H\Projects\Aether-CBT`  
**Stack:** Go/Fiber, SQLite WAL, SvelteKit, Tailwind  
**Current date of handoff:** 2026-05-25  
**Primary dev command:** `npm run dev`

## Executive Summary

Aether CBT has moved beyond a raw MVP. The main issues found during the iSpring/CBT review have been hardened: iSpring parsing is now parser-backed and tested, protected routes are role-scoped, student login issues JWTs, exam result submission requires an active per-attempt token, newly created/imported student passwords are bcrypt-hashed, and the frontend no longer keeps critical API/tenant assumptions scattered across pages.

Do not claim final production readiness yet. Before a real exam day, the project still needs real iSpring QuizMaker XML fixtures from the school, backup/restore rehearsal, realistic load testing, full login rate limiting, and proper credential rotation procedures (lihat docs/credential-rotation.md).

**Recent progress (after initial handoff):** Additional security hardening has been implemented, including JWT algorithm protection, removal of hardcoded secrets, webhook rate limiting + body limits, production CORS allow-list, and stricter tenant isolation in middleware.

## Verification Evidence

Fresh verification already run after the latest changes:

```bash
go test ./...
```

Result: passed for all Go packages.

```bash
npm run build
```

Result: passed. Previous SvelteKit/Svelte runtime export warnings were removed by upgrading to Svelte 5 and `@sveltejs/vite-plugin-svelte` 4.

```bash
npm audit --audit-level=moderate
```

Result: still fails. Advisories remain in the frontend toolchain:

- `cookie <0.7.0` through SvelteKit dependency chain.
- `esbuild <=0.24.2` through Vite/dev-server dependency chain.

The suggested `npm audit fix --force` is not safe as an automatic step because it proposes breaking toolchain jumps. Do not expose the Vite dev server publicly. Revisit this with a deliberate SvelteKit/Vite upgrade plan.

## Key Completed Changes

### iSpring Result Handling

Files:

- `internal/ispring/parser.go`
- `internal/ispring/parser_test.go`
- `internal/api/handlers/ispring.go`
- `internal/api/handlers/ispring_test.go`
- `docs/ISPRING_RESULT_INTEGRATION.md`
- `docs/ISPRING_COMPATIBILITY_TASKS.md`

What changed:

- `POST /api/ispring/webhook` now uses `internal/ispring` parser.
- Parser supports iSpring `quizReport` XML shape, including namespace-aware parsing.
- Supported question families include multiple choice, multiple response, true/false, matching, sequence, type-in, fill-in-the-blank, essay, word bank, numeric, and drag-and-drop.
- Invalid XML returns HTTP `400`.
- Results are upserted using stable unique constraints.
- Detail answers are normalized into `hasil_tes_detail`.

### Database Guarantees

Files:

- `internal/db/migrations/017_create_exam_upsert_indexes.sql`
- `internal/db/migrations/018_alter_cek_login_attempt_token.sql`
- `internal/db/migrate_test.go`
- `docs/Database_Schema.md`

What changed:

- Added unique index for `cek_login(tenant_id, peserta_id, mapel_id)`.
- Added unique index for `hasil_tes(tenant_id, validasi)`.
- Added `cek_login.attempt_token`.
- Added migration tests to keep these assumptions covered.

### Student Authentication and Exam Session Security

Files:

- `internal/api/handlers/exam.go`
- `internal/api/handlers/student_exam.go`
- `internal/api/handlers/student_auth_flow_test.go`
- `internal/utils/auth.go`

What changed:

- Student login now returns a JWT and user object.
- Student protected exam routes can use the JWT.
- Student role can only start its own exam session.
- `POST /api/student/start` generates a secure random `attempt_token`.
- Web simulator/iSpring submission must include matching `attempt_token` or `AETHER_ATTEMPT_TOKEN`.
- New helper `GenerateSecureToken`.
- New helper `CheckPasswordOrPlaintext` allows legacy plaintext rows while supporting bcrypt rows.

### Role Middleware

Files:

- `internal/api/middleware/role.go`
- `internal/api/middleware/role_test.go`
- `cmd/server/main.go`

What changed:

- Added `RequireRoles(...)`.
- Admin, supervisor, superadmin, and student routes are separated by role.
- Supervisor/admin can access room monitoring and result exports.
- Student-only routes include exam start/progress/infraction.
- Admin/superadmin routes protect management operations.

### Student Password Hardening

Files:

- `internal/api/handlers/student.go`
- `internal/api/handlers/csv_utility.go`
- `internal/api/handlers/student_auth_flow_test.go`
- `internal/utils/auth.go`
- `USAGE_GUIDE.md`

What changed:

- Manually created students are stored with bcrypt.
- CSV-imported students are stored with bcrypt.
- Existing plaintext student rows still work so old seed/import data does not break immediately.
- Tests cover bcrypt login, create, and CSV import.

### Frontend Runtime Configuration and iSpring Attempt Token Flow

Files:

- `web/src/lib/api.ts`
- `web/src/routes/student/login/+page.svelte`
- `web/src/routes/student/select-subject/+page.svelte`
- `web/src/routes/student/exam/+page.svelte`
- `web/src/routes/admin/+page.svelte`
- `web/src/routes/admin/+layout.svelte`
- `web/src/routes/admin/settings/+page.svelte`
- `web/src/routes/admin/students/+page.svelte`
- `web/src/routes/admin/students/print-cards/+page.svelte`
- `web/src/routes/supervisor/+page.svelte`

What changed:

- Centralized helpers: `apiUrl`, `authHeaders`, `qrCodeUrl`.
- Frontend honors `VITE_API_BASE` and `VITE_TENANT_ID`.
- Production default is same-origin `/api`.
- Vite dev fallback still points to `http://localhost:3000/api`.
- Student login stores JWT and user in local storage.
- Subject selection stores `attempt_token`.
- Exam submit sends `attempt_token` to `/api/ispring/webhook`.
- Admin/supervisor QR rendering no longer hardcodes `http://localhost:3000`.
- Admin and print-card pages fetch real admin settings token instead of using the seeded token.

### Supervisor Settings

Files:

- `internal/api/handlers/settings_handler.go`
- `internal/api/handlers/supervisor_settings_test.go`
- `cmd/server/main.go`
- `web/src/routes/supervisor/+page.svelte`

What changed:

- Added `GET /api/supervisor/settings`.
- Allows supervisor/admin/superadmin to read active exam token/title for room display.
- Rejects student role.
- Covered by tests.

### Frontend Tooling

Files:

- `web/package.json`
- `web/package-lock.json`
- `web/src/lib/components/ui/Modal.svelte`
- `web/src/lib/components/ui/Toast.svelte`
- `web/src/routes/admin/settings/+page.svelte`

What changed:

- Upgraded `svelte` to 5.
- Upgraded `@sveltejs/vite-plugin-svelte` to 4.
- Added aria labels to icon-only buttons flagged by Svelte 5 a11y checks.
- `npm run build` now completes without the earlier Svelte runtime export warnings.

### Documentation

Files:

- `README.md`
- `PROJECT_STATUS.md`
- `DEVELOPMENT_COMPLETE.md`
- `HANDOFF.md`
- `USAGE_GUIDE.md`
- `docs/Technical_Architecture.md`
- `docs/Database_Schema.md`
- `docs/ISPRING_RESULT_INTEGRATION.md`
- `docs/ISPRING_COMPATIBILITY_TASKS.md`
- `docs/superpowers/plans/2026-05-25-production-hardening.md`

What changed:

- Removed confusing production-ready claims.
- Current status is consistently described as hardened MVP/pilot-ready preparation, not final production.
- Documented `attempt_token` requirement.
- Documented frontend env configuration.
- Documented remaining deployment gaps.

## Important Current Behavior

Student flow:

1. Student logs in at `/student/login` with `no_id`, password, and exam token.
2. Backend validates exam token from `settings`.
3. Backend validates password using bcrypt if hash-like, otherwise plaintext legacy comparison.
4. Backend returns JWT with role `student`.
5. Student selects subject.
6. `POST /api/student/start` creates/updates `cek_login` and returns `attempt_token`.
7. Student exam page stores and submits `attempt_token` with result payload.
8. `POST /api/ispring/webhook` rejects missing/wrong attempt token with HTTP `403`.
9. Successful result upserts `hasil_tes`, writes details, and deletes active `cek_login`.

iSpring webhook minimum expected fields:

- `sid` or `USER_NAME`
- `sp`
- `tp`
- `dr`
- `attempt_token` or `AETHER_ATTEMPT_TOKEN`

Frontend environment:

```bash
VITE_API_BASE=http://localhost:3000/api
VITE_TENANT_ID=1
```

If `VITE_API_BASE` is unset, production build uses `/api`. Vite dev on port `5173` falls back to `http://localhost:3000/api`.

## Known Deployment Gaps

These are intentionally not hidden:

1. Real iSpring QuizMaker XML fixtures from the school are still needed.
2. Backup and restore rehearsal for `data/cbt_aether.db` is not yet automated/proven.
3. Realistic load test is not yet run.
4. ~~Production CORS should be allow-listed instead of wildcard.~~ (Completed — now uses `CORS_ALLOWED_ORIGINS` with strict production enforcement)
5. Default JWT secret/admin/supervisor/student credentials must be rotated before deployment.
6. `npm audit` still reports frontend toolchain advisories that require a deliberate breaking upgrade plan.
7. Legacy plaintext student rows remain accepted for migration compatibility; eventually add a migration/rotation workflow to replace them.

## Recommended Next Tasks

1. Add real iSpring fixture tests:
   - Export sample quizzes from the actual school iSpring QuizMaker projects.
   - Save XML fixtures under a new test fixture directory.
   - Assert parser output per question type used by the school.

2. Add backup/restore procedure:
   - Backup `data/cbt_aether.db`, WAL, and SHM files safely.
   - Restore into a clean data directory.
   - Run app smoke test after restore.
   - Document exact commands in `USAGE_GUIDE.md`.

3. Add load test:
   - Simulate login, subject start, progress updates, and result submission.
   - Use realistic payload sizes.
   - Verify SQLite WAL behavior under concurrent submits.

4. Harden deployment config (partially completed):
   - [x] Set `JWT_SECRET` (now strictly enforced at startup).
   - [x] Restrict CORS (now uses allow-list via `CORS_ALLOWED_ORIGINS`).
   - Rotate default accounts.
   - Ensure Vite dev server is never public.

5. Plan frontend dependency audit resolution:
   - Avoid `npm audit fix --force` without a branch.
   - Test a controlled Vite/SvelteKit upgrade in isolation.

## Useful Commands

```bash
npm run dev
npm run seed
go test ./...
cd web
npm run build
npm audit --audit-level=moderate
```

## Git State at Handoff

**Note:** This document was originally written at the initial hardening phase. Subsequent security improvements (JWT protection, secret enforcement, webhook hardening, CORS allow-list, and tenant validation) have since been implemented, committed, and pushed (see commit history after 2026-05-25).

High-signal new files include:

- `internal/ispring/parser.go`
- `internal/ispring/parser_test.go`
- `internal/api/middleware/role.go`
- `internal/api/middleware/role_test.go`
- `internal/api/handlers/student_auth_flow_test.go`
- `internal/api/handlers/supervisor_settings_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/017_create_exam_upsert_indexes.sql`
- `internal/db/migrations/018_alter_cek_login_attempt_token.sql`
- `docs/ISPRING_RESULT_INTEGRATION.md`
- `docs/ISPRING_COMPATIBILITY_TASKS.md`
- `docs/superpowers/plans/2026-05-25-production-hardening.md`

## Handoff Warning

If the next worker has limited context, start by reading:

1. `docs/superpowers/plans/2026-05-25-production-hardening.md`
2. `docs/ISPRING_RESULT_INTEGRATION.md`
3. `HANDOFF.md`
4. `cmd/server/main.go`
5. `internal/api/handlers/exam.go`
6. `internal/api/handlers/student_exam.go`
7. `internal/api/handlers/ispring.go`
8. `web/src/lib/api.ts`

Then run:

```bash
go test ./...
cd web
npm run build
```
