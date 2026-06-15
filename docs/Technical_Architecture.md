# TECHNICAL ARCHITECTURE DOCUMENT
## Aether CBT — Modern Computer-Based Testing Platform

**Version**: 1.0  
**Date**: 23 May 2026  
**Tech Stack**: Go (Fiber) + SQLite + SvelteKit + Tailwind + PWA

---

## 1. ARCHITECTURE OVERVIEW

### 1.1 High-Level Architecture (Multi-Tenant)

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                         │
│  Tenant A (School 1)     Tenant B (School 2)                │
│  /tenant/sekolah1       /tenant/sekolah2                    │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────────┐
│              Tenant Middleware (Isolation Layer)           │
│         - Resolve tenant from path/subdomain               │
│         - Inject tenant_id into all requests               │
│         - Enforce strict data isolation                    │
└───────────────────────────┬────────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────────┐
│                      API Gateway                           │
│                   Go Fiber Server                          │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │   Auth      │ │   Student   │ │  Supervisor │          │
│  │  Middleware │ │   Routes    │ │   Routes    │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │   Admin     │ │   iSpring   │ │   Export    │          │
│  │   Routes    │ │  Webhook    │ │   Routes    │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
└───────────────────────────┬────────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────────┐
│                    Data Layer                              │
│                    SQLite (WAL)                            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │         Single Database with Tenant Isolation       │   │
│  │  - tenants table                                    │   │
│  │  - All other tables contain tenant_id               │   │
│  │  - Strict row-level isolation enforced in queries   │   │
│  └─────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────┘
```

### 1.2 Design Principles

- **Simplicity First**: Prefer simple, proven solutions over complex abstractions
- **Single Responsibility**: Each module has one clear purpose
- **Offline Native**: Architecture designed from the ground up for offline operation
- **Zero External Dependencies** at runtime (except iSpring HTML5 assets)
- **Observable**: Every critical path has logging and metrics

---

## 2. PROJECT STRUCTURE (Recommended)

```
aether-cbt/
├── cmd/
│   └── server/
│       └── main.go                 # Application entrypoint
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── auth.go
│   │   │   ├── student.go
│   │   │   ├── supervisor.go
│   │   │   ├── admin.go
│   │   │   └── ispring.go
│   │   ├── middleware/
│   │   │   ├── auth.go
│   │   │   ├── cors.go
│   │   │   └── rate_limit.go
│   │   └── routes.go
│   ├── config/
│   │   └── config.go
│   ├── db/
│   │   ├── sqlite.go
│   │   └── migrations/
│   │       └── *.sql
│   ├── models/
│   │   ├── user.go
│   │   ├── peserta.go
│   │   ├── hasil.go
│   │   └── ...
│   ├── services/
│   │   ├── auth/
│   │   ├── exam/
│   │   ├── result/
│   │   └── ispring/
│   ├── utils/
│   │   ├── excel/
│   │   ├── token/
│   │   └── logger/
│   └── repository/
│       ├── user_repo.go
│       ├── peserta_repo.go
│       └── hasil_repo.go
├── web/                            # SvelteKit frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/
│   │   │   ├── stores/
│   │   │   ├── api/
│   │   │   └── utils/
│   │   ├── routes/
│   │   │   ├── (admin)/
│   │   │   ├── (supervisor)/
│   │   │   └── (student)/
│   │   └── app.html
│   ├── static/
│   │   └── (icons, manifest, etc.)
│   └── package.json
├── data/                           # Runtime data directory
│   ├── soal/                       # iSpring HTML5 folders
│   └── uploads/
├── docs/                           # All documentation
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 3. MODULE DESIGN

### 3.1 Backend Modules (Go)

