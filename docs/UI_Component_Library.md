# UI COMPONENT LIBRARY SPECIFICATION
## Aether CBT — Modern Computer-Based Testing Platform

**Version**: 1.0  
**Design System**: Modern Professional Elegance  
**Date**: 23 May 2026

---

## 1. DESIGN SYSTEM OVERVIEW

### 1.1 Philosophy
**"Calm Confidence"** — Every component should make users feel in control, focused, and respected. The interface must feel premium without being distracting.

### 1.2 Core Principles
- **Clarity over decoration**
- **Subtle depth, never flat**
- **Generous whitespace**
- **Purposeful motion**
- **High readability at all times**

---

## 2. COLOR SYSTEM

### 2.1 Semantic Colors

```css
:root {
  /* Base */
  --color-bg: #F8FAFC;
  --color-surface: #FFFFFF;
  --color-surface-2: #F1F5F9;

  /* Text */
  --color-text-primary: #0F172A;
  --color-text-secondary: #475569;
  --color-text-muted: #94A3B8;

  /* Brand */
  --color-primary: #6366F1;
  --color-primary-hover: #4F46E5;
  --color-primary-light: #E0E7FF;

  /* Status */
  --color-success: #10B981;
  --color-success-light: #D1FAE5;
  --color-warning: #F59E0B;
  --color-warning-light: #FEF3C7;
  --color-danger: #EF4444;
  --color-danger-light: #FEE2E2;

  /* Border */
  --color-border: #E2E8F0;
  --color-border-strong: #CBD5E1;
}
```

---

## 3. TYPOGRAPHY

### 3.1 Type Scale

| Token          | Size   | Weight    | Line Height | Usage                     |
|----------------|--------|-----------|-------------|---------------------------|
| `text-xs`      | 12px   | 400       | 1.5         | Captions, timestamps      |
| `text-sm`      | 14px   | 400/500   | 1.5         | Secondary text            |
| `text-base`    | 16px   | 400       | 1.6         | Body text                 |
| `text-lg`      | 18px   | 500       | 1.6         | Important body            |
| `text-xl`      | 20px   | 600       | 1.5         | Card titles               |
| `text-2xl`     | 24px   | 600       | 1.4         | Section headers           |
| `text-3xl`     | 30px   | 700       | 1.3         | Page titles               |
| `text-4xl`     | 36px   | 700       | 1.2         | Hero / Login titles       |

**Font Stack**: `Inter`, system-ui, sans-serif

---

## 4. SPACING SYSTEM

8-point grid system:

```css
--space-1: 4px;
--space-2: 8px;
--space-3: 12px;
--space-4: 16px;
--space-5: 20px;
--space-6: 24px;
--space-8: 32px;
--space-10: 40px;
--space-12: 48px;
--space-16: 64px;
```

---

## 5. COMPONENT LIBRARY

### 5.1 Button

**Variants**:
- `primary` — Main actions (Start Exam, Save, Submit)
- `secondary` — Secondary actions
- `ghost` — Subtle actions
- `danger` — Destructive actions

**Sizes**: `sm`, `md`, `lg`

**States**: Default, Hover, Active, Disabled, Loading

**Example**:
```svelte
<Button variant="primary" size="lg">
  Mulai Ujian
</Button>
```

### 5.2 Card

**Base Card**:
- Background: `white`
- Border: `1px solid var(--color-border)`
- Shadow: `0 1px 3px rgba(15, 23, 42, 0.08)`
- Border radius: `12px`
- Padding: `24px`

**Elevated Card**:
- Stronger shadow for important content

### 5.3 Input

**Text Input**:
- Height: 44px
- Padding: `0 16px`
- Border radius: `8px`
- Focus ring: `2px solid var(--color-primary)`

**Variants**: Default, Error, Disabled

### 5.4 Table

**Features**:
- Clean header with subtle background
- Striped rows (`--color-surface-2`)
- Hover highlight
- Responsive with horizontal scroll on mobile

**Components**:
- `DataTable`
- `SortableHeader`
- `Pagination`

