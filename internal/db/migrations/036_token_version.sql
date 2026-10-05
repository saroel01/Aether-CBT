-- M5: per-account JWT revocation counter. Tokens carry claim "tv"; AuthMiddleware
-- rejects a token whose tv differs from the row (bumped on password change).
ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ruang ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE peserta ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;
