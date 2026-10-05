# Aether CBT — Quick Start (Developer)

This guide gets the multi-tenant CBT platform running locally.

## Prerequisites
- Go 1.26+ (see `go.mod`)
- Node.js 22 (for frontend)
- Git

## 1. Configure

Copy `.env.example` to `.env` and set at least:

```bash
ENV=development
JWT_SECRET=<output of: openssl rand -hex 32>
SETUP_ADMIN_PASSWORD=<your first admin password, min 8 chars>
```

There is no default admin account. If `SETUP_ADMIN_PASSWORD` is empty, create the first
admin with `go run ./cmd/createadmin -password '<password>'` instead.

## 2. Start Everything

From the project root:

```bash
npm run dev
```

This starts backend (Go) and frontend (SvelteKit) together using `concurrently`.

On first run the server will:
- Create `data/cbt_aether.db`
- Apply all 34 migrations from `internal/db/migrations` (embedded in the binary). Applied
  files are recorded in `schema_migrations` and never run twice; each file runs in one
  transaction. Set `MIGRATIONS_DIR` only to run migrations from a folder on disk instead.
- Create the admin user `admin` from `SETUP_ADMIN_PASSWORD` (only when no admin exists)

## 3. Seed Sample Data (Optional)

In another terminal:

```bash
npm run seed
# or
go run ./cmd/seed
```

This adds classes, subjects, 2 exam rooms, 8 students and room supervisors, and prints a
random exam token.

**Sample credentials (development data only)**
- Student: No. ID `2024001`, password `siswa123`, plus the printed exam token
- Room supervisor: `ruang_a` / `ruang123`

## 4. Login Flows

### Admin
1. Open http://localhost:5173/admin
2. Login with `admin` and the password you configured

### Student
1. Open http://localhost:5173/student/login
2. Use No. ID + password + the token of an active exam session
3. Choose a session. Entering a session requires that same session token (the frontend
   sends the token used at login automatically)

### Supervisor
http://localhost:5173/supervisor/login — supervisors can change their room password from
the dashboard header ("Ganti Password").

## Useful Commands

| Command                    | Description                      |
|----------------------------|----------------------------------|
| `make run`                 | Start backend                    |
| `make seed`                | Seed sample data                 |
| `make clean`               | Delete database + rebuild        |
| `go run ./cmd/server`      | Direct backend start             |
| `go run ./cmd/migratepasswords` | Hash legacy plaintext passwords (required for old data) |

## API Testing (with curl or Postman)

Before login, identify the tenant with `X-Tenant-ID: 1` (or `X-Tenant-Slug`, a tenant
subdomain, or `DEFAULT_TENANT_ID` on the server). After login the tenant comes from the JWT:
```
Authorization: Bearer <token>
```

## Environment Variables

| Variable                  | Required?  | Notes |
|---------------------------|------------|-------|
| `JWT_SECRET`              | **Yes**    | Min. 32 chars, random (`openssl rand -hex 32`). The server refuses to start with an empty or known-weak value. |
| `CORS_ALLOWED_ORIGINS`    | **Yes in production** | Comma-separated origins. Without it the server exits at startup in production. |
| `SETUP_ADMIN_PASSWORD`    | First start | Bootstraps the `admin` account when none exists. |
| `DEFAULT_TENANT_ID`       | Single-school servers | Tenant used when a request carries no tenant identifier (production). |
| `TRUSTED_PROXIES`         | Behind a proxy | IPs/CIDRs whose `X-Forwarded-For` is trusted (needed for correct login rate limiting). |
| `MIGRATIONS_DIR`          | Optional   | Empty = embedded migrations. |
| `PORT`, `DATABASE_URL`    | Optional   | Defaults: `3000`, `data/cbt_aether.db` |

## Notes

- Student passwords are mandatory (CSV column 6 is required for new students; existing
  students keep their password when the column is empty).
- Packages uploaded before answer-key extraction existed show "Tanpa kunci — unggah ulang"
  in the package list; upload them again so scores are graded by the server.
