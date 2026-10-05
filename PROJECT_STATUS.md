# Aether CBT - Project Status

## Status

Aether CBT is a hardened MVP moving toward production use. The findings of the 2026-10 repository audit (C1, H1-H4, M1-M11, L1-L12) are addressed in code; real exam deployment still requires testing with the school's own iSpring packages, backup/restore rehearsal, and load evidence.

## Stable Foundation

- Go/Fiber backend with SQLite WAL.
- Versioned SQL migrations (`schema_migrations`, one transaction per file, embedded in the binary).
- SvelteKit frontend with admin, student, and supervisor routes; CI runs gofmt, go vet, go test, svelte-check and the frontend build.
- Tenant-aware request context and tenant-scoped core tables.
- JWT authentication with per-request revocation check (`token_version`, inactive/deleted accounts).
- Role middleware for admin, supervisor, superadmin, and student route scopes; supervisors only see results of their own room.
- Login rate limiting per account + IP (needs `TRUSTED_PROXIES` behind a proxy).
- All passwords stored with bcrypt; plaintext hashes are rejected at login (`go run ./cmd/migratepasswords` converts legacy data). No default passwords for new students.
- Student active-session tracking through `cek_login`; entering a session requires that session's token.
- Per-attempt result submission tokens for active exam sessions.

## iSpring Result Handling

The webhook treats `sp`/`tp`/`dr` as untrusted and the server grades results from the answer key extracted from the uploaded package (`score_source`: server / mixed / client / unmatched). Late results (beyond duration + 5 minutes) are refused with 409 and recorded in `submission_failure`. See `docs/ISPRING_RESULT_INTEGRATION.md`.

The built-in student simulator was removed; results only come from real iSpring packages.

## Remaining Before Real Exam Deployment

1. Validate grading with a real `dr` report from the school's packages (key extraction is verified against `contoh_soal/informatika-x-w.zip`; matching against a real report is not).
2. Complete deployment, backup, restore, and load-test evidence.
3. Frontend dev-tooling advisories (`npm audit` in `web/`) need major upgrades of vite/kit/tailwind; they affect the dev server and build, not the shipped static bundle.
