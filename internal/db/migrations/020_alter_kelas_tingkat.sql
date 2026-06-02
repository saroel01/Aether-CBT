-- Tambah tingkatan (jenjang kelas: X/XI/XII) pada tabel kelas.
-- Idempoten: error "duplicate column name" ditelan oleh RunMigrations pada rerun.
ALTER TABLE kelas ADD COLUMN tingkat TEXT;

CREATE INDEX IF NOT EXISTS idx_kelas_tingkat ON kelas(tenant_id, tingkat);
