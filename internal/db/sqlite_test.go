package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// Gate 1 of codebase-bug-sweep, clause 2.1 (bug condition C1).
//
// The DSN this project used for its whole life — "<path>?_journal_mode=WAL&_foreign_keys=on&
// _busy_timeout=5000" — is mattn/go-sqlite3 syntax. modernc.org/sqlite splits the DSN at the
// first '?' and, when the remainder is NOT prefixed with "file:", throws the query string away
// and opens the bare filename. Every parameter was therefore silently ignored: connections ran
// with journal_mode=delete, busy_timeout=0 and foreign_keys=0, so no FOREIGN KEY in any of the
// 30+ migrations was ever enforced.
//
// These tests convert that empirical finding into a permanent regression test. They fail on F
// (delete / 0 / 0) and pass on F' once Connect switches to the driver's own _pragma syntax.

// pragmaState is the observed value of the three pragmas clause 2.1 requires.
type pragmaState struct {
	journalMode string
	foreignKeys int
	busyTimeout int
}

// readPragmas reads the three pragmas that clause 2.1 governs from one pooled connection.
func readPragmas(t *testing.T, database *sql.DB) pragmaState {
	t.Helper()
	var got pragmaState
	if err := database.QueryRow(`PRAGMA journal_mode`).Scan(&got.journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if err := database.QueryRow(`PRAGMA foreign_keys`).Scan(&got.foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if err := database.QueryRow(`PRAGMA busy_timeout`).Scan(&got.busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	return got
}

// assertPragmasActive asserts the three required pragma values, reporting each mismatch
// individually so a failure names every pragma that is wrong, not just the first.
func assertPragmasActive(t *testing.T, got pragmaState) {
	t.Helper()
	if !strings.EqualFold(got.journalMode, "wal") {
		t.Errorf("journal_mode = %q, want %q (case-insensitive)", got.journalMode, "wal")
	}
	if got.foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1 (FOREIGN KEY enforcement must be on)", got.foreignKeys)
	}
	if got.busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", got.busyTimeout)
	}
}

// connectForTest calls Connect against the given path and restores the package-global DB
// afterwards, so the global is never left dangling for other tests in this package.
func connectForTest(t *testing.T, databasePath string) *sql.DB {
	t.Helper()
	previous := DB
	t.Cleanup(func() {
		if DB != nil {
			_ = DB.Close()
		}
		DB = previous
	})
	if err := Connect(databasePath, DefaultPoolConfig()); err != nil {
		t.Fatalf("Connect(%q): %v", databasePath, err)
	}
	return DB
}

// TestConnectActivatesConfiguredPragmas is the primary C1 assertion: the pragmas Connect
// claims to set must actually be in effect on connections handed out by the pool.
func TestConnectActivatesConfiguredPragmas(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "pragma.db")
	database := connectForTest(t, databasePath)

	assertPragmasActive(t, readPragmas(t, database))

	// foreign_keys and busy_timeout are per-connection while sql.DB is a pool, so read them
	// again on a second, concurrently-held connection: the guarantee has to come from the DSN
	// (which the driver applies to every new connection), not from one lucky connection.
	first, err := database.Conn(t.Context())
	if err != nil {
		t.Fatalf("acquire first connection: %v", err)
	}
	defer first.Close()
	second, err := database.Conn(t.Context())
	if err != nil {
		t.Fatalf("acquire second connection: %v", err)
	}
	defer second.Close()

	for i, conn := range []*sql.Conn{first, second} {
		var foreignKeys, busyTimeout int
		if err := conn.QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
			t.Fatalf("read foreign_keys on connection %d: %v", i, err)
		}
		if err := conn.QueryRowContext(t.Context(), `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
			t.Fatalf("read busy_timeout on connection %d: %v", i, err)
		}
		if foreignKeys != 1 {
			t.Errorf("connection %d: foreign_keys = %d, want 1", i, foreignKeys)
		}
		if busyTimeout != 5000 {
			t.Errorf("connection %d: busy_timeout = %d, want 5000", i, busyTimeout)
		}
	}
}

// TestConnectCreatesWALSidecar proves WAL is active on the database FILE, not merely reported by
// a pragma. WAL journalling creates a "-wal" companion next to the database on first write; its
// absence next to data/cbt_aether.db was the corroborating evidence that WAL had never once been
// active in this project despite the connection log claiming it since day one.
func TestConnectCreatesWALSidecar(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "sidecar.db")
	database := connectForTest(t, databasePath)

	if _, err := database.Exec(`CREATE TABLE probe (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("write to database: %v", err)
	}
	if _, err := os.Stat(databasePath + "-wal"); err != nil {
		t.Errorf("no %q sidecar after a write; WAL journalling is not active: %v", databasePath+"-wal", err)
	}
}

// TestConnectEnforcesForeignKeys proves the pragma is not merely reported as on but actually
// rejects a dangling reference — the behaviour the whole Gate 0/Gate 1 sequence exists for.
func TestConnectEnforcesForeignKeys(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "enforce.db")
	database := connectForTest(t, databasePath)

	for _, stmt := range []string{
		`CREATE TABLE parent (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id))`,
		`INSERT INTO parent (id) VALUES (1)`,
	} {
		if _, err := database.Exec(stmt); err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	if _, err := database.Exec(`INSERT INTO child (parent_id) VALUES (999)`); err == nil {
		t.Error("a dangling foreign key was accepted; foreign_keys enforcement is not active")
	}
	if _, err := database.Exec(`INSERT INTO child (parent_id) VALUES (1)`); err != nil {
		t.Errorf("a valid reference was rejected: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO child (parent_id) VALUES (NULL)`); err != nil {
		t.Errorf("a NULL reference was rejected; NULL always satisfies a FOREIGN KEY: %v", err)
	}
}

// TestConnectHandlesURISpecialCharactersInPath covers the deliberate-escaping requirement of
// task 3.2. Once the DSN carries a "file:" prefix, the driver hands the whole string to
// SQLite's URI parser, which ends the filename at the first '?' or '#' and percent-decodes the
// rest. A path containing those characters therefore has to be escaped, not concatenated.
//
// The assertion is two-sided: the pragmas must still be active (the query string was parsed)
// AND the database file must exist at exactly the requested path (the filename was not
// truncated or mangled).
func TestConnectHandlesURISpecialCharactersInPath(t *testing.T) {
	cases := []struct {
		name string
		dir  string
	}{
		{"hash", "ujian#2026"},
		{"percent", "ujian%2026"},
		{"ampersand", "ujian&sesi"},
		{"space", "ujian sesi 1"},
		{"plus", "ujian+sesi"},
		{"equals", "ujian=sesi"},
		// '?' is not a legal filename character on Windows, so it is only exercised where the
		// filesystem permits it. The escaping it requires is identical to '#'.
		{"question", "ujian?sesi"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "question" && runtime.GOOS == "windows" {
				t.Skip("'?' is not a legal filename character on Windows")
			}
			dir := filepath.Join(t.TempDir(), tc.dir)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Skipf("filesystem rejects directory %q: %v", tc.dir, err)
			}
			databasePath := filepath.Join(dir, "aether.db")

			database := connectForTest(t, databasePath)
			assertPragmasActive(t, readPragmas(t, database))

			if _, err := database.Exec(`CREATE TABLE probe (id INTEGER PRIMARY KEY)`); err != nil {
				t.Fatalf("write to database at %q: %v", databasePath, err)
			}
			if _, err := os.Stat(databasePath); err != nil {
				t.Errorf("database was not created at the requested path %q: %v", databasePath, err)
			}
		})
	}
}

