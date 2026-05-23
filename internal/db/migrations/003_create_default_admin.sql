-- +goose Up
-- Create default admin user for the default tenant (password: admin123)
INSERT OR IGNORE INTO users (tenant_id, username, password_hash, role, full_name, is_active)
VALUES (1, 'admin', '$2a$14$ZWg8M9q80U7P9MaoOFunseFWwQFM2nQsamPDBneEtxrUkIMdpwuMm', 'admin', 'System Administrator', TRUE);
