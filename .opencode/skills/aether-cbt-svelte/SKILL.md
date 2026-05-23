---
name: aether-cbt-svelte
description: Use when developing the SvelteKit frontend for Aether CBT. Covers Svelte components, routes, Tailwind styling, API integration with Go backend, admin/supervisor/student dashboards, and exam UI. Trigger keywords: Svelte, SvelteKit, +page.svelte, frontend, web/, Tailwind, UI.
---

# Aether CBT - SvelteKit Frontend Skill

This skill supports building the SvelteKit + TypeScript + Tailwind frontend for the Aether CBT platform.

## Current State
- Location: `web/`
- Framework: SvelteKit 2 + Vite
- Styling: Tailwind CSS (assumed, add if missing)
- Current pages (minimal):
  - `web/src/routes/(admin)/+page.svelte` — Admin dashboard (dummy stats)
  - `web/src/routes/(student)/login/+page.svelte` — Student login
  - `web/src/routes/(supervisor)/` — empty

## Integration Points
- Backend runs on `http://localhost:3000/api/*`
- All protected calls require `Authorization: Bearer <jwt>`
- Student exam flow uses separate login (`/api/auth/student-login`)
- iSpring results come via webhook (no direct frontend call)

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

## Best Practices
- Use SvelteKit server load functions to fetch from Go API (avoid CORS issues)
- Store JWT in httpOnly cookie or localStorage (with care)
- Follow UI_Component_Library.md in docs/ for design system
- Keep exam UI offline-capable (store answers in IndexedDB if needed)

## Common Tasks
- Building admin CRUD for students/classes/subjects/rooms
- Supervisor live monitoring (use SSE from backend when implemented)
- Student exam player that loads iSpring content from `data/soal/`

Always refer to `HANDOFF.md` section 9 (Next Development Priorities) and `docs/UI_Component_Library.md`.
