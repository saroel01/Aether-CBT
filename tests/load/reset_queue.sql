-- Cleanup stale queue/test data from previous PoC runs.
DELETE FROM submission_queue;
DELETE FROM failed_submissions;
DELETE FROM hasil_tes_detail WHERE hasil_tes_id IN (
    SELECT id FROM hasil_tes WHERE peserta_id IN (
        SELECT id FROM peserta WHERE no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
            OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%'
    )
);
DELETE FROM hasil_tes WHERE peserta_id IN (
    SELECT id FROM peserta WHERE no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
        OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%'
);
DELETE FROM cek_login WHERE peserta_id IN (
    SELECT id FROM peserta WHERE no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
        OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%'
);
DELETE FROM peserta WHERE no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
    OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%';
PRAGMA wal_checkpoint(TRUNCATE);
