# iSpring Shim — Manual Verification Checklist

**Status:** Manual verification required before each exam day.
**Why manual:** the shim intercepts the browser network layer (`XMLHttpRequest`, `fetch`,
`sendBeacon`, form submit) so the iSpring player's proprietary `data/player.js` must run in
a real browser. The fixture in `contoh_soal/KIMIA_XII_UAS_2025 (Published)` is **incomplete**
(it ships only `index.html`, `ismplayer.html`, `metainfo.xml`, `preview.png` — no
`data/player.js`), so a headless automated run cannot exercise the player.

This document is the Requirement 9.6 fallback: automated Go tests cover the shim's
*injection* and *content* (`internal/soalpkg`), and Property 10's no-loss guarantee is
covered by `internal/submission/property10_test.go`. This checklist covers the
*end-to-end* browser behaviour that only a real player exercises.

---

## Prerequisites

1. A **complete** iSpring QuizMaker HTML5 export — the ZIP must contain `index.html` **and**
   `data/player.js` (plus the rest of the `data/` tree the quiz references). Re-export from
   QuizMaker if your fixture is missing `data/`.
2. In QuizMaker → **Reporting**, enable **"Send quiz result to server"** at export. The
   server *address field can be any placeholder* — the shim overrides it at runtime to
   `/api/ispring/webhook`. (See `design.md` AD-3 and HANDOFF §3.6.)
3. The backend running (`go run cmd/server/main.go`) and the frontend built
   (`npm run build` inside `web/`).

## Setup

1. As **admin**, upload the complete ZIP via **Admin → Paket Soal**.
2. Create an **exam** linking that package (Admin → Definisi Ujian).
3. Create an **exam session** (Admin → Sesi Ujian) with:
   - a window bracketing *now*,
   - the student's class linked,
   - status `aktif`.
4. Note the session **token**.
5. As the **student**, log in with the token, start the session, and open the exam page.

## Verification steps

Perform each in the student exam page (the iSpring player inside the iframe). Expected =
the shim is working. If any step deviates, the shim is **not** intercepting and the result
will not reach the server — **do not run the exam**.

| # | Action | Expected |
|---|--------|----------|
| 1 | The exam page loads | The iSpring player renders inside the iframe (not a blank page or a 403). |
| 2 | Open DevTools → Network | Sub-asset requests (`data/player.js`, fonts, images) return 200 from `/api/exam/content/...`. |
| 3 | Answer a question, click **Finish/Submit** in the player | A `POST /api/ispring/webhook` request appears with body fields `dr`, `sp`, `tp` **and** the shim-appended `attempt_token`, `tenant_id`, `sid`. The destination is the **same origin** (relative), NOT the placeholder URL typed in QuizMaker. |
| 4 | The webhook response | `200 OK` (accepted/queued). If `403 Invalid attempt token` → the session was not started; if `500` → check the queue/worker logs. |
| 5 | Backend check | After the worker flushes (≤ a few seconds), `hasil_tes` has exactly one row for this `(tenant, no_id, session)`, and the matching `cek_login` row was deleted. |
| 6 | Re-submit the same result (replay) | Still one `hasil_tes` row (UPSERT idempotency, Property 9). |
| 7 | Trigger ≥3 tab-switch infractions | The server locks the session; subsequent `/api/exam/content/*` requests return `403` and the student sees the lock overlay (Property 11). |

## What is already covered by automated tests

- Shim injection only on `index.html`, disk untouched → `internal/soalpkg/*_test.go`
  (Properties 4, 12).
- Anti-zip-slip, non-ZIP rejection, cleanup on failure → `internal/soalpkg/storage_test.go`
  (Properties 2, 3).
- Content auth: owner vs non-owner (403), window, lock, traversal →
  `internal/api/handlers/content_serving_test.go`.
- No lost results under 500-concurrent submission →
  `internal/submission/property10_test.go` (Property 10).
- Webhook `validasi` key session-based, idempotent UPSERT → `ispring_test.go`,
  `features_test.go` (Property 9).

## What only this checklist covers

The live player emitting the result POST through the browser network layer and the shim
re-routing it. This is why it is mandatory before each exam day.
