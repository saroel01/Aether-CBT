-- Ensure the upsert targets used by the exam session and iSpring result flows
-- are backed by real SQLite unique constraints.
DELETE FROM cek_login
WHERE id NOT IN (
    SELECT MAX(id)
    FROM cek_login
    WHERE mapel_id IS NOT NULL
    GROUP BY tenant_id, peserta_id, mapel_id
)
AND mapel_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_cek_login_unique_exam_session
ON cek_login(tenant_id, peserta_id, mapel_id);

DELETE FROM hasil_tes
WHERE validasi IS NOT NULL
AND id NOT IN (
    SELECT MAX(id)
    FROM hasil_tes
    WHERE validasi IS NOT NULL
    GROUP BY tenant_id, validasi
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_hasil_tes_unique_validasi
ON hasil_tes(tenant_id, validasi);
