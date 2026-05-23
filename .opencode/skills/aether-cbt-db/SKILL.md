---
name: aether-cbt-db
description: Use when working with SQLite database, migrations, schema, or seeding data for Aether CBT. Covers database connection, WAL mode, tenant isolation, migration files, and initial data setup. Trigger keywords: SQLite, database, migration, seed, schema, db/, peserta, kelas, hasil_tes.
---

# Aether CBT - Database & Migration Skill

This skill guides all database work for the Aether CBT multi-tenant platform.

## Database Details
- Engine: SQLite 3 with WAL mode (`_journal_mode=WAL`)
- File: `data/cbt_aether.db`
- Connection: `internal/db/sqlite.go`
- Current status (handoff): file may be 0 bytes — tables not created

## Core Tables (from docs/Database_Schema.md)
- `tenants` (id, slug, name, ...)
- `users` (tenant_id, username, password_hash, role, ...)
- `peserta` (students)
- `kelas`, `mapel`, `ruang`
- `hasil_tes` (exam results from iSpring)
- `cek_login`, `settings`

Every table except `tenants` **must** have `tenant_id`.

## Migration System
- Location: `internal/db/migrations/`
- Files use `+goose Up` format (but no goose runner is active yet)
- Current migrations:
  - 001_create_tenants.sql (includes default tenant id=1)
  - 002_create_users.sql
  - 003_create_default_admin.sql (broken hash)
  - 004_create_admin_user.sql (also broken hash)

## How to Apply Migrations Manually
Use `cmd/createadmin/main.go` as reference or run raw SQL via a temporary Go script or sqlite3 CLI.

Recommended first steps after fresh clone:
1. Run `go run cmd/createadmin/main.go` (after fixing DB connection)
2. Seed sample kelas, mapel, ruang, peserta for tenant 1

## Important Rules
- Always include `tenant_id` filter in every query
- Use soft delete (`deleted_at IS NULL`)
- Passwords: bcrypt (cost 14) via `internal/utils/auth.go`
- Settings table stores global exam `token` per tenant

## Known Problems
- No automatic migration on server start
- Default admin password hashes in migrations are placeholders (not real bcrypt)
- `peserta.password` column stores plaintext (security risk)

When editing schema, always update both the migration file **and** `docs/Database_Schema.md`.
