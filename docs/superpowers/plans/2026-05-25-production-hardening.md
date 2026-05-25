# Aether CBT Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move Aether CBT from demo-grade MVP toward a safer, deployable school CBT application.

**Architecture:** Harden the existing Go/Fiber and SvelteKit code in place. Keep compatibility with existing SQLite data while adding security controls around roles, student sessions, iSpring submissions, credentials, and runtime configuration.

**Tech Stack:** Go/Fiber, SQLite WAL, SvelteKit, Tailwind, iSpring QuizMaker result XML.

---

## Task Status

- [x] Add parser-backed iSpring `quizReport` result handling with automated tests.
- [x] Add SQLite migration coverage for result/session upsert indexes.
- [x] Issue JWTs from student login and require student JWTs for exam start/progress routes.
- [x] Add role middleware for admin, supervisor, superadmin, and student route scopes.
- [x] Add per-attempt tokens so iSpring/web simulator submissions must match an active exam session.
- [x] Replace hardcoded frontend API URLs and tenant headers with `web/src/lib/api.ts` helpers.
- [x] Add read-only supervisor settings endpoint so room dashboards show the active server token.
- [x] Store newly created/imported student passwords as bcrypt while accepting legacy plaintext rows.
- [x] Upgrade Svelte runtime/tooling compatibility and remove build warnings.
- [x] Update documentation so current behavior and remaining risks are stated consistently.

## Remaining Before Real Exam Deployment

- [ ] Add acceptance fixture XML files exported from real iSpring QuizMaker projects used by the school.
- [ ] Add backup/restore rehearsal documentation and a tested restore command for `data/cbt_aether.db`.
- [ ] Add full login rate limiting (currently only webhook has rate limiting).
- [ ] Run concurrent exam/load tests using realistic student counts, result payload sizes, and SQLite WAL settings.
- [ ] Rotate default admin/supervisor/student credentials before each deployment.

## Additional Security Hardening Completed (Post-Initial Plan)

- [x] Fixed JWT algorithm confusion vulnerability in AuthMiddleware.
- [x] Removed hardcoded weak JWT secret; enforced strict environment-only secret with panic on missing value.
- [x] Added BodyLimit (5MB) and per-IP rate limiting specifically on the iSpring webhook endpoint.
- [x] Replaced wildcard CORS (`*`) with proper allow-list via `CORS_ALLOWED_ORIGINS` (required in production).
- [x] Strengthened TenantMiddleware: no longer silently defaults to tenant 1 in non-development environments.
