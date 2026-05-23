-- +goose Up
CREATE TABLE IF NOT EXISTS peserta (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    no_id TEXT NOT NULL,
    password TEXT NOT NULL,                 -- plaintext for simple student mass import (can be changed later)
    nama_peserta TEXT NOT NULL,
    kelas_id INTEGER NOT NULL,
    jenis_kelamin TEXT CHECK(jenis_kelamin IN ('L', 'P')),
    ruang_id INTEGER NOT NULL,
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
