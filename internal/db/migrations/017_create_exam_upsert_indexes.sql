-- Ensure the upsert targets used by the exam session and iSpring result flows
-- are backed by real SQLite unique constraints.
--
-- The one-shot dedup DELETEs are guarded on the index's existence in sqlite_master so they
-- run ONLY on first application (when the index does not yet exist). On an install where
-- migration 017 already ran, the index is present, so NOT EXISTS is false and the DELETE
-- becomes a no-op — the dedup never re-runs on every startup (review data finding #4, Task 33).
DELETE FROM cek_login
WHERE id NOT IN (
    SELECT MAX(id)
    FROM cek_login
    WHERE mapel_id IS NOT NULL
    GROUP BY tenant_id, peserta_id, mapel_id
)
AND mapel_id IS NOT NULL
AND NOT EXISTS (
    SELECT 1 FROM sqlite_master
    WHERE type = 'index' AND name = 'idx_cek_login_unique_exam_session'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cek_login_unique_exam_session
ON cek_login(tenant_id, peserta_id, mapel_id);

DELETE FROM hasil_tes
WHERE validasi IS NOT NULL
AND id NOT IN (
    SELECT MAX(id)
    FROM hasil_tes
    WHERE validasi IS NOT NULL
    GROUP BY tenant_id, validasi
)
AND NOT EXISTS (
    SELECT 1 FROM sqlite_master
    WHERE type = 'index' AND name = 'idx_hasil_tes_unique_validasi'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_hasil_tes_unique_validasi
ON hasil_tes(tenant_id, validasi);
