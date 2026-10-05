package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forgetAppliedMigrations clears schema_migrations, simulating a pre-versioning
// database so the next RunMigrations re-executes every file.
func forgetAppliedMigrations(t *testing.T, database *sql.DB) {
	t.Helper()
	if _, err := database.Exec(`DELETE FROM schema_migrations`); err != nil {
		t.Fatalf("forget applied migrations: %v", err)
	}
}

func countSQLFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			n++
		}
	}
	return n
}

func countApplied(t *testing.T, database *sql.DB) int {
	t.Helper()
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRunMigrationsRecordsEveryFileOnce(t *testing.T) {
	testDB, cleanup := newTestDB(t)
	defer cleanup()
	dir := migrationsDir(t)

	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("first run: %v", err)
	}
	want := countSQLFiles(t, dir)
	if got := countApplied(t, testDB); got != want {
		t.Fatalf("schema_migrations rows = %d, want %d", got, want)
	}
	// 001 seeds tenant 1 with INSERT OR IGNORE; if 001 re-ran the row would come back.
	if _, err := testDB.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(`DELETE FROM tenants WHERE id = 1`); err != nil {
		t.Fatalf("delete tenant 1: %v", err)
	}
	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := countApplied(t, testDB); got != want {
		t.Fatalf("schema_migrations rows after rerun = %d, want %d", got, want)
	}
	var n int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM tenants WHERE id = 1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("tenant 1 came back: migration 001 was re-executed")
	}
}

func TestRunMigrationsBootstrapsLegacyDatabase(t *testing.T) {
	testDB, cleanup := newTestDB(t)
	defer cleanup()
	dir := migrationsDir(t)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") || e.Name()[:3] > "033" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, stmt := range splitSQLStatements(string(content)) {
			if _, err := testDB.Exec(stmt); err != nil &&
				!strings.Contains(err.Error(), "duplicate column name") &&
				!strings.Contains(err.Error(), "already exists") {
				t.Fatalf("legacy %s: %v", e.Name(), err)
			}
		}
	}
	if tableHasColumn(t, testDB, "soal_package", "answer_key") {
		t.Fatal("precondition: legacy DB must not have answer_key yet")
	}

	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("bootstrap run: %v", err)
	}
	if !tableHasColumn(t, testDB, "soal_package", "answer_key") {
		t.Error("034 not applied on legacy DB")
	}
	if !objectExists(t, testDB, "table", "submission_failure") {
		t.Error("035 not applied on legacy DB")
	}
	for _, tbl := range []string{"users", "ruang", "peserta"} {
		if !tableHasColumn(t, testDB, tbl, "token_version") {
			t.Errorf("036 not applied: %s.token_version missing", tbl)
		}
	}
	want := countSQLFiles(t, dir)
	if got := countApplied(t, testDB); got != want {
		t.Fatalf("schema_migrations rows = %d, want %d", got, want)
	}

	// Second run executes nothing: drop a table created by 035; it must stay dropped.
	if _, err := testDB.Exec(`DROP TABLE submission_failure`); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if objectExists(t, testDB, "table", "submission_failure") {
		t.Fatal("035 was re-executed on second run")
	}
}

func TestRunMigrationsRollsBackFailedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_ok.sql"), []byte("CREATE TABLE t1 (x INTEGER);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_bad.sql"), []byte("CREATE TABLE t2 (x INTEGER);\nINSERT INTO nope VALUES (1);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testDB, cleanup := newTestDB(t)
	defer cleanup()

	err := RunMigrations(testDB, dir)
	if err == nil || !strings.Contains(err.Error(), "002_bad.sql") {
		t.Fatalf("expected error naming 002_bad.sql, got %v", err)
	}
	if objectExists(t, testDB, "table", "t2") {
		t.Error("t2 exists: failed file was not rolled back")
	}
	var n int
	_ = testDB.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = '002_bad.sql'`).Scan(&n)
	if n != 0 {
		t.Error("002_bad.sql recorded as applied")
	}
	_ = testDB.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = '001_ok.sql'`).Scan(&n)
	if n != 1 {
		t.Error("001_ok.sql not recorded")
	}
}

func TestRunMigrationsMissingDirIsError(t *testing.T) {
	testDB, cleanup := newTestDB(t)
	defer cleanup()
	if err := RunMigrations(testDB, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected error for missing migrations dir")
	}
}

func TestRunMigrationsEmptyDirUsesEmbedded(t *testing.T) {
	testDB, cleanup := newTestDB(t)
	defer cleanup()
	if err := RunMigrations(testDB, ""); err != nil {
		t.Fatalf("embedded run: %v", err)
	}
	if got, want := countApplied(t, testDB), countSQLFiles(t, migrationsDir(t)); got != want {
		t.Fatalf("embedded applied %d files, want %d", got, want)
	}
}
