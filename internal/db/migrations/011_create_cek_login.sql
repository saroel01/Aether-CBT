-- +goose Up
CREATE TABLE IF NOT EXISTS cek_login (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    peserta_id INTEGER NOT NULL,
    ruang_id INTEGER,
    login_time DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_activity DATETIME,
    ip_address TEXT,
    user_agent TEXT,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (peserta_id) REFERENCES peserta(id)
);

CREATE INDEX IF NOT EXISTS idx_cek_login_tenant ON cek_login(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cek_login_peserta ON cek_login(peserta_id);
