# DETAILED DATABASE SCHEMA
## Aether CBT — Modern Computer-Based Testing Platform

**Version**: 1.0  
**Database**: SQLite 3 (WAL Mode)  
**Date**: 23 May 2026

---

## 1. DATABASE DESIGN PHILOSOPHY

- **Single File**: One `.db` file for entire application (easy backup & restore)
- **WAL Mode**: Write-Ahead Logging enabled for better concurrency and reliability
- **Normalization**: 3NF with minimal redundancy
- **Audit Fields**: `created_at`, `updated_at` on all mutable tables
- **Soft Delete**: Most tables use `deleted_at` for safe recovery
- **Indexing Strategy**: Heavy indexing on frequently queried columns (`no_id`, `ruang_ujian`, `validasi`, etc.)

---

## 2. CORE TABLES

### 2.1 `tenants`

Master table for multi-tenant support. Every school is one tenant.

```sql
CREATE TABLE tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,              -- URL-friendly identifier (e.g. "sman1kluet")
    name TEXT NOT NULL,                     -- School name
    logo TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX idx_tenants_slug ON tenants(slug);
```

> **Note**: All subsequent tables must include `tenant_id` as a foreign key. Queries must always filter by `tenant_id`.

### 2.2 `settings`

Tenant-specific configuration.

```sql
CREATE TABLE settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_title TEXT NOT NULL,
    proctor_name TEXT,
    footer_text TEXT,
    token TEXT NOT NULL,                    -- Global exam token
    token_expiry DATETIME,
    is_exam_active BOOLEAN DEFAULT FALSE,
    data_soal_path TEXT,                    -- Path to iSpring folder
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_settings_tenant ON settings(tenant_id);
```

### 2.3 `users`

System users (Admin & Supervisor) — scoped per tenant.

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('admin', 'supervisor', 'superadmin')),
    ruang_id INTEGER,                       -- NULL for admin
    full_name TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    last_login DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (ruang_id) REFERENCES ruang(id),
    UNIQUE(tenant_id, username)
);

CREATE INDEX idx_users_tenant ON users(tenant_id);
```

### 2.4 `ruang`

Examination rooms — scoped per tenant.

```sql
CREATE TABLE ruang (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    nama_ruang TEXT NOT NULL,
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    UNIQUE(tenant_id, nama_ruang),
    UNIQUE(tenant_id, username)
);

CREATE INDEX idx_ruang_tenant ON ruang(tenant_id);
```

### 2.4 `kelas`

Classes.

```sql
CREATE TABLE kelas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nama_kelas TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
```

### 2.5 `mapel`

Subjects.

```sql
CREATE TABLE mapel (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nama_mapel TEXT UNIQUE NOT NULL,
    kode_mapel TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
```

### 2.6 `kelas_mapel`

Many-to-many relationship between classes and subjects.

```sql
CREATE TABLE kelas_mapel (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kelas_id INTEGER NOT NULL,
    mapel_id INTEGER NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (kelas_id) REFERENCES kelas(id),
    FOREIGN KEY (mapel_id) REFERENCES mapel(id),
    UNIQUE(kelas_id, mapel_id)
);
```

### 2.8 `peserta`

Students / Exam participants — scoped per tenant.

```sql
CREATE TABLE peserta (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    no_id TEXT NOT NULL,
    password TEXT NOT NULL,                 -- Plain or hashed (decided later)
    nama_peserta TEXT NOT NULL,
    kelas_id INTEGER NOT NULL,
    jenis_kelamin TEXT CHECK(jenis_kelamin IN ('L', 'P')),
    ruang_id INTEGER NOT NULL,
    foto TEXT,                              -- Path to photo
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (kelas_id) REFERENCES kelas(id),
    FOREIGN KEY (ruang_id) REFERENCES ruang(id),
    UNIQUE(tenant_id, no_id)
);

CREATE INDEX idx_peserta_tenant ON peserta(tenant_id);
CREATE INDEX idx_peserta_no_id ON peserta(tenant_id, no_id);
CREATE INDEX idx_peserta_ruang ON peserta(ruang_id);
```

### 2.9 `hasil_tes`

Main exam results table — scoped per tenant.

```sql
CREATE TABLE hasil_tes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    peserta_id INTEGER NOT NULL,
    mapel_id INTEGER NOT NULL,
    skor REAL,
    skor_maks REAL,
    kkm REAL,
    durasi_kerja INTEGER,                   -- in seconds
    waktu_mulai DATETIME,
    waktu_selesai DATETIME,
    status TEXT DEFAULT 'in_progress' CHECK(status IN ('in_progress', 'submitted', 'invalid')),
    validasi TEXT NOT NULL,                 -- tenant_id + no_id + mapel_id
    detail_xml TEXT,                        -- Raw XML from iSpring (dr)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (peserta_id) REFERENCES peserta(id),
    FOREIGN KEY (mapel_id) REFERENCES mapel(id),
    UNIQUE(tenant_id, validasi)
);

