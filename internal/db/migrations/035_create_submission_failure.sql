-- Audit H1 (D7): submissions dead-lettered by the filesystem queue (permanent error or
-- retries exhausted) are recorded here so admin/pengawas can see them and reset the
-- student's session instead of the result silently vanishing into data/queue/failed/.
CREATE TABLE IF NOT EXISTS submission_failure (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    no_id TEXT NOT NULL,
    validasi TEXT NOT NULL DEFAULT '',
    attempt_token TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    submitted_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_submission_failure_tenant ON submission_failure(tenant_id);
