-- Alter cek_login to support student progress tracking
ALTER TABLE cek_login ADD COLUMN answered_count INTEGER DEFAULT 0;
ALTER TABLE cek_login ADD COLUMN total_questions INTEGER DEFAULT 0;
