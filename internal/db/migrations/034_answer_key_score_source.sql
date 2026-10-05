-- Audit C1: server-side answer key extracted from the iSpring index.html at upload time,
-- plus the provenance of each stored score (server|mixed|client). Re-run on every boot;
-- the runner swallows "duplicate column", so these ALTERs are idempotent.
ALTER TABLE soal_package ADD COLUMN answer_key TEXT;
ALTER TABLE soal_package ADD COLUMN answer_key_status TEXT NOT NULL DEFAULT 'none';
ALTER TABLE hasil_tes ADD COLUMN score_source TEXT NOT NULL DEFAULT 'client';
