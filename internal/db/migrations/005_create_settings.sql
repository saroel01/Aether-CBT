-- +goose Up
-- Settings per tenant (exam token, title, etc.)
-- NOTE: no default token is seeded here. getSettingsForTenant auto-seeds a crypto-random
-- token on first access (Task 18), so the column has no DEFAULT either (the handler always
-- supplies one). A known constant default would be guessable (review H22).
CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_title TEXT DEFAULT 'Ujian Sekolah',
    proctor_name TEXT,
    footer_text TEXT,
    token TEXT NOT NULL,
    token_expiry DATETIME,
    is_exam_active BOOLEAN DEFAULT TRUE,
    data_soal_path TEXT DEFAULT 'data/soal',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    UNIQUE(tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_settings_tenant ON settings(tenant_id);
