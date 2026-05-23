-- +goose Up
-- Settings per tenant (exam token, title, etc.)
CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_title TEXT DEFAULT 'Ujian Sekolah',
    proctor_name TEXT,
    footer_text TEXT,
    token TEXT NOT NULL DEFAULT 'ujian2026',
    token_expiry DATETIME,
    is_exam_active BOOLEAN DEFAULT TRUE,
    data_soal_path TEXT DEFAULT 'data/soal',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    UNIQUE(tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_settings_tenant ON settings(tenant_id);

-- Seed default settings for tenant 1
INSERT OR IGNORE INTO settings (tenant_id, exam_title, token, is_exam_active)
VALUES (1, 'Ujian Akhir Semester - Default School', 'ujian2026', TRUE);