| Module          | Responsibility                              | Key Files |
|-----------------|---------------------------------------------|---------|
| `api/handlers`  | HTTP request handling & response (thin)     | exam.go, student_exam.go, exam_session_handler.go, content_serving_handler.go, etc. |
| `api/middleware`| Cross-cutting concerns (auth, tenant, rate limit) | auth.go, tenant.go, cors.go |
| `ispring/`      | Parse iSpring `quizReport` detail XML from webhook payloads | parser.go |
| `service/`      | Cross-entity scheduling rules, content auth, legacy migration | scheduling_service.go, content_session_service.go, legacy_migration.go |
| `soalpkg/`      | iSpring package storage/serve/shim injection (Req 3, 8, 9) | storage.go, serve.go, shim.go, assets/ispring-shim.js |
| `repository/`   | Tenant-scoped data access layer             | exam_repo.go, exam_session_repo.go, soal_package_repo.go, grade_repo.go, cek_login_repo.go |
| `submission/`   | Filesystem submission queue + worker (no-loss) | fsqueue.go, worker.go, processor.go |
| `models/`       | Struct definitions & status consts          | exam.go, exam_session.go, soal_package.go, cek_login.go |
| `db/`           | Database connection, per-statement migrations, canonical pool config | sqlite.go, migrate.go, migrations/ |
| `utils/`        | Shared utilities (Excel, secure-token, JWT, logging) | excel/, token.go, jwt.go |

### 3.2 Frontend Modules (SvelteKit)

| Module               | Responsibility                          |
|----------------------|-----------------------------------------|
| `routes/(admin)`     | Admin panel pages                       |
| `routes/(supervisor)`| Supervisor room dashboard               |
| `routes/(student)`   | Student exam flow                       |
| `lib/components`     | Reusable UI components                  |
| `lib/stores`         | Svelte stores (auth, exam state, etc.)  |
| `lib/api`            | Typed API client functions              |

---

## 4. API SPECIFICATION (High-Level)

### 4.1 Authentication

| Method | Endpoint                    | Description                  | Auth |
|--------|-----------------------------|------------------------------|------|
| POST   | `/api/auth/login`           | Login (admin/supervisor)     | Public |
| POST   | `/api/auth/student-login`   | Student login                | Public |
| POST   | `/api/auth/logout`          | Logout                       | Protected |

### 4.2 Student Flow

| Method | Endpoint                          | Description                     |
|--------|-----------------------------------|---------------------------------|
| GET    | `/api/student/subjects`           | Get available subjects          |
| POST   | `/api/student/start-exam`         | Start selected subject          |
| POST   | `/api/ispring/webhook`            | Receive iSpring result (public) |

`/api/ispring/webhook` accepts iSpring POST fields such as `sid`, `USER_NAME`, `sp`, `tp`, `dr`, and `attempt_token`. The `dr` field must use iSpring `quizReport` XML; parser behavior and supported question types are documented in `docs/ISPRING_RESULT_INTEGRATION.md`.

### 4.2a Exam Scheduling & iSpring Content Delivery (session model)

The scheduling spec replaces the single global `settings.token`/`is_exam_active` model with
**per-session tokens and time windows**. Entities: `kelas.tingkat` (grade level),
`soal_package` (uploaded iSpring export), `exam` (reusable definition), `exam_session`
(a scheduled wave), and `exam_session_kelas`/`exam_session_ruang` (eligible classes/rooms).
See `docs/Database_Schema.md §2.S` for the schema.

| Method | Endpoint                              | Description                                          | Auth |
|--------|---------------------------------------|------------------------------------------------------|------|
| GET    | `/api/student/my-sessions`            | Sessions the student may enter now (Req 5.3, 6.4)   | JWT student |
| POST   | `/api/student/start`                  | Start session → `attempt_token` + content cookie     | JWT student |
| GET    | `/api/student/remaining-time`         | `min(duration, sessionEnd − now)` clamped (Req 7.5)  | JWT student |
| POST   | `/api/student/progress`               | Debounced progress UPSERT (Req 13.2)                 | JWT student |
| POST   | `/api/student/infraction`             | Increment; server-locks at threshold (Req 10.2)      | JWT student |
| GET    | `/api/exam/content/*`                 | Serve iSpring package + shim (Req 8)                 | **content cookie** (not Bearer) |
| POST   | `/api/ispring/webhook`                | Receive result → enqueue (no-loss queue)             | Public (rate-limited) |
| PUT    | `/api/classes/:id/tingkat`            | Set class grade level (Req 1.1)                      | admin |
| GET/POST/DELETE | `/api/admin/soal-packages[/...]` | Upload/list/delete iSpring packages (Req 3)        | admin |
| GET/POST/PUT/DELETE | `/api/admin/exams[/...]`     | Exam definition CRUD (Req 2)                         | admin |
| GET/POST/PUT/DELETE | `/api/admin/exam-sessions[/...]` | Session CRUD + link classes/rooms (Req 4, 5)    | admin |

