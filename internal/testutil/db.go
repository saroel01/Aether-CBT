// Package testutil provides shared test fixtures (per-test migrated SQLite databases)
// for repository and integration tests. It is imported only by _test.go files, so it
// is never compiled into production binaries.
package testutil

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/saroel01/aether-cbt/internal/db"
	_ "modernc.org/sqlite"
)

// NewMigratedDB opens a fresh, file-backed SQLite database in a per-test temp directory
// and applies every migration against it, returning the database together with a
// cleanup that closes it. The package-global db.DB is never touched, so tests that use
// this helper can run in parallel without clobbering process-wide state (Requirement 16.7).
//
// The database is opened through db.DSN — the same connection string production uses — so
// fixtures run with WAL, a 5s busy timeout and, above all, FOREIGN KEY enforcement actually
// on (codebase-bug-sweep clause 2.1). This is deliberate and load-bearing: a fixture with its
// own DSN would give the suite different pragma semantics from the application, which is how a
// project-wide absence of FK enforcement stayed invisible through a fully green test suite.
// db.VerifyPragmas is asserted here for the same reason it is asserted in db.Connect.
func NewMigratedDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "aether-test.db")
	database, err := sql.Open("sqlite", db.DSN(databasePath))
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := db.VerifyPragmas(database); err != nil {
		_ = database.Close()
		t.Fatalf("test fixture database does not have the required pragmas: %v", err)
	}
	if err := db.RunMigrations(database, migrationsDir(t)); err != nil {
		_ = database.Close()
		t.Fatalf("run migrations: %v", err)
	}
	return database, func() { _ = database.Close() }
}

// migrationsDir resolves the migrations directory from this source file's location
// (runtime.Caller), so it is correct regardless of which package's test invoked it -
// the package working directory is not always a sibling of internal/db.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller: cannot locate testutil source")
	}
	// file = <repo>/internal/testutil/db.go -> repo root is three dirs up.
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	return filepath.Join(repoRoot, "internal", "db", "migrations")
}
