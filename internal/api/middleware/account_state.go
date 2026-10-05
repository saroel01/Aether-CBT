package middleware

import (
	"context"
	"errors"

	"github.com/saroel01/aether-cbt/internal/db"
)

var errUnknownRole = errors.New("unknown role")

// accountTokenVersion returns the stored token_version of the live account behind a
// JWT. A deactivated or soft-deleted account yields sql.ErrNoRows (M5). One primary
// key lookup per request; no cache, so revocation takes effect immediately.
func accountTokenVersion(ctx context.Context, role string, tenantID, userID int) (int, error) {
	var q string
	switch role {
	case "admin", "superadmin":
		q = `SELECT token_version FROM users WHERE id = ? AND tenant_id = ? AND is_active = TRUE AND deleted_at IS NULL`
	case "supervisor":
		q = `SELECT token_version FROM ruang WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`
	case "student":
		q = `SELECT token_version FROM peserta WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`
	default:
		return 0, errUnknownRole
	}
	var tv int
	err := db.DB.QueryRowContext(ctx, q, userID, tenantID).Scan(&tv)
	return tv, err
}
