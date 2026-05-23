# Aether CBT

**Modern Multi-Tenant Computer-Based Testing Platform**

Aether CBT is a high-performance, offline-capable, and premium examination platform designed for educational institutions. Built with Go + SvelteKit, it supports up to 500+ concurrent students per tenant with true offline functionality.

## Tech Stack

- **Backend**: Go + Fiber
- **Database**: SQLite (WAL mode)
- **Frontend**: SvelteKit + TypeScript + Tailwind CSS
- **Real-time**: Server-Sent Events (SSE)
- **Offline**: PWA + Service Worker + IndexedDB

## Architecture

- Multi-tenant by design (each school = one tenant)
- Single binary deployment
- Strict data isolation between tenants
- Single-tenant usage supported (default tenant)

## Project Structure

```
aether-cbt/
├── cmd/server/main.go
├── internal/
│   ├── api/
│   ├── config/
│   ├── db/
│   ├── models/
│   ├── services/
│   ├── repository/
│   └── utils/
├── web/                 # SvelteKit frontend
├── data/
│   ├── soal/            # iSpring HTML5 assets per tenant
│   └── uploads/
└── docs/                # All documentation
```

## Getting Started

```bash
go run cmd/server/main.go
```

## Documentation

All project documentation is located in the `docs/` folder:

- `PRD.md` — Product Requirements Document
- `Technical_Architecture.md` — Technical architecture & API spec
- `Database_Schema.md` — Complete database schema
- `UI_Component_Library.md` — Design system & component library

## License

Private project.
