-- Definisi ujian: kombinasi tetap mapel + tingkat + paket soal + durasi + KKM + pengacakan.
-- Dapat dijalankan berkali-kali melalui exam_session (gelombang).
CREATE TABLE IF NOT EXISTS exam (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    mapel_id INTEGER NOT NULL,
    tingkat TEXT,
    soal_package_id INTEGER,             -- boleh NULL saat draft
    durasi_menit INTEGER NOT NULL DEFAULT 90,
    kkm REAL NOT NULL DEFAULT 0,
    shuffle_questions BOOLEAN NOT NULL DEFAULT FALSE,
    shuffle_answers BOOLEAN NOT NULL DEFAULT FALSE,
    nama TEXT,                           -- label tampil opsional (mis. "UAS Kimia XII")
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (mapel_id) REFERENCES mapel(id),
    FOREIGN KEY (soal_package_id) REFERENCES soal_package(id)
);

CREATE INDEX IF NOT EXISTS idx_exam_tenant ON exam(tenant_id);
CREATE INDEX IF NOT EXISTS idx_exam_mapel ON exam(tenant_id, mapel_id);
