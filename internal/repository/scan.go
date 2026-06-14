package repository

import (
	"database/sql"
	"time"
)

// scanner is satisfied by both *sql.Row and *sql.Rows, so a single scan helper can
// serve point-lookups (QueryRow) and row iteration (Query) without duplication.
type scanner interface {
	Scan(dest ...any) error
}

// nullString converts a nullable SQL string column into a *string (nil when not valid).
func nullString(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

// nullInt converts a nullable SQL integer column into a *int (nil when not valid).
func nullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	i := int(n.Int64)
	return &i
}

// nullTime converts a nullable SQL timestamp into a *time.Time (nil when not valid).
func nullTime(n sql.NullTime) *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time
	return &t
}
