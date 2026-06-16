-- Attribute each hasil_tes row to the exam_session wave it belongs to (review data ER #2,
-- Task 37). Nullable so legacy rows and the legacy mapel-based path still insert fine; the
-- session-based processor path sets it. No hard FK (exam_session rows may be soft-deleted),
-- just an indexed reference for per-wave attribution.
ALTER TABLE hasil_tes ADD COLUMN exam_session_id INTEGER REFERENCES exam_session(id);
CREATE INDEX IF NOT EXISTS idx_hasil_tes_session
ON hasil_tes(tenant_id, exam_session_id)
WHERE exam_session_id IS NOT NULL;
