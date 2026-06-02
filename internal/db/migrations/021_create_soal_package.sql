-- Metadata paket soal hasil ekspor iSpring QuizMaker HTML5.
-- Berkas paket disimpan di data/soal/{tenant_slug}/{package_uuid}/.
CREATE TABLE IF NOT EXISTS soal_package (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    nama TEXT NOT NULL,
    package_uuid TEXT NOT NULL,          -- nama folder di data/soal/{slug}/{uuid}
    entry_path TEXT NOT NULL DEFAULT 'index.html',
    ispring_version TEXT,                -- best-effort dari komentar header; NULL bila tak terdeteksi
    total_size INTEGER NOT NULL DEFAULT 0,
    checksum TEXT,                       -- sha256 arsip terunggah (audit/dedup)
    uploaded_by INTEGER,                 -- users.id
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    UNIQUE(tenant_id, package_uuid)
);

CREATE INDEX IF NOT EXISTS idx_soal_package_tenant ON soal_package(tenant_id);
