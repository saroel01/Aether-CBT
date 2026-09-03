-- Gate 0 data repair for codebase-bug-sweep clause 2.2.
--
-- peserta.kelas_id and peserta.ruang_id are FOREIGN KEYs to kelas(id) / ruang(id)
-- (migration 009). Before this sweep, every write path expressed "not assigned" as the
-- integer 0, and no parent row has id 0, so each such row is a referential-integrity
-- violation. It stayed invisible because the DSN in internal/db/sqlite.go never actually
-- enabled foreign_keys (clause 1.1); enabling it is Gate 1, so the data has to be clean
-- first.
--
-- Idempotent: after the first run no row matches kelas_id = 0 / ruang_id = 0, so a re-run
-- updates zero rows. FK-safe: NULL always satisfies a referential constraint, so this stays
-- correct once Gate 1 has foreign_keys enabled on the very same connection. A row whose old
-- value already violates the constraint is not re-checked — SQLite validates the NEW value
-- of an UPDATE, not the old one.
--
-- Relaxing the NOT NULL that used to sit on both columns is a table rebuild, which needs
-- transaction and pragma control that a plain statement runner cannot provide; it is handled
-- by the peserta FK repair step in internal/db/migrate.go, which runs before this file.
UPDATE peserta SET kelas_id = NULL WHERE kelas_id = 0;
UPDATE peserta SET ruang_id = NULL WHERE ruang_id = 0;
