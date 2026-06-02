-- Sesi ujian (gelombang): instans terjadwal dari sebuah exam dengan jendela waktu
-- (mulai-selesai) dan token unik per-sesi.
CREATE TABLE IF NOT EXISTS exam_session (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    exam_id INTEGER NOT NULL,
    nama TEXT,                           -- label gelombang (mis. "Sesi 1")
    waktu_mulai DATETIME NOT NULL,
    waktu_selesai DATETIME NOT NULL,
    token TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK(status IN ('draft','terjadwal','aktif','selesai','dibatalkan')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (exam_id) REFERENCES exam(id)
);

CREATE INDEX IF NOT EXISTS idx_exam_session_tenant ON exam_session(tenant_id);
CREATE INDEX IF NOT EXISTS idx_exam_session_exam ON exam_session(exam_id);
-- Keunikan token diberlakukan di lapisan aplikasi terhadap sesi yang jendela waktunya
-- tumpang tindih (token sesi selesai boleh dipakai ulang). Indeks bantu pencarian token:
CREATE INDEX IF NOT EXISTS idx_exam_session_token ON exam_session(tenant_id, token);
