package submission

import (
	"database/sql"
	"errors"
)

// ErrGraceExceeded reports a submission that arrived after durasi + 5 minutes of grace.
// It is permanent: retrying can never bring the submission back inside the window.
var ErrGraceExceeded = errors.New("grace period exceeded")

// PermanentError marks a processing failure that no retry can fix (missing peserta,
// missing session, grace exceeded, invalid XML, rejected overwrite, score mismatch). The
// queue dead-letters it immediately instead of burning retries on it (audit H1, D7).
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent wraps err as a PermanentError. A nil err stays nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// permanentIfNoRows marks a lookup failure permanent only when the row is genuinely absent;
// any other SQL error (busy/locked database) stays transient and is retried.
func permanentIfNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return Permanent(err)
	}
	return err
}

// IsPermanent reports whether err (or anything it wraps) is a PermanentError or
// ErrGraceExceeded.
func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe) || errors.Is(err, ErrGraceExceeded)
}
