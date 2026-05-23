-- +goose Up
CREATE TABLE IF NOT EXISTS hasil_tes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    peserta_id INTEGER NOT NULL,
    mapel_id INTEGER,
    skor REAL,
    skor_maks REAL,
    kkm REAL,
    durasi_kerja INTEGER,
    waktu_mulai DATETIME,
    waktu_selesai DATETIME,
    status TEXT DEFAULT 'submitted' CHECK(status IN ('in_progress', 'submitted', 'invalid')),
    validasi TEXT,
    detail_xml TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (peserta_id) REFERENCES peserta(id),
    FOREIGN KEY (mapel_id) REFERENCES mapel(id)
);

CREATE INDEX IF NOT EXISTS idx_hasil_tenant ON hasil_tes(tenant_id);
CREATE INDEX IF NOT EXISTS idx_hasil_peserta ON hasil_tes(peserta_id);
