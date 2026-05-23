-- Alter cek_login to support subjects and anti-cheat tracking
-- Since RunMigrations runs this and ALTER TABLE throws error if columns exist, we do it safely using standard column additions
-- In SQLite, we can just run them and handle gracefully or use schema modifications, but since migrations run on start, we use standard additions:

ALTER TABLE cek_login ADD COLUMN mapel_id INTEGER;
ALTER TABLE cek_login ADD COLUMN tab_switch_count INTEGER DEFAULT 0;
