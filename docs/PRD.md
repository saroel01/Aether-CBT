# PRODUCT REQUIREMENTS DOCUMENT (PRD)
## Aether CBT — Modern Computer-Based Testing Platform

**Version**: 1.0  
**Date**: 23 May 2026  
**Tech Stack**: Go (Fiber) + SQLite + SvelteKit + Tailwind CSS + PWA  
**Status**: Final Draft

---

## 1. EXECUTIVE SUMMARY

**Aether CBT** is a next-generation Computer-Based Testing (CBT) platform designed specifically for educational institutions. It is a complete rebuild of the legacy `cbt_sekolah` system with a strong focus on performance, reliability, offline capability, and premium user experience.

The platform enables schools to conduct large-scale digital examinations (up to 500 concurrent students) even in environments with limited or no internet access. It integrates seamlessly with iSpring QuizMaker HTML5 output while offering a modern, elegant, and trustworthy interface.

**Core Differentiators**:
- Single-binary deployment (Go)
- True offline-first architecture
- Premium luxury design language
- Extremely low resource consumption
- Enterprise-grade reliability for mission-critical exams

---

## 2. PRODUCT VISION & GOALS

### 2.1 Vision Statement
To become the most reliable, elegant, and accessible digital examination platform for schools in Indonesia — empowering educators and students with a calm, confident, and professional testing experience.

### 2.2 Success Metrics (First 12 Months)

| Metric                        | Target                  | Measurement Method             |
|-------------------------------|-------------------------|--------------------------------|
| Concurrent Users Supported    | 500+ students per tenant | Load testing                   |
| Average Response Time         | < 200ms                 | APM / Server logs              |
| System Uptime During Exams    | 99.9%                   | Monitoring dashboard           |
| Offline Functionality         | 100% core flows         | QA testing                     |
| User Satisfaction (NPS)       | ≥ 70                    | Post-exam survey               |
| Deployment Time (New School)  | < 15 minutes            | Internal benchmark             |
| Multi-Tenant Isolation        | Zero data leakage between tenants | Security audit          |
| Bug Reports (Critical)        | 0 during exam windows   | Issue tracker                  |

---

## 3. TARGET USERS & PERSONAS

### 3.1 Primary Personas

**1. Bu Sari – School Administrator / Proktor**
- Age: 42 | Tech-savvy but not a developer
- Goals: Easy setup, real-time monitoring, reliable result export
- Pain Points: Legacy system is slow, hard to monitor multiple rooms, complicated Excel imports
- Success Criteria: Can manage an entire exam day with minimal IT support

**2. Pak Budi – Room Supervisor (Pengawas)**
- Age: 38 | Moderate tech literacy
- Goals: Monitor his assigned room, reset specific students quickly, see live status
- Pain Points: Cannot see who is still working, resetting students is tedious
- Success Criteria: Feels in full control of his room during the exam

**3. Andi – Grade 12 Student**
- Age: 17 | Digital native
- Goals: Clean interface, clear timer, confidence that answers are saved even if connection drops
- Pain Points: Old interface feels cheap and stressful, fear of losing progress
- Success Criteria: Feels calm and focused throughout the exam

### 3.2 Secondary Users
- School Principal (high-level result dashboards)
- Subject Teachers (question analysis & result review)
- IT Staff (deployment & maintenance)

---

## 4. CORE FEATURES & SCOPE

### 4.1 MVP Features (Phase 1)

**Administration Module**
- Full CRUD for Students, Classes, Subjects, Rooms
- Bulk import/export via Excel (.xlsx)
- iSpring HTML5 quiz upload and configuration
- Token management (generate, rotate, reset)
- Exam activation per subject/class
- Global settings (school name, logo, footer, exam title)

**Supervision Module**
- Room-based login for supervisors
- Live student status dashboard per room
- Individual student reset capability
- Token display (QR code + text)
- Real-time count of students working vs finished

**Student Module**
- Secure login (No ID + Password + Global Token)
- Subject selection (if multiple active)
- iSpring quiz delivery with timer
- Auto-submit on time expiry
- Graceful handling of connection loss

**Result Module**
- View and export results (Excel + PDF)
- Basic scoring and duration data
- Validation of submitted results

### 4.2 Post-MVP Features (Phase 2+)

- Detailed answer analysis (parsed from iSpring XML)
- Student photo capture & verification
- Advanced analytics dashboard
- Offline mode with background sync
- Native question authoring (beyond iSpring)
- Multi-tenant management dashboard (untuk super admin)

