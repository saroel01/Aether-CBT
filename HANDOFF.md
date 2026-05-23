# AETHER CBT - HANDOFF DOCUMENT

**Project**: Aether CBT (Multi-Tenant Computer-Based Testing Platform)  
**Status**: Backend Solid + Functional Frontend MVP (UI masih sangat dasar)  
**Date**: 23 May 2026 (Updated)  
**Tech Stack**: Go (Fiber) + SQLite + SvelteKit + Tailwind

---

## 1. PROJECT OVERVIEW

Aether CBT is a modern, high-performance, multi-tenant Computer-Based Testing platform designed for schools. It supports up to 500+ concurrent students per tenant with true offline capability.

**Current State** (as of latest review):

**Backend**:
- Strong and mature
- Full schema with 11 migrations + auto-apply
- Multi-tenant properly enforced
- JWT + bcrypt + CORS configured
- All core CRUD endpoints working

**Frontend**:
- Functional but very basic / early MVP
- Admin login + basic data management works
- Student login exists but exam screen is placeholder
- **Does not follow** the design system documented in `docs/UI_Component_Library.md`
- No proper component library yet

**Overall**:
- Can run with one command (`npm run dev`)
- Suitable for continued development
- Not ready for real school usage or pilot testing

---

## 2. TECH STACK

| Layer       | Technology                          | Version     |
|-------------|-------------------------------------|-------------|
| Backend     | Go + Fiber                          | 1.22 / 2.52 |
| Database    | SQLite (WAL mode)                   | 3           |
| Auth        | JWT + bcrypt                        | v5 / latest |
| Frontend    | SvelteKit + TypeScript + Tailwind   | Latest      |
| Real-time   | Server-Sent Events (SSE)            | -           |
| QR Code     | go-qrcode                           | v0.0.0      |

---

## 3. PROJECT STRUCTURE

```
aether-cbt/
├── cmd/
│   ├── server/main.go                 # Main application entry
│   ├── createadmin/main.go            # Create default admin
│   └── seed/main.go                   # Seed sample data
├── internal/
│   ├── api/
│   │   ├── handlers/                  # All HTTP handlers (incl. me.go)
│   │   └── middleware/
│   │       ├── auth.go
│   │       └── tenant.go
│   ├── config/config.go
│   ├── db/
│   │   ├── sqlite.go
│   │   ├── migrate.go                 # Auto migration runner
│   │   └── migrations/                # 11 migration files
│   ├── models/
│   ├── repository/
│   └── utils/
├── web/                               # SvelteKit frontend (basic MVP)
├── data/
├── docs/                              # Includes UI_Component_Library.md (not yet implemented)
├── .opencode/skills/                  # 4 development skills
├── package.json                       # Primary scripts (npm run dev)
├── go.mod
├── Makefile
├── QUICKSTART.md
└── HANDOFF.md
```

---

## 4. HOW TO RUN (Recommended)

### Prerequisites
- Go 1.22+
- Node.js 18+

### One Command (Recommended)

```bash
npm run dev
```

This starts **both** backend (port 3000) and frontend (port 5173) together.

### Other Useful Commands

| Command                    | Description                              |
|---------------------------|------------------------------------------|
| `npm run dev`             | Backend + Frontend (main command)        |
| `npm run seed`            | Seed sample data (admin + students)      |
| `npm run dev:backend-only`| Start only Go backend                    |
| `go run cmd/server/main.go` | Legacy backend only                    |

**Default Credentials** (after `npm run seed`):
- Admin: `admin` / `admin123`
- Student: `2024001` / `siswa123` (token: `ujian2026`)
- Room Supervisor: `ruang_a` / `ruang123`

---

## 5. API ENDPOINTS

### Public Endpoints
| Method | Endpoint                    | Description                     |
|--------|-----------------------------|---------------------------------|
| GET    | `/`                         | Root health message             |
| GET    | `/api/health`               | Health check                    |
| POST   | `/api/auth/login`           | Admin / Supervisor login        |
| POST   | `/api/auth/student-login`   | Student login                   |
| POST   | `/api/ispring/webhook`      | Receive iSpring quiz results    |

### Protected Endpoints (require JWT)
| Method | Endpoint           | Description                        |
|--------|--------------------|------------------------------------|
| GET    | `/api/me`          | Get current logged-in user         |
| GET/POST | `/api/tenants`   | Manage tenants (superadmin only)   |
| GET/POST | `/api/users`     | Manage users                       |
| GET/POST | `/api/students`  | Manage students                    |
| GET/POST | `/api/classes`   | Manage classes                     |
| GET/POST | `/api/mapel`     | Manage subjects                    |
| GET/POST | `/api/rooms`     | Manage exam rooms                  |

---

## 6. DATABASE

**File**: `data/cbt_aether.db`

**Core Tables**:
- `tenants`
- `users`
- `peserta`
- `kelas`
- `mapel`
- `ruang`
- `hasil_tes`
- `cek_login`
- `settings`

All tables (except `tenants`) contain `tenant_id` for isolation.

---

## 7. MULTI-TENANT ARCHITECTURE

- Every request goes through `TenantMiddleware`
- Tenant can be specified via headers: `X-Tenant-ID` or `X-Tenant-Slug`
- All queries must filter by `tenant_id`
- Default tenant = ID 1 (`slug: default`)
- Super Admin can manage multiple tenants

---

## 8. KNOWN ISSUES / TODO (Updated - Honest Assessment)

**Completed (solid):**
- Backend architecture, migrations, auth, multi-tenant, CORS
- Basic admin functionality (view + create data)
- Basic student login flow

**Still Major Gaps:**
- [ ] Frontend UI is still very crude and does not follow `docs/UI_Component_Library.md`
- [ ] No proper component library / design system implementation
- [ ] Student exam screen is only a placeholder (no real iSpring integration yet)
- [ ] Supervisor dashboard almost non-existent
- [ ] No Excel import/export
- [ ] No QR Code functionality
- [ ] Very limited error handling and user feedback
- [ ] No tests

---

## 9. NEXT DEVELOPMENT PRIORITIES (Recommended Order)

**High Priority (Foundation):**
1. **Implement Design System** – Build proper components following `docs/UI_Component_Library.md` (colors, typography, buttons, cards, tables, modals, etc.)
2. **Refactor existing Admin UI** to follow the new design system
3. **Improve Student Exam Screen** – Make it actually usable

**Next Features:**
4. Real iSpring content integration in student exam
5. Supervisor real-time monitoring
6. Excel import/export
7. QR Code support

**Later:**
- Polish, error handling, logging, tests, production readiness

---

## 10. DOCUMENTATION

All project documentation is located in `docs/`:

- `PRD.md` — Product Requirements Document
- `Technical_Architecture.md` — Architecture & API spec
- `Database_Schema.md` — Complete database schema
- `UI_Component_Library.md` — Design system

---

## 11. CONTACT / MAINTAINER

This project was built as a complete foundation for a modern multi-tenant CBT platform.

**Status**: 
- Backend is mature and reliable.
- Frontend is functional but still very basic and does not yet follow the design system specified in the documentation.
- Suitable for internal development and testing.
- **Not yet suitable** for real school deployment or pilot testing.

---

**New/Updated Files (as of latest update)**:
- `QUICKSTART.md` — Recommended starting guide
- Root `package.json` — `npm run dev` to run both backend + frontend
- `.opencode/skills/` — 4 skills to help development in opencode
- Basic admin pages (still need major UI improvement)
- Complete migration system + seeder

**Important Note**: The UI Component Library and Design System documented in `docs/UI_Component_Library.md` have **not been implemented yet**.

**End of Handoff Document**
