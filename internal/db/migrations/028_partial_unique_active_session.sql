-- One active (unsubmitted, non-legacy) session per peserta per tenant (review H4, Task 14).
-- Legacy rows (session_id IS NULL) are exempt so the transition path keeps working.
-- Once a result is submitted, the cek_login row is deleted (processor.go), so this index
-- naturally allows a fresh session to start after submission.
CREATE UNIQUE INDEX IF NOT EXISTS idx_cek_login_one_active_session
ON cek_login (tenant_id, peserta_id)
WHERE session_id IS NOT NULL;
