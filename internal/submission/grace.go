package submission

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// gracePeriod is added on top of the exam duration before a submission is refused.
const gracePeriod = 5 * time.Minute

// rowQuerier is satisfied by *sql.DB, *sql.Tx and *sql.Conn.
type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// CheckGrace returns ErrGraceExceeded (wrapped) when `at` is later than login_time +
// durasi_menit + 5 minutes for the cek_login row owning attemptToken. Shared by the webhook
// (synchronous check before replying 200) and the processor so both apply the same rule
// (audit H1, D8). A missing cek_login row is returned as a wrapped sql.ErrNoRows.
func CheckGrace(ctx context.Context, q rowQuerier, tenantID int, attemptToken string, at time.Time) error {
	var loginTime time.Time
	if err := q.QueryRowContext(ctx,
		"SELECT login_time FROM cek_login WHERE tenant_id = ? AND attempt_token = ?",
		tenantID, attemptToken,
	).Scan(&loginTime); err != nil {
		return fmt.Errorf("grace check: lookup cek_login: %w", err)
	}

	// Duration lookup keeps the processor's historical behaviour: any failure falls back to
	// the 90-minute default rather than rejecting the submission.
	durasiMenit := 90
	_ = q.QueryRowContext(ctx, `
		SELECT COALESCE(ex.durasi_menit, m.durasi_menit, 90)
		FROM cek_login cl
		LEFT JOIN exam_session es ON es.id = cl.session_id AND es.tenant_id = cl.tenant_id
		LEFT JOIN exam ex ON ex.id = es.exam_id AND ex.tenant_id = es.tenant_id
		LEFT JOIN mapel m ON m.id = cl.mapel_id AND m.tenant_id = cl.tenant_id
		WHERE cl.tenant_id = ? AND cl.attempt_token = ?
	`, tenantID, attemptToken).Scan(&durasiMenit)

	maxAllowed := time.Duration(durasiMenit)*time.Minute + gracePeriod
	if at.UTC().Sub(loginTime.UTC()) > maxAllowed {
		return fmt.Errorf("%w (login %s, submitted %s, allowed %s)",
			ErrGraceExceeded, loginTime.UTC().Format(time.RFC3339), at.UTC().Format(time.RFC3339), maxAllowed)
	}
	return nil
}
