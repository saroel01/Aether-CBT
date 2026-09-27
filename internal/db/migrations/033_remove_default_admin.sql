-- Remove default seed admin account if its password hash matches the known default seed hash:
-- 1. $2a$14$ZWg8M9q80U7P9MaoOFunseFWwQFM2nQsamPDBneEtxrUkIMdpwuMm (from 004_create_admin_user.sql)
-- 2. Legacy seed hash patterns (such as $2a$10$wTqS... for admin123)
-- Accounts with custom or rotated passwords remain untouched (P0-4).
DELETE FROM users
WHERE role = 'admin'
  AND username = 'admin'
  AND (
    password_hash = '$2a$14$ZWg8M9q80U7P9MaoOFunseFWwQFM2nQsamPDBneEtxrUkIMdpwuMm'
    OR password_hash LIKE '$2a$10$wTqS%'
  );
