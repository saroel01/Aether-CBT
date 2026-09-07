# Project: Aether-CBT Comprehensive UI/UX Redesign

## Architecture
Aether-CBT is an enterprise Computer-Based Testing (CBT) platform combining a Go Fiber backend (`cmd/server/main.go`) and a SvelteKit SPA frontend (`web/`).
The redesign establishes the **'Precision Cobalt & Institutional Slate'** aesthetic system across all user-facing portals:
- **Design Tokens & Foundation**: Centralized tokens in `web/tailwind.config.js` and `web/src/app.css` defining Cobalt primary accents (`#1D4ED8` / `#2563EB`), Slate neutral palette, and functional status colors (Emerald, Amber, Ruby).
- **Atomic Components**: Reusable UI primitives in `web/src/lib/components/ui/` with border-based elevation, micro-shadows (`shadow-sm`), keyboard focus-rings (`focus-visible:ring-2`), and `tabular-nums`.
- **Student CBT Portal** (`/student/*`): Focused, anxiety-reducing, distraction-free examination environment with stable timers, authoritative anti-cheat overlays, and scaled iSpring player preservation.
- **Proctor Workspace** (`/supervisor`): High-density live monitoring cockpit with compact tabular data, accessible modal confirmations, and a classroom projector presentation modal.
- **Admin Operations Panel** (`/admin/*`): Cohesive institutional workspace with unified table structures, accessible destructive action confirmations, bug fixes (`goto` import, QR code port), and preserved backend API contracts.

## Feature Inventory
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| 1 | Anti-Slop Visual Purge | Remove all 6 blurred glow circles (`blur-[100px]` to `blur-[140px]`), wild gradients, scale bounces, pulsing rings, and thin neon lines | M1 | Survey 1, 2, 3 |
| 2 | Color Tokens System | Establish Precision Cobalt, Institutional Slate, and Emerald/Amber/Ruby status tokens in Tailwind config | M1 | Survey 1 |
| 3 | Typography & Baseline Standards | Enforce `text-balance`, `text-pretty`, `tabular-nums`, and clear typographic hierarchy | M1 | Survey 1, SKILL baseline-ui |
| 4 | Landing Page Polish | Clean root landing page (`/`) to Precision Cobalt without blur blobs or scale distortion | M1 | Survey 1 |
| 5 | Button Component Refactor | Normalize variants (primary, secondary, ghost, danger, warning), remove scale-bounce, add `focus-visible:ring-2` | M2 | Survey 1, 2 |
| 6 | Card & Container Elevation | Replace arbitrary OKLCH colors with crisp `border-slate-200`/`border-slate-800` + `shadow-sm` | M2 | Survey 1 |
| 7 | Form Inputs & Controls | Refactor Input.svelte with clear focus rings, label typography, and error states | M2 | Survey 1 |
| 8 | Dense & Themed Table Component | Update Table.svelte with compact density modes and consistent header/row styling | M2 | Survey 2 |
| 9 | Badges & Status Indicators | High-contrast Emerald/Amber/Ruby/Cobalt badges adhering to WCAG 2.1 AA | M2 | Survey 1, 2 |
| 10 | Timer Component Stability | Add `tabular-nums font-mono` to Timer.svelte, prevent CLS, and add Amber warning state | M2 | Survey 1, 2 |
| 11 | Accessible Modal & Confirm Dialog | Enhance Modal.svelte (Escape, backdrop click) and create reusable ConfirmModal.svelte for destructive actions | M2 | Survey 2, 3 |
| 12 | Student Login Redesign | Clean `/student/login` without blur blobs, high contrast, crisp inputs, and focus rings | M3 | Survey 1, 2 |
| 13 | Student Subject Selection | Redesign `/student/select-subject`, remove 100px/3xl blurs and gradients, clear exam cards | M3 | Survey 1, 2 |
| 14 | Exam Room & Distraction-Free Header | Redesign `/student/exam` header, stable countdown timer with `min-w`, clean question navigation | M3 | Survey 2 |
| 15 | iSpring Iframe Scaler Preservation | Preserve responsive iframe scaling math and container dimensions in `/student/exam` | M3 | Survey 1, 2 |
| 16 | 2-Step Exam Submission Safety | Redesign "Hentikan Ujian" modal with Ruby danger action and deliberate confirmation | M3 | Survey 2 |
| 17 | Authoritative Anti-Cheat & Lock Overlays | Professional, authoritative anti-cheat warning modal and server lock overlay | M3 | Survey 2 |
| 18 | Proctor Login Redesign | Clean `/supervisor/login`, remove 140px blur blob and hairlines | M4 | Survey 1, 2 |
| 19 | Proctor High-Density Cockpit | Redesign `/supervisor` monitoring table (`py-2.5 px-3`) for 30-50 simultaneous students | M4 | Survey 2 |
| 20 | Proctor Metrics & Tabular Numerals | Compact stats cards with `tabular-nums font-mono` for attendance, active, and completed students | M4 | Survey 2 |
| 21 | Safe Proctor Actions | Fix `Button variant="warning"` on Unlock, replace `window.confirm` with ConfirmModal for Reset | M4 | Survey 2 |
| 22 | Projector QR Presentation Modal | Add high-contrast, fullscreen-friendly Projector Presentation modal for room token & QR code | M4 | Survey 2 |
| 23 | Admin Layout & Theme Harmonization | Harmonize `/admin/+layout.svelte` sidebar and content canvas, use `h-dvh`, fix active link scale | M5 | Survey 3 |
| 24 | Fix Admin Logout Crash | Import `goto` from `$app/navigation` in `/admin/+layout.svelte` to fix logout ReferenceError | M5 | Survey 3 |
| 25 | Fix Hardcoded Port in Print Cards | Replace `:5173` with dynamic `window.location.origin` in `/admin/students/print-cards/+page.svelte` | M5 | Survey 3 |
| 26 | Admin Text Selection Usability | Remove restrictive `select-none` locks on administrative containers | M5 | Survey 3 |
| 27 | Admin Destructive Action Safety | Replace all 7 `window.confirm()` calls with ConfirmModal across all admin management pages | M5 | Survey 3 |
| 28 | Admin Master Data Redesign | Redesign Students, Classes, Rooms, Mapel, Tenants with unified table structures and modal forms | M5 | Survey 3 |
| 29 | Admin Exam Operations Redesign | Redesign Packages, Exams, Sessions, Settings, Results Analysis with Precision Cobalt theme | M5 | Survey 3 |
| 30 | Backend API Contract Preservation | Preserve all 30+ endpoints, auth headers, and multipart upload contracts untouched | M5 | Survey 3 |
| 31 | Full SvelteKit Build Verification | Verify `npm run build` succeeds with exit code 0 | M6 | Survey 1, 3 |
| 32 | Static Anti-Slop & Contrast Audit | Verify 0 `blur-[` instances, 0 hallucinated classes, and WCAG 2.1 AA compliance | M6 | Survey 1, 2 |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | M1: Tokens & Anti-Slop Foundation | `web/tailwind.config.js`, `web/src/app.css`, `web/src/routes/+page.svelte` | none | DONE |
| 2 | M2: Atomic UI Components | `web/src/lib/components/ui/*` (Button, Card, Input, Table, Badge, Timer, Modal, Toast, ConfirmModal) | M1 | DONE |
| 3 | M3: Student CBT Experience | `web/src/routes/student/*` (login, select-subject, exam) | M2 | DONE |
| 4 | M4: Proctor Workspace | `web/src/routes/supervisor/*` (login, dashboard cockpit, projector modal) | M2 | DONE |
| 5 | M5: Admin Operations Panel | `web/src/routes/admin/*` (layout bug fix, print-cards port fix, all admin pages, ConfirmModal) | M2 | DONE |
| 6 | M6: QA, Polish & Build Integrity | `web/build`, full build check, slop grep, WCAG audit, regression tests | M3, M4, M5 | DONE |