**Content authorization (AD-2):** the iSpring player loads sub-assets (`data/player.js`,
fonts, images) via plain HTML tags with no `Authorization` header, so `GET /api/exam/content/*`
is registered **outside** the Bearer `AuthMiddleware` group and authorized instead by the
same-origin `aether_exam` cookie set on `POST /student/start`. The cookie value (`content_token`)
maps 1:1 to a `cek_login` row (unique partial index, migration 026). `TenantMiddleware`
early-returns for the exact `/api/exam/content` path (the tenant comes from the cookie token).

**iSpring shim (AD-3):** the shim (`internal/soalpkg/assets/ispring-shim.js`, embedded via
`go:embed`) is injected into the served `index.html` only (disk untouched — Property 12). It
intercepts the browser network layer (`XMLHttpRequest`, `fetch`, `sendBeacon`, form submit),
detects iSpring result payloads (`dr`/`sp`/`tp`), and re-routes them to the same-origin
`/api/ispring/webhook`, appending `attempt_token`/`tenant_id`/`sid` from the injected context.
This frees teachers from configuring a server URL at export — any placeholder works (see
`tests/load/SHIM_VERIFICATION.md` for the export instruction and manual verification checklist).

**Submission queue (no-loss):** the webhook only `Enqueue`s (atomic rename to `queued/`) and
returns 200; a single worker drains batches in one transaction (write `hasil_tes` + detail,
delete `cek_login` per-session), serializing writes so the SQLite write-lock is not contended
on the hot path. Property 10 (no lost results under ~500-concurrent submission) is verified by
`internal/submission/property10_test.go`.

**Legacy migration (Req 14.3):** at startup, after `RunMigrations`, `service.LegacyMigrator`
creates one placeholder `exam` + `exam_session` (wide-open window, status `aktif`) for each
tenant that still has a `settings` row but no `exam_session`, using the old `settings.token` as
the session token so existing logins resolve to the session path. Idempotent; tenants already on
the session model are skipped. The legacy `mapel`-based `StudentLogin`/`StartExamSession` paths
remain during the transition (Req 6.6).

**Anti-cheat (Req 10):** `RecordInfraction` increments on the server and sets `cek_login.locked`
when the count reaches `ANTICHEAT_LOCK_THRESHOLD` (default 3). A locked session is rejected at
start, content-serve, and progress (Property 11); a supervisor `reset` clears the row/lock.

### 4.3 Supervisor

| Method | Endpoint                          | Description                     |
|--------|-----------------------------------|---------------------------------|
| GET    | `/api/supervisor/room-status`     | Live status of room             |
| POST   | `/api/supervisor/reset-student`   | Reset specific student          |

### 4.4 Admin

| Method | Endpoint                          | Description                          |
|--------|-----------------------------------|--------------------------------------|
| GET    | `/api/admin/peserta`              | List students (with filters)         |
| POST   | `/api/admin/peserta/import`       | Bulk import from Excel               |
| GET    | `/api/admin/results/export`       | Export results (Excel/PDF)           |
| POST   | `/api/admin/settings`             | Update global configuration          |

---

## 5. DATABASE CONNECTION & MIGRATIONS (Multi-Tenant)

