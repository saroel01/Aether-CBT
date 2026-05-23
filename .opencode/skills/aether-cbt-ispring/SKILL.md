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
- Tenant resolved via `TenantMiddleware` (currently always tenant 1)

## Data Mapping
- `peserta.no_id` ↔ `sid` from iSpring
- Score saved as string (consider converting to float/int)
- `detail_xml` stores full iSpring XML for later validation/reporting

## Frontend Responsibilities
- Load iSpring content from local folder (offline mode)
- Pass `sid` (student no_id) to iSpring player
- After completion, iSpring auto-posts to webhook — no extra frontend call needed

## Security & Validation Notes
- Webhook is currently unauthenticated (anyone can post results)
- Add HMAC or shared secret validation in production
- `hasil_tes.validasi` field is used to mark the result as coming from specific student

## Related Tables
- `peserta`
- `hasil_tes`
- `settings` (stores exam title, token, data_soal_path)

## Future Enhancements
- Add SSE or polling so supervisor sees live results
- Build result validation UI in admin panel
- Support multiple iSpring quizzes per subject

Always coordinate changes here with `aether-cbt-db` skill and `docs/Database_Schema.md`.
