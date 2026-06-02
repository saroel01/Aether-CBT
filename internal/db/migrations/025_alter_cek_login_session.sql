-- Tautkan sesi aktif (cek_login) ke exam_session, tambahkan penguncian server-side
-- dan token konten untuk otorisasi penyajian paket iSpring.
-- Idempoten: error "duplicate column name" ditelan RunMigrations pada rerun.
ALTER TABLE cek_login ADD COLUMN session_id INTEGER;
ALTER TABLE cek_login ADD COLUMN locked INTEGER NOT NULL DEFAULT 0;
ALTER TABLE cek_login ADD COLUMN content_token TEXT;

-- Indeks unik sesi-berbasis (menggantikan basis mapel di masa depan).
-- Baris lama dengan session_id NULL diperlakukan distinct oleh SQLite sehingga
-- tidak mengganggu alur lama selama transisi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_cek_login_unique_session
    ON cek_login(tenant_id, peserta_id, session_id);

CREATE INDEX IF NOT EXISTS idx_cek_login_content_token ON cek_login(content_token);

-- CATATAN PENGURUTAN MIGRASI (penting):
-- Indeks unik lama berbasis mapel `idx_cek_login_unique_exam_session`
-- (tenant_id, peserta_id, mapel_id) dari migrasi 017 SENGAJA TIDAK di-drop di sini.
-- Runtime `StartExamSession` saat ini masih memakai ON CONFLICT berbasis mapel,
-- sehingga men-drop indeks itu sekarang akan merusak alur "mulai ujian".
-- Drop indeks lama dilakukan pada migrasi terpisah yang dibuat bersamaan dengan
-- konversi `StartExamSession` ke basis sesi (task 7.3), agar setiap state migrasi
-- tetap konsisten dan tidak meninggalkan alur yang rusak.
