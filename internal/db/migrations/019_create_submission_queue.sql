-- 019_create_submission_queue.sql
-- Queue untuk pemrosesan webhook iSpring secara serial (anti write-contention)

CREATE TABLE IF NOT EXISTS submission_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    no_id TEXT NOT NULL,
    score TEXT,
    max_score TEXT,
    detail_xml TEXT,
    attempt_token TEXT,
    validasi TEXT NOT NULL,
    retry_count INTEGER DEFAULT 0,
    last_error TEXT,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'failed')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    next_retry_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_submission_queue_status_retry
    ON submission_queue(status, next_retry_at);

CREATE INDEX IF NOT EXISTS idx_submission_queue_tenant_validasi
    ON submission_queue(tenant_id, validasi);

-- Tabel untuk job yang gagal permanen (dead letter)
CREATE TABLE IF NOT EXISTS failed_submissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    original_job_id INTEGER,
    tenant_id INTEGER NOT NULL,
    no_id TEXT NOT NULL,
    error_message TEXT,
    detail_xml TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_failed_submissions_tenant
    ON failed_submissions(tenant_id);
