// Package testutil provides shared test fixtures (per-test migrated SQLite databases)
// for repository and integration tests. It is imported only by _test.go files, so it
// is never compiled into production binaries.
package testutil

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/saroel01/aether-cbt/internal/db"
	_ "modernc.org/sqlite"
)

// NewMigratedDB opens a fresh, file-backed SQLite database in a per-test temp directory
// and applies every migration against it, returning the database together with a
// cleanup that closes it. The package-global db.DB is never touched, so tests that use
// this helper can run in parallel without clobbering process-wide state (Requirement 16.7).
func NewMigratedDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "aether-test.db")
	database, err := sql.Open("sqlite", databasePath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.RunMigrations(database, migrationsDir(t)); err != nil {
		_ = database.Close()
		t.Fatalf("run migrations: %v", err)
	}
	return database, func() { _ = database.Close() }
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "db", "migrations"))
	if err != nil {
		t.Fatalf("resolve migrations dir: %v", err)
	}
	return abs
}
