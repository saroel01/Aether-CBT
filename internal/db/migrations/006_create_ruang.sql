-- +goose Up
CREATE TABLE IF NOT EXISTS ruang (
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

CREATE INDEX IF NOT EXISTS idx_ruang_tenant ON ruang(tenant_id);