---

## 5. TECHNICAL REQUIREMENTS

### 5.1 Technology Stack (Locked)

| Layer          | Technology                          | Rationale |
|----------------|-------------------------------------|---------|
| Backend        | Go 1.22+ + Fiber                    | Highest performance, single binary, minimal memory |
| Database       | SQLite 3 (WAL mode)                 | True offline support, single file, zero config |
| Frontend       | SvelteKit 2 + TypeScript            | Excellent DX, smallest bundle size, modern |
| Styling        | Tailwind CSS v4                     | Consistency, speed, premium aesthetic control |
| Real-time      | Server-Sent Events (SSE)            | Lightweight, sufficient for monitoring |
| Offline        | PWA + Service Worker + IndexedDB    | Full offline capability |
| Authentication | JWT + secure session cookies        | Industry standard, simple to implement |
| File Handling  | Go standard library + excelize      | No heavy external dependencies |

### 5.2 Non-Functional Requirements

- **Performance**: Support 500 concurrent students with p95 latency < 200ms
- **Reliability**: Zero data loss during connection interruptions
- **Offline**: All core student flows must work without internet
- **Security**: Protection against SQL injection, XSS, CSRF, and brute-force
- **Maintainability**: Modular codebase with clear separation of concerns
- **Deployability**: Single binary + one database file deployment

---

## 6. DESIGN SYSTEM DIRECTION

### 6.1 Design Philosophy
**"Modern Professional Elegance"** — A calm, confident, and premium interface that respects the seriousness of examinations while providing a delightful user experience.

### 6.2 Visual Direction
- Clean, spacious layouts with excellent typography
- Subtle depth using soft shadows and elegant borders
- Indigo as primary accent color for trust and focus
- High readability with generous line height and contrast
- Smooth, purposeful micro-interactions (not flashy)

### 6.3 Color Palette (Luxury Professional)

| Token              | Hex       | Usage                          |
|--------------------|-----------|--------------------------------|
| `--color-primary`  | `#0F172A` | Headers, text, dark backgrounds |
| `--color-surface`  | `#1E2937` | Cards, sidebars                |
| `--color-accent`   | `#6366F1` | Primary buttons, highlights    |
| `--color-success`  | `#10B981` | Completed status               |
| `--color-warning`  | `#F59E0B` | Time warnings                  |
| `--color-danger`   | `#EF4444` | Errors, critical alerts        |
| `--color-bg`       | `#F8FAFC` | Page backgrounds               |
| `--color-muted`    | `#64748B` | Secondary text                 |

### 6.4 Typography
- **Primary Font**: Inter (or Satoshi for premium feel)
- **Base Size**: 16px
- **Scale**: 1.25 modular scale
- **Headings**: Semibold/Bold with tight tracking
- **Body**: Regular with generous line-height (1.6)

---

## 7. OFFLINE STRATEGY

1. **Progressive Web App (PWA)** with Service Worker caching entire application shell
2. **IndexedDB** for temporary storage of student data and answers
3. **Background Sync** for automatic result submission when connection returns
4. **Graceful Degradation** — student can continue working even if connection drops mid-exam
5. **Conflict Resolution** — last-write-wins with clear visual indicators

---

## 8. FUTURE ROADMAP

- Native question editor (multiple choice, essay, matching, etc.)
- Advanced statistical analysis of results
- Integration API for school information systems
- Mobile companion app for supervisors
- Multi-school / multi-tenant architecture
- AI-assisted question analysis and cheating detection

---

## 9. ARCHITECTURE DECISION (FINAL)

**Multi-Tenant by Design**

Aplikasi dibangun dengan arsitektur **multi-tenant** sejak awal, dengan ketentuan berikut:

- Setiap sekolah = 1 tenant
- Data antar tenant **sepenuhnya terisolasi**
- Single tenant deployment tetap didukung (menggunakan 1 tenant default)
- Super Admin (opsional) dapat mengelola multiple tenants
- Setiap tenant memiliki:
  - Konfigurasi sendiri (nama sekolah, logo, token, settings)
  - Data peserta, hasil, ruangan, dll yang terisolasi
  - Subdomain atau path-based isolation (contoh: `/tenant/{slug}`)

Keputusan ini diambil karena:
- Lebih future-proof
- Single tenant tetap bisa berjalan tanpa perubahan
- Memungkinkan model SaaS di masa depan tanpa rewrite besar

---

**Document Status**: Ready for Technical Architecture phase.

*This PRD serves as the single source of truth for product decisions.*