CREATE INDEX idx_hasil_tenant ON hasil_tes(tenant_id);
CREATE INDEX idx_hasil_validasi ON hasil_tes(tenant_id, validasi);
CREATE INDEX idx_hasil_peserta_mapel ON hasil_tes(peserta_id, mapel_id);
```

### 2.9 `hasil_tes_detail`

Parsed individual question results (from iSpring XML).

```sql
CREATE TABLE hasil_tes_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hasil_tes_id INTEGER NOT NULL,
    question_id TEXT NOT NULL,
    question_text TEXT,
    question_type TEXT,
    status TEXT,                            -- correct / incorrect / partial
    awarded_points REAL,
    max_points REAL,
    user_answer TEXT,
    correct_answer TEXT,
    attempts_used INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (hasil_tes_id) REFERENCES hasil_tes(id)
);

CREATE INDEX idx_detail_hasil ON hasil_tes_detail(hasil_tes_id);
```

### 2.11 `cek_login`

Tracks currently logged-in students (real-time monitoring) — scoped per tenant.

```sql
CREATE TABLE cek_login (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    peserta_id INTEGER NOT NULL,
    mapel_id INTEGER NOT NULL,
    login_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_activity DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (peserta_id) REFERENCES peserta(id),
    FOREIGN KEY (mapel_id) REFERENCES mapel(id),
    UNIQUE(tenant_id, peserta_id, mapel_id)
);

CREATE INDEX idx_login_tenant ON cek_login(tenant_id);
CREATE INDEX idx_login_peserta ON cek_login(peserta_id);
```

---

## 3. SUPPORTING TABLES

### 3.1 `migrations`

Tracks applied database migrations.

```sql
CREATE TABLE migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version TEXT UNIQUE NOT NULL,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 3.2 `activity_logs`

Audit trail for important actions.

```sql
CREATE TABLE activity_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    action TEXT NOT NULL,
    entity_type TEXT,
    entity_id INTEGER,
    details TEXT,
    ip_address TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## 4. INDEXING STRATEGY

- Primary keys are auto-indexed
- `no_id`, `validasi`, and `(peserta_id, mapel_id)` are heavily indexed
- Foreign keys have dedicated indexes for join performance
- Composite indexes used where multi-column queries are expected

---

## 5. DATA INTEGRITY RULES

- `validasi` column in `hasil_tes` enforces one result per student per subject
- Soft deletes used on master data (`peserta`, `kelas`, `mapel`, `ruang`)
- Check constraints on role and gender fields
- Foreign key constraints enforced at application level (SQLite foreign keys are optional)

---

## 6. MULTI-TENANT DATA ISOLATION RULES (CRITICAL)

1. **Every table** (except `tenants` and `migrations`) **must** have `tenant_id`.
2. **Every query** in the repository layer **must** filter by `tenant_id`.
3. **No query** is allowed to access data without tenant context.
4. `validasi` column di `hasil_tes` harus unik per tenant (`tenant_id + no_id + mapel_id`).
5. File soal iSpring disimpan di `data/soal/{tenant_slug}/` untuk isolasi fisik.

## 7. BACKUP & RECOVERY

- Recommended backup strategy: Copy `cbt_aether.db` + WAL file during low-traffic periods
- Point-in-time recovery possible via WAL
- Export functionality available for results (Excel/PDF)
- Backup otomatis per tenant dapat ditambahkan di masa depan

---

**This schema is designed for simplicity, performance, long-term maintainability, and strict multi-tenant isolation.**

*Next: UI Component Library Specification*
