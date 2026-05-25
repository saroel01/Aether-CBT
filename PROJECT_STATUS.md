# Aether CBT - Project Status

## Status

Aether CBT is a hardened MVP moving toward production use. Core security gaps found during the iSpring review have been addressed, but real exam deployment still requires school-specific fixture testing, backup/restore rehearsal, and load evidence.

## Stable Foundation

- Go/Fiber backend with SQLite WAL.
- Automatic SQL migrations.
- SvelteKit frontend with admin, student, and supervisor routes.
- Tenant-aware request context and tenant-scoped core tables.
- JWT authentication for protected routes.
- Role middleware for admin, supervisor, superadmin, and student route scopes.
- Student active-session tracking through `cek_login`.
- Per-attempt result submission tokens for active exam sessions.
- Bcrypt storage for newly created/imported student passwords, with legacy plaintext compatibility for existing rows.

## iSpring Result Handling

The result webhook accepts standard iSpring POST fields and parses `dr` XML in `quizReport` form. See `docs/ISPRING_RESULT_INTEGRATION.md` for the final contract.

Important database guarantees now present in migrations:

- `cek_login(tenant_id, peserta_id, mapel_id)` is unique for active exam sessions.
- `hasil_tes(tenant_id, validasi)` is unique for final result upserts.

## Remaining Before Real Exam Deployment

1. Add real iSpring fixture tests from published QuizMaker output used by the school.
2. Complete deployment, backup, restore, and load-test evidence.
3. Configure production CORS, secrets, and credential rotation.
4. Review npm audit advisories before internet-facing development server use.
