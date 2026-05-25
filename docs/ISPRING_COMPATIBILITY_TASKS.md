# iSpring Compatibility Task Plan

## Objective

Make Aether CBT's iSpring result handling robust, testable, and aligned with the official `ispringsolutions/QuizResults` sample without claiming unsupported production guarantees.

## Completed In This Revision

1. Add parser tests based on official `quizReport` XML shape.
2. Extract iSpring detail parsing from the HTTP handler into `internal/ispring`.
3. Support richer iSpring question types and answer resolution rules.
4. Integrate the parser into `POST /api/ispring/webhook`.
5. Add migration coverage for SQLite upsert conflict targets.
6. Add unique indexes for `cek_login` active sessions and `hasil_tes` result validation.
7. Change the built-in student simulator to generate `quizReport` XML instead of a custom `<report>` XML.
8. Publish final integration documentation in `docs/ISPRING_RESULT_INTEGRATION.md`.
9. Replace conflicting status documents with a single MVP/hardening status narrative.
10. Add per-attempt token validation for iSpring result submissions.
11. Add role-specific middleware for protected routes.
12. Remove critical hardcoded frontend localhost and tenant assumptions.
13. Store new/imported student credentials as bcrypt while preserving legacy login compatibility.
14. Upgrade Svelte tooling compatibility so production build is warning-clean.

## Remaining Production Tasks

1. Add real iSpring QuizMaker fixture XML files and acceptance tests for each question family used by schools.
2. Add operational tests for backup, restore, and concurrent exam submission.
3. Add full login rate limiting (webhook rate limiting + body limit already implemented; CORS allow-list already enforced).
4. Rotate all default credentials during deployment handoff.