// TestVerifyPragmasErrorNamesPragmaExpectedAndActual asserts the diagnostic quality clause 2.1
// asks for: on mismatch the error must name the pragma, the expected value and the actual
// value. Without those three, a future DSN typo would produce "verification failed" and send
// the reader back to guessing — the same silence clause 1.1 was made of.
//
// The mismatching connection is opened with the project's historical mattn/go-sqlite3-style
// DSN, which pins the empirical finding behind this gate: that syntax is silently ignored by
// modernc.org/sqlite, yielding delete / 0 / 0.
func TestVerifyPragmasErrorNamesPragmaExpectedAndActual(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "legacy-dsn.db")
	legacy, err := sql.Open("sqlite", databasePath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open legacy-DSN database: %v", err)
	}
	defer legacy.Close()
	if err := legacy.Ping(); err != nil {
		t.Fatalf("ping legacy-DSN database: %v", err)
	}

	// Precondition: the legacy DSN really is ignored. If a driver upgrade ever starts honouring
	// it, this test would otherwise pass for the wrong reason.
	got := readPragmas(t, legacy)
	if strings.EqualFold(got.journalMode, "wal") || got.foreignKeys != 0 || got.busyTimeout != 0 {
		t.Fatalf("precondition: expected the mattn-style DSN to be ignored (delete/0/0), got %q/%d/%d",
			got.journalMode, got.foreignKeys, got.busyTimeout)
	}

	verifyErr := VerifyPragmas(legacy)
	if verifyErr == nil {
		t.Fatal("VerifyPragmas accepted a connection with none of the required pragmas active")
	}

	message := verifyErr.Error()
	for _, want := range []string{
		// the pragma names ...
		"journal_mode", "foreign_keys", "busy_timeout",
		// ... their expected values ...
		`"wal"`, `"1"`, `"5000"`,
		// ... and the values actually observed.
		`"delete"`, `"0"`,
	} {
		if !strings.Contains(message, want) {
			t.Errorf("verification error does not mention %s; got: %s", want, message)
		}
	}
}

// TestVerifyPragmasAcceptsAConnectionOpenedThroughDSN closes the loop between the DSN builder
// and the verifier: anything DSN produces must satisfy VerifyPragmas, or Connect could reject
// its own connection string.
func TestVerifyPragmasAcceptsAConnectionOpenedThroughDSN(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "roundtrip.db")
	database, err := sql.Open("sqlite", DSN(databasePath))
	if err != nil {
		t.Fatalf("open via DSN: %v", err)
	}
	defer database.Close()

	if err := VerifyPragmas(database); err != nil {
		t.Errorf("VerifyPragmas rejected a connection opened with DSN(): %v", err)
	}
}
