package db

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// PoolConfig holds the SQLite connection pool tuning parameters. It is defined
// here (rather than importing the config package) to keep the db package free of
// configuration dependencies and avoid import cycles.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DefaultPoolConfig returns conservative pool settings suitable for CLI tools and
// tests where explicit configuration is not provided.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
	}
}

// pragmaQuery is the driver-specific query string that carries the connection pragmas.
//
// This is modernc.org/sqlite syntax and it is not interchangeable with the mattn/go-sqlite3
// syntax this project used previously ("?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000").
// modernc.org/sqlite splits the DSN at the first '?' and, unless the DSN is prefixed with
// "file:", discards the query string entirely and opens the bare filename — which is why every
// pragma was silently ignored for the whole life of the project (codebase-bug-sweep clause 1.1).
// The "file:" prefix added by DSN is therefore load-bearing, not decorative.
const pragmaQuery = "_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)"

// requiredPragmas is the post-connection expectation matching pragmaQuery, and the single
// source of truth for verifyPragmas. journal_mode is compared case-insensitively because
// SQLite reports it in lower case while the DSN sets it in upper case.
var requiredPragmas = []struct {
	name     string
	expected string
	matches  func(actual string) bool
}{
	{"journal_mode", "wal", func(actual string) bool { return strings.EqualFold(actual, "wal") }},
	{"foreign_keys", "1", func(actual string) bool { return actual == "1" }},
	{"busy_timeout", "5000", func(actual string) bool { return actual == "5000" }},
}

// DSN builds the SQLite connection string for databasePath, with the connection pragmas
// expressed in the syntax modernc.org/sqlite actually understands.
//
// databasePath is percent-escaped rather than concatenated. Once the DSN carries the "file:"
// prefix, the driver hands the whole string to SQLite's URI parser, which ends the filename at
// the first '?' or '#' and percent-decodes what precedes it. A path containing those characters
// (or '%' itself) would otherwise be truncated or silently rewritten. Escaping keeps that
// correct by construction instead of by luck.
//
// Exported so that test fixtures (internal/testutil) open databases through exactly the same
// DSN as production; a fixture with a different DSN would run the suite with different pragma
// semantics from the application, which is how clause 1.1 stayed invisible for so long.
func DSN(databasePath string) string {
	return "file:" + escapeURIPath(databasePath) + "?" + pragmaQuery
}

// escapeURIPath percent-escapes a filesystem path for inclusion in a SQLite file: URI.
//
// url.URL is deliberately not used: it would escape ':' (breaking Windows drive letters and the
// ":memory:" special filename) or, with Opaque, leave reserved characters untouched. The
// unreserved set kept verbatim here is exactly what SQLite's URI parser needs to see literally:
// path separators, drive-letter and ":memory:" colons, and the RFC 3986 unreserved marks.
// Everything else — including '?', '#', '%', '&' and spaces — becomes a percent triplet, which
// SQLite decodes back to the original byte.
func escapeURIPath(databasePath string) string {
	// SQLite URIs use '/' as the separator on every platform; Windows backslashes are not URI
	// path separators and would have to be escaped, so they are normalized instead.
	databasePath = filepath.ToSlash(databasePath)

	var b strings.Builder
	b.Grow(len(databasePath))
	for i := 0; i < len(databasePath); i++ {
		c := databasePath[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '/', c == ':', c == '.', c == '-', c == '_', c == '~':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// Connect opens the SQLite database with WAL journalling, FOREIGN KEY enforcement and a 5s
// busy timeout, verifies that all three are genuinely in effect, and applies the connection
// pool configuration. Explicit pool limits are required for reliable concurrency at scale
// (Requirement 13.1): WAL permits many parallel readers with a single writer, and busy_timeout
// serializes writes safely.
//
// The verification is not redundant. The three pragmas are configured through the DSN, and a
// DSN whose syntax the driver does not recognise fails silently — the connection opens, every
// parameter is dropped, and the application runs with journal_mode=delete, busy_timeout=0 and
// no FOREIGN KEY enforcement (codebase-bug-sweep clause 1.1). Verifying after Ping turns that
// class of mistake from a silent data-integrity hole into a startup error.
func Connect(databasePath string, pool PoolConfig) error {
	var err error
	DB, err = sql.Open("sqlite", DSN(databasePath))
	if err != nil {
		return err
	}

	applyPoolConfig(DB, pool)

	if err = DB.Ping(); err != nil {
		return err
	}

	if err = VerifyPragmas(DB); err != nil {
		return err
	}

	log.Printf("Database connected successfully (SQLite + WAL, foreign_keys on, busy_timeout 5s, max_open=%d, max_idle=%d)",
		pool.MaxOpenConns, pool.MaxIdleConns)
	return nil
}

// VerifyPragmas reports whether the pragmas required by codebase-bug-sweep clause 2.1 are in
// effect on the given database, naming the offending pragma together with its expected and
// actual value for every mismatch.
//
// Scope of the guarantee. foreign_keys and busy_timeout are per-connection settings, while
// *sql.DB is a pool that opens connections lazily (MaxOpenConns=25). Reading them here observes
// one pooled connection, so this is not, and cannot be, a per-connection assertion for the
// whole pool. The actual guarantee comes from the DSN: the driver replays every _pragma on each
// new connection it opens. What this function detects is a malformed or unrecognised DSN —
// exactly the failure mode that went unnoticed before — which is deterministic and therefore
// fully observable from a single connection. journal_mode=WAL is unaffected either way: it is
// persisted in the database file header, not per connection.
//
// Exported so that test fixtures opening a database directly can assert the same invariant
// without duplicating the expectations.
func VerifyPragmas(database *sql.DB) error {
	var problems []string
	for _, p := range requiredPragmas {
		var actual string
		// The pragma name is from a compile-time constant list, never from input.
		if err := database.QueryRow("PRAGMA " + p.name).Scan(&actual); err != nil {
			return fmt.Errorf("read PRAGMA %s: %w", p.name, err)
		}
		if !p.matches(actual) {
			problems = append(problems, fmt.Sprintf("%s: expected %q, actual %q", p.name, p.expected, actual))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("sqlite pragma verification failed (%s); the connection string %q was not applied as expected",
			strings.Join(problems, "; "), pragmaQuery)
	}
	return nil
}

// applyPoolConfig applies the connection-pool limits. All three limits are always
// applied: the pool values originate from db.DefaultPoolConfig (25/10/30m) or from the
// environment via config.Load, both of which are guaranteed positive — getEnvInt
// falls back on non-positive input, so zero can never reach this code. There is
// intentionally no "zero means unlimited" escape hatch: such a path would be
// unreachable and its presence would be a misleading dead branch (Requirement 16.6).
func applyPoolConfig(database *sql.DB, pool PoolConfig) {
	database.SetMaxOpenConns(pool.MaxOpenConns)
	database.SetMaxIdleConns(pool.MaxIdleConns)
	database.SetConnMaxLifetime(pool.ConnMaxLifetime)
}

func Close() error {
	return DB.Close()
}
