---
name: aether-cbt-go
description: Use when working on Go backend code for Aether CBT. Covers Fiber routes, multi-tenant middleware, JWT auth, handlers (tenant/user/student/kelas/mapel/ruang/exam/ispring), utils, config, database connection, and Go module structure. Trigger keywords: Go, Fiber, handler, middleware, tenant, JWT, api, internal/.
---

# Aether CBT - Go Backend Skill

This skill helps develop and maintain the Go + Fiber backend for the Aether CBT multi-tenant exam platform.

## Project Context
- Module: `github.com/anomalyco/aether-cbt`
- Entry points: `cmd/server/main.go`, `cmd/createadmin/main.go`
- Core packages:
  - `internal/api/handlers/` — all HTTP handlers
  - `internal/api/middleware/` — TenantMiddleware + AuthMiddleware
  - `internal/db/sqlite.go` — SQLite + WAL connection
  - `internal/models/` — structs (User, Tenant, etc.)
  - `internal/utils/` — auth (bcrypt + JWT), response wrappers, qrcode
  - `internal/config/config.go`
  - `internal/repository/` — data access (currently only tenant_repo)

## Key Patterns
- Every protected route uses `middleware.AuthMiddleware()`
- All data queries **must** filter by `tenant_id` from `c.Locals("tenant_id")`
- Use `utils.SuccessResponse` and `utils.ErrorResponse`
- Soft delete via `deleted_at IS NULL`
- JWT claims contain: `user_id`, `tenant_id`, `role`

## Important Files
- `cmd/server/main.go` — route registration (watch for unexported handler bugs)
- `internal/api/middleware/tenant.go` — currently defaults to tenant 1
- `internal/api/middleware/auth.go` — validates JWT (hardcoded secret)
- `internal/db/migrations/` — SQL files (no auto-runner yet)

## Common Tasks
- Adding new entity (kelas, mapel, ruang, etc.): create handler + register route + migration
- Fixing multi-tenant isolation: ensure every query has `WHERE tenant_id = ?`
- Auth changes: update `utils/auth.go` and middleware together

## Known Issues (as of handoff)
- Compile error: `handlers.iSpringWebhook` is unexported (lowercase)
- No migration runner on startup
- Database may be empty (0-byte file possible)
- Hardcoded JWT secret

Always check `HANDOFF.md` and `docs/Database_Schema.md` before making schema or auth changes.
