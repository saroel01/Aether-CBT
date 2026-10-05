# Aether CBT - Development Status

## Current Status

Aether CBT is a hardened MVP with a Go/Fiber backend, SQLite WAL storage, SvelteKit frontend, admin/student/supervisor flows, result export utilities, and an iSpring result webhook.

This project is suitable for controlled school pilot preparation. It is not declared fully production-ready until tests with the school's own iSpring packages, backup/restore rehearsal, and load tests are completed.

## Implemented

- Multi-tenant schema foundation with tenant-scoped tables; tenants are provisioned by a superadmin through the API.
- Admin, student, and supervisor authentication flows; supervisors can change their room password.
- JWT revocation on password change, deactivation, or deletion (`token_version`).
- Login rate limiting per account + IP.
- Student session tracking through `cek_login`; session start requires the session token.
- iSpring-compatible result webhook at `POST /api/ispring/webhook` with server-side grading from the package answer key.
- Parser for `quizReport` detail XML in `internal/ispring`.
- Normalized result detail storage in `hasil_tes_detail`; item analysis grouped per soal package.
- Bcrypt storage for all passwords; plaintext hashes are rejected.
- CSV/XLSX/PDF export features, scoped to the supervisor's room, with truncation flagged (`X-Export-Truncated`).
- Versioned, transactional migrations; non-root Docker image with healthcheck; GitHub Actions CI.

## Recently Hardened

- The built-in student simulator was removed.
- Per-route body limits (2 MB / 1 MB webhook) with streamed package uploads, plus read/write timeouts.
- CSP `object-src 'none'; base-uri 'self'`, HSTS over HTTPS, gzip compression, static asset caching.
- Late results are refused with 409 and listed for the supervisor.

## Remaining Before Production Use

- Validate grading against a real iSpring `dr` report from school packages.
- Complete load testing and operational backup/restore procedures.