## Interface Contracts
### UI Tokens & Primitives ↔ Application Routes
- `Button.svelte`:
  - Props: `variant` ('primary' | 'secondary' | 'ghost' | 'danger' | 'warning'), `size` ('sm' | 'md' | 'lg'), `theme` ('dark' | 'light'), `class`, `disabled`, `type`.
  - Styling: `focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2`, `rounded-xl`, no bouncy scale transforms.
- `Card.svelte`:
  - Props: `theme` ('dark' | 'light'), `padded` (boolean), `class`.
  - Border elevation: `border-slate-200` (light) / `border-slate-800` (dark), `shadow-sm`, `rounded-xl`/`rounded-2xl`.
- `ConfirmModal.svelte`:
  - Props: `isOpen` (boolean), `title` (string), `message` (string), `confirmLabel` (string), `cancelLabel` (string), `variant` ('danger' | 'warning' | 'primary'), `loading` (boolean).
  - Events: `confirm`, `cancel`.
- `Timer.svelte`:
  - Formatting: strict `tabular-nums font-mono`.

## Code Layout
- `web/tailwind.config.js`: Central color palette tokens and font configurations.
- `web/src/app.css`: Global base styles, selection colors, scrollbar polish.
- `web/src/lib/components/ui/`: Atomic component primitives.
- `web/src/routes/`: Root landing page.
- `web/src/routes/student/`: Student CBT examination routes.
- `web/src/routes/supervisor/`: Proctor and supervisor cockpit routes.
- `web/src/routes/admin/`: Admin operations, master data, and settings routes.
