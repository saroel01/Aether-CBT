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
| `api/handlers`  | HTTP request handling & response            | student.go, admin.go, etc. |
| `api/middleware`| Cross-cutting concerns (auth, rate limit)   | auth.go, cors.go |
| `ispring/`      | Parse iSpring `quizReport` detail XML from webhook payloads | parser.go |
| `services/`     | Business logic                              | exam/, result/, ispring/ |
| `repository/`   | Data access layer (SQLite queries)          | *_repo.go |
| `models/`       | Struct definitions & validation             | *.go |
| `db/`           | Database connection & migrations            | sqlite.go |
| `utils/`        | Shared utilities (Excel, token, logging)    | excel/, token/ |

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

---

## 8. OBSERVABILITY

- Structured logging (JSON format)
- Request ID propagation
- Basic metrics endpoint (`/metrics`) for Prometheus (optional)
- Error tracking with stack traces

---

**This architecture prioritizes simplicity, performance, and reliability while remaining maintainable for a single developer or small team.**

*Next: Detailed Database Schema*
