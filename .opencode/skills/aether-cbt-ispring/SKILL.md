---
name: aether-cbt-ispring
description: Use when implementing or debugging iSpring QuizMaker integration for Aether CBT. Covers webhook receiver, result storage in hasil_tes, student mapping via no_id, and exam content delivery. Trigger keywords: iSpring, webhook, hasil_tes, dr, sp, tp, sid, ispring.
---

# Aether CBT - iSpring Integration Skill

This skill covers the iSpring QuizMaker external quiz engine integration.

## How It Works
1. Students take exams in iSpring content (HTML5 quizzes exported to `data/soal/`)
2. When quiz finishes, iSpring posts results to `POST /api/ispring/webhook`
3. Backend receives: `sid` (student no_id), `sp` (score), `tp` (max score), `dr` (detail XML)
4. Result is stored in `hasil_tes` table with `validasi = no_id`

## Current Implementation
- Handler: `internal/api/handlers/ispring.go`
  - Function `iSpringWebhook` (note: currently unexported — causes compile error)
- Route registered in `cmd/server/main.go:74` (public, no auth)
- Tenant resolved via `TenantMiddleware` (defaults to 1 only in dev; requires explicit tenant in production)

## Data Mapping
- `peserta.no_id` ↔ `sid` from iSpring
- Score saved as string (consider converting to float/int)
- `detail_xml` stores full iSpring XML for later validation/reporting

## Frontend Responsibilities
- Load iSpring content from local folder (offline mode)
- Pass `sid` (student no_id) to iSpring player
- After completion, iSpring auto-posts to webhook — no extra frontend call needed

## Security Hardening (Updated May 2026)
- Webhook is protected with:
  - Per-IP rate limiting (10 requests per minute)
  - Body size limit (5MB)
- Strong anti-cheat: Requires matching `attempt_token` from active `cek_login` session
- Submission rejected if outside grace period or without valid session
- Tenant resolution is stricter in production (no silent default to tenant 1)

## Available Global Skills (Superpowers)

Skill global berikut tersedia dan dapat digunakan:

- `superpowers-executing-plans`
- `superpowers-writing-plans`
- `superpowers-subagent-driven-development`
- `superpowers-verification-before-completion`
- `superpowers-systematic-debugging`
- `superpowers-writing-skills`

Gunakan skill ini untuk tugas perencanaan, eksekusi roadmap, dan debugging yang kompleks.

## Related Tables
- `peserta`
- `hasil_tes`
- `settings` (stores exam title, token, data_soal_path)

## Future Enhancements
- Add SSE or polling so supervisor sees live results
- Build result validation UI in admin panel
- Support multiple iSpring quizzes per subject

Always coordinate changes here with `aether-cbt-db` skill and `docs/Database_Schema.md`.
