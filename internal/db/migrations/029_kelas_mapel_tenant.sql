-- Denormalize tenant_id onto kelas_mapel so a cross-tenant link (kelas from tenant A,
-- mapel from tenant B) is structurally rejected and the table can be filtered without a join
-- (review data finding #13 / handler H16, Task 17). Backfill from the parent kelas row.
ALTER TABLE kelas_mapel ADD COLUMN tenant_id INTEGER REFERENCES tenants(id);
UPDATE kelas_mapel
SET tenant_id = (SELECT tenant_id FROM kelas WHERE kelas.id = kelas_mapel.kelas_id);
CREATE INDEX IF NOT EXISTS idx_kelas_mapel_tenant ON kelas_mapel(tenant_id);