- Single SQLite file: `data/cbt_aether.db`
- WAL mode enabled for better concurrency
- All tables (except `tenants`) **must** contain `tenant_id`
- Every query **must** filter by `tenant_id` (enforced via repository layer)
- Migrations stored in `internal/db/migrations/`
- Version table (`schema_migrations`) to track applied migrations
- Tenant creation automatically creates isolated data scope (no separate database)

---

## 6. SECURITY ARCHITECTURE

- JWT tokens protect admin, supervisor, superadmin, and student routes.
- Role middleware enforces route-level access boundaries.
- Student exam starts generate a per-attempt token stored in `cek_login`; result submission must echo this token.
- **Content serving uses a same-origin `aether_exam` cookie (HttpOnly, SameSite=Strict, Secure on HTTPS), not Bearer JWT** — because the iSpring player loads sub-assets with no `Authorization` header (AD-2). The cookie maps 1:1 to a `cek_login` row.
- **Server-side anti-cheat lock:** `cek_login.locked` is authoritative — when set, start/serve/progress are rejected with 403 regardless of client state (Property 11). Only a supervisor reset clears it (Req 10.4).
- Newly created/imported student passwords are stored with bcrypt; legacy plaintext rows are accepted for migration compatibility.
- Production deployment must still add full login rate limiting (webhook already has rate limiting + body limits).
- CORS is now enforced via allow-list (`CORS_ALLOWED_ORIGINS`); wildcard is no longer used.
- All user inputs sanitized
- SQL queries use prepared statements only
- File uploads validated (type, size, content)

---

## 7. DEPLOYMENT MODEL (Multi-Tenant Ready)

**Recommended Production Deployment**:
- Single Linux server (or even Windows)
- One compiled binary (`aether-cbt`)
- One SQLite database file (contains all tenants)
- iSpring quiz folders placed in `data/soal/{tenant_slug}/`
- Reverse proxy (Caddy/Nginx) **recommended** for clean URL routing (`/tenant/{slug}`)
- Default tenant created automatically on first run

**Development**:
- `npm run dev` runs both Go backend and SvelteKit dev server
- Hot reload enabled for frontend
- Default tenant `default` automatically created for single-tenant usage
- Frontend API configuration uses `VITE_API_BASE` and `VITE_TENANT_ID`; production defaults to same-origin `/api`.

**iSpring export (teacher, per quiz — Requirement 9.6, AD-3):**
In iSpring QuizMaker → **Reporting**, enable **"Send quiz result to server"** at export. The
server **address field can be any placeholder** — the injected shim overrides the destination
at runtime to the same-origin `/api/ispring/webhook` and appends `attempt_token`/`tenant_id`/
`sid`. Without this setting the player emits no POST and the shim has nothing to intercept.
Same package works on LAN (IP) and online (domain) with no per-deployment URL change. Run the
manual checklist in `tests/load/SHIM_VERIFICATION.md` before each exam day.

**Kiosk / lockdown mode (deployment guidance, out of code scope):**
For exam integrity at the OS level, run student devices in a locked-down browser/OS profile:
- **Windows:** assign the exam device to a kiosk account (Assigned Access) launching the
  browser fullscreen to the exam URL; or use a managed browser (e.g. Edge kiosk mode
  `--kiosk https://exam.url --edge-kiosk-type=fullscreen`).
- **Chrome/Chromium OS:** use the Kiosk App / Single App mode policy.
- Network: restrict egress to the exam server origin only (firewall/proxy) so students cannot
  reach search engines or chat. The application enforces tab-switch/blur infractions
  server-side regardless (Req 10), but kiosk mode prevents the easier circumvention paths.
These are deployment/operator steps, not code; the app itself is kiosk-agnostic.

---

## 8. OBSERVABILITY

- Structured logging (JSON format)
- Request ID propagation
- Basic metrics endpoint (`/metrics`) for Prometheus (optional)
- Error tracking with stack traces

---

**This architecture prioritizes simplicity, performance, and reliability while remaining maintainable for a single developer or small team.**

*Next: Detailed Database Schema*
