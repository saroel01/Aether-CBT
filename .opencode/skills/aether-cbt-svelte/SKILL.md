---
name: aether-cbt-svelte
description: Use when developing the SvelteKit frontend for Aether CBT. Covers Svelte components, routes, Tailwind styling, API integration with Go backend, admin/supervisor/student dashboards, and exam UI. Trigger keywords: Svelte, SvelteKit, +page.svelte, frontend, web/, Tailwind, UI.
---

# Aether CBT - SvelteKit Frontend Skill

This skill supports building the SvelteKit + TypeScript + Tailwind frontend for the Aether CBT platform.

## Current State
- Location: `web/`
- Framework: Svelte 5 + SvelteKit + Vite
- Styling: Tailwind CSS
- Key centralized file: `web/src/lib/api.ts` (API base, auth headers, tenant handling)
- Major pages:
  - Admin, Supervisor, and Student sections
  - Student exam flow with iSpring integration
  - Real-time supervisor monitoring (SSE)

## Integration Points
- Backend API: controlled via `web/src/lib/api.ts`
- Uses `VITE_API_BASE` and `VITE_TENANT_ID` environment variables
- All protected routes use JWT (stored in localStorage as `aether_token`)
- Student flow: Login → Select Subject → Start (gets `attempt_token`) → iSpring exam
- Results submitted to `/api/ispring/webhook` with `attempt_token` for anti-cheat

## Recommended Structure
```
web/src/
├── lib/
│   ├── api.ts           # centralized fetch with token
│   └── components/
│       └── ui/          # reusable Tailwind components
├── routes/
│   ├── (admin)/
│   │   ├── +layout.svelte
│   │   ├── +page.svelte
│   │   └── students/
│   ├── (supervisor)/
│   └── (student)/
│       ├── login/
│       └── exam/
```

## Security Hardening Awareness (Frontend)
- All authenticated requests must include JWT via `Authorization: Bearer` header
- Student exam flow requires `attempt_token` (obtained from `/api/student/start`)
- Results submission to iSpring webhook must carry the correct `attempt_token`
- Frontend respects `CORS_ALLOWED_ORIGINS` configured on backend
- Use `web/src/lib/api.ts` helpers to avoid leaking credentials or tenant assumptions

## Available Global Skills (Superpowers)

Skill global berikut tersedia dan dapat digunakan:

- `superpowers-executing-plans`
- `superpowers-writing-plans`
- `superpowers-subagent-driven-development`
- `superpowers-verification-before-completion`
- `superpowers-systematic-debugging`
- `superpowers-writing-skills`

Gunakan skill ini untuk perencanaan fitur frontend besar, refactoring, atau saat membuat dokumentasi UI yang kompleks.

## Best Practices
- Use centralized helpers in `web/src/lib/api.ts` (apiUrl, authHeaders, qrCodeUrl)
- Follow the UI components in `web/src/lib/components/ui/`
- Always send `X-Tenant-ID` header
- Student exam pages must handle `attempt_token` correctly
- Refer to `HANDOFF.md` for current security requirements (JWT + attempt_token flow)

## Common Tasks
- Building admin CRUD for students/classes/subjects/rooms
- Supervisor live monitoring (use SSE from backend when implemented)
- Student exam player that loads iSpring content from `data/soal/`

Always refer to `HANDOFF.md` section 9 (Next Development Priorities) and `docs/UI_Component_Library.md`.