### 5.5 Modal / Dialog

- Backdrop: `rgba(15, 23, 42, 0.6)` with blur
- Content: White card with generous padding
- Animation: Fade + scale (150ms)

### 5.6 Toast / Notification

- Position: Top-right
- Variants: Success, Warning, Error, Info
- Auto-dismiss after 4 seconds (except errors)

### 5.7 Status Badge

| Variant     | Background          | Text Color     |
|-------------|---------------------|----------------|
| `success`   | `#D1FAE5`           | `#065F46`      |
| `warning`   | `#FEF3C7`           | `#92400E`      |
| `danger`    | `#FEE2E2`           | `#991B1B`      |
| `info`      | `#E0E7FF`           | `#3730A3`      |
| `neutral`   | `#F1F5F9`           | `#475569`      |

### 5.8 Timer Component

Special component for exam countdown:
- Large, highly readable font
- Color changes: Green → Amber → Red based on remaining time
- Smooth second-by-second animation

### 5.9 Loading States

- **Skeleton**: Clean gray blocks matching content layout
- **Spinner**: Subtle indigo spinner (only when necessary)
- **Progress Bar**: For file uploads and long operations

---

## 6. LAYOUT PATTERNS

### 6.1 Admin Dashboard
- Top navigation bar (logo + user)
- Sidebar (collapsible)
- Main content area with generous padding

### 6.2 Supervisor View
- Full-width status cards at top
- Live student table below
- Minimal chrome, maximum information density

### 6.3 Student Exam View
- Clean centered content
- Fixed top bar with timer + student info
- Full focus mode (minimal distractions)

---

## 7. MOTION & INTERACTION

- **Duration**: 150ms for most transitions
- **Easing**: `cubic-bezier(0.4, 0, 0.2, 1)` (standard ease)
- **Hover Scale**: `1.02` on primary buttons and cards
- **Focus Ring**: Always visible for accessibility

---

## 8. RESPONSIVE BREAKPOINTS

```css
--bp-sm: 640px;
--bp-md: 768px;
--bp-lg: 1024px;
--bp-xl: 1280px;
```

Mobile-first approach. All components must work well on tablets (common in schools).

---

## 9. ACCESSIBILITY

- WCAG 2.1 AA compliance target
- Minimum 4.5:1 contrast ratio
- All interactive elements keyboard accessible
- Proper ARIA labels on complex components
- Focus management in modals

---

## 10. ICONOGRAPHY

- **Library**: Lucide Icons (or Heroicons)
- **Size**: 16px, 20px, 24px
- **Stroke Width**: 2px
- **Color**: Inherits from text color or uses semantic colors

---

## 11. COMPONENT NAMING CONVENTION

All components follow PascalCase and are located in:

```
src/lib/components/
├── ui/           # Base components (Button, Input, Card, etc.)
├── layout/       # Layout components (Sidebar, Navbar, etc.)
├── exam/         # Exam-specific (Timer, QuestionNav, etc.)
├── admin/        # Admin-specific components
└── tenant/       # Tenant-aware components (TenantSelector, TenantHeader, etc.)
```

---

## 12. MULTI-TENANT UI CONSIDERATIONS

- Admin interface harus menampilkan **Tenant Context** di header (nama sekolah + logo)
- Super Admin (jika ada) dapat berpindah antar tenant
- Semua halaman admin/supervisor/student **harus** menerima `tenant_id` dari middleware
- URL structure: `/tenant/{slug}/admin/...` atau `/tenant/{slug}/supervisor/...`

---

**This component library is designed to be beautiful, consistent, and delightful while remaining extremely practical for a high-stakes examination environment.**

---

## END OF DOCUMENTATION SET

Semua dokumen telah di-update secara presisi dengan arsitektur **Multi-Tenant by Design**:

1. `PRD.md`
2. `Technical_Architecture.md`
3. `Database_Schema.md`
4. `UI_Component_Library.md`

Dokumen ini sekarang **final** dan siap untuk implementasi.

Ready for implementation.
