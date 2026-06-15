// Package migratepasswords provides a one-shot, idempotent migration that rehashes
// any legacy plaintext peserta.password values to bcrypt. It is invoked by
// cmd/migratepasswords before enabling strict (no-plaintext-fallback) auth.
package migratepasswords

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/saroel01/aether-cbt/internal/utils"
)

// Stats reports how many rows were rehashed, skipped (already hashed), or empty.
type Stats struct {
	Hashed  int
	Skipped int
	Empty   int
}

// isBcryptHash returns true when stored already looks like a bcrypt hash.
func isBcryptHash(stored string) bool {
	return strings.HasPrefix(stored, "$2a$") ||
		strings.HasPrefix(stored, "$2b$") ||
		strings.HasPrefix(stored, "$2y$")
}

// Migrate scans every peserta.password row. Plaintext values are rehashed to bcrypt
// (one UPDATE per row, so a large table is not locked for the whole scan). Rows that
// are already bcrypt hashes are skipped, making the migration safe to run repeatedly.
// Empty passwords are counted but left untouched. Returns aggregate stats.
//
// The full rowset is read into memory and the iterator closed before any UPDATE, so a
// single-connection pool (or the WAL reader/writer split) cannot deadlock between the
// open SELECT cursor and the writes.
func Migrate(db *sql.DB) (Stats, error) {
	var stats Stats

	rows, err := db.Query(`SELECT id, password FROM peserta WHERE password IS NOT NULL`)
	if err != nil {
		return stats, fmt.Errorf("migratepasswords: query peserta: %w", err)
	}

	type pesertaRow struct {
		id       int
		password string
	}
	var toRehash []pesertaRow
	for rows.Next() {
		var r pesertaRow
		if err := rows.Scan(&r.id, &r.password); err != nil {
			rows.Close()
			return stats, fmt.Errorf("migratepasswords: scan peserta: %w", err)
		}
		switch {
		case r.password == "":
			stats.Empty++
		case isBcryptHash(r.password):
			stats.Skipped++
		default:
			toRehash = append(toRehash, r)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return stats, fmt.Errorf("migratepasswords: iterate rows: %w", err)
	}
	rows.Close() // release the read cursor before writing

	// Rehash the plaintext values one at a time. Each UPDATE is its own implicit tx so
	// the write lock is short — a live server can keep serving during the migration.
	for _, r := range toRehash {
		hash, err := utils.HashPassword(r.password)
		if err != nil {
			return stats, fmt.Errorf("migratepasswords: hash peserta %d: %w", r.id, err)
		}
		if _, err := db.Exec(`UPDATE peserta SET password = ? WHERE id = ?`, hash, r.id); err != nil {
			return stats, fmt.Errorf("migratepasswords: update peserta %d: %w", r.id, err)
		}
		stats.Hashed++
	}
	return stats, nil
}
