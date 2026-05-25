-- Alter mapel to support customizable time limit per subject (durasi_menit)
ALTER TABLE mapel ADD COLUMN durasi_menit INTEGER DEFAULT 90;
