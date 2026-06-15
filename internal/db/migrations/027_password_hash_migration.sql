-- Marker: password hashing migration is performed by cmd/migratepasswords.
-- This file exists so the schema doc reflects that peserta.password now MUST
-- contain a bcrypt hash ($2a$/$2b$/$2y$ prefix). No DDL: the column type is
-- unchanged (TEXT), only its content contract changes.
-- Run cmd/migratepasswords once per install before enabling strict auth.
SELECT 1; -- no-op so the migration runner records the file as applied
