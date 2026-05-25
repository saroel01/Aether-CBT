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
- Migrations run automatically on server start via `db.RunMigrations()` in `cmd/server/main.go`

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
- Migrations run automatically on startup (`internal/db/migrate.go`)
- Important recent migrations:
  - 017_create_exam_upsert_indexes.sql (unique constraints for cek_login and hasil_tes)
  - 018_alter_cek_login_attempt_token.sql (added attempt_token for anti-cheat)

## How to Apply Migrations
Migrations are applied automatically when the server starts (`go run cmd/server/main.go` or the compiled binary).

For development:
- Run `npm run dev` (it starts the Go backend)
- Or manually: `go run cmd/server/main.go`

Use `npm run seed` or `go run cmd/seed/main.go` to populate sample data.

## Important Rules
- Always include `tenant_id` filter in every query
- Use soft delete (`deleted_at IS NULL`)
- Passwords: bcrypt (cost 14) for new/imported users via `internal/utils/auth.go`. Legacy plaintext still accepted for migration.
- Settings table stores global exam `token` per tenant

## Security Hardening (Updated May 2026)
- `JWT_SECRET` is now strictly required from environment (no hardcoded fallback)
- Tenant isolation strengthened: TenantMiddleware no longer silently defaults to tenant 1 in production
- Legacy plaintext passwords still supported temporarily (documented trade-off)

## Available Global Skills (Superpowers)

Skill global berikut tersedia di proyek:

- `superpowers-executing-plans`
- `superpowers-writing-plans`
- `superpowers-subagent-driven-development`
- `superpowers-verification-before-completion`
- `superpowers-systematic-debugging`
- `superpowers-writing-skills`

Gunakan skill ini terutama saat menyusun rencana migrasi database, backup strategy, atau debugging isu data yang kompleks.

## Known Problems / Current State
- `peserta.password` supports both bcrypt and legacy plaintext (for migration compatibility)
- New/imported students are always stored with bcrypt (cost 14)
- JWT secret is now strictly enforced via environment (no hardcoded fallback)

When editing schema, always update both the migration file **and** `docs/Database_Schema.md`.
