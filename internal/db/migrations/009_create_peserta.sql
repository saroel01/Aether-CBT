-- +goose Up
CREATE TABLE IF NOT EXISTS peserta (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    no_id TEXT NOT NULL,
    password TEXT NOT NULL,                 -- plaintext for simple student mass import (can be changed later)
    nama_peserta TEXT NOT NULL,
    -- kelas_id / ruang_id are nullable on purpose: a student can be enrolled before a class
    -- or room has been assigned. They used to be NOT NULL, which forced every write path to
    -- express "not assigned" as the integer 0 — a value that violates the FOREIGN KEY below
    -- because no kelas/ruang row has id 0 (codebase-bug-sweep clause 2.2). Existing databases
    -- are converted to this shape by the peserta FK repair in internal/db/migrate.go, and the
    -- leftover sentinel rows are nulled by migration 032.
    kelas_id INTEGER,
    jenis_kelamin TEXT CHECK(jenis_kelamin IN ('L', 'P')),
    ruang_id INTEGER,
    foto TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (kelas_id) REFERENCES kelas(id),
    FOREIGN KEY (ruang_id) REFERENCES ruang(id),
    UNIQUE(tenant_id, no_id)
);

CREATE INDEX IF NOT EXISTS idx_peserta_tenant ON peserta(tenant_id);
CREATE INDEX IF NOT EXISTS idx_peserta_no_id ON peserta(tenant_id, no_id);
CREATE INDEX IF NOT EXISTS idx_peserta_ruang ON peserta(ruang_id);
