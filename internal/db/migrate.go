package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// RunMigrations applies every .sql migration not yet recorded in schema_migrations,
// in lexical order, each file inside one transaction that also records its version,
// so a file runs exactly once and a failing file leaves no partial schema behind.
//
// migrationsDir == "" uses the migrations embedded in the binary (default). A
// non-empty migrationsDir must exist on disk; a missing directory is an error.
//
// A pre-versioning database (schema_migrations empty, tenants present) is
// bootstrapped by running every file once in tolerant mode ("duplicate column name" /
// "already exists" are skipped per statement), which matches the historical boot
// behaviour, and recording each one. Files should still be written idempotently.
func RunMigrations(database *sql.DB, migrationsDir string) error {
	// Gate 0 prerequisite (codebase-bug-sweep clause 2.2): an existing database may still
	// declare peserta.kelas_id / peserta.ruang_id NOT NULL, which is what forced every write
	// path to express "not assigned" as the sentinel 0 — a FOREIGN KEY violation. Relaxing
	// that is a table rebuild and must happen before migration 032 nulls the sentinels.
	if err := repairPesertaNullableFK(database); err != nil {
		return err
	}

	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migFiles, err := loadMigrationFiles(migrationsDir)
	if err != nil {
		return err
	}

	applied := map[string]bool{}
	rows, err := database.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(applied) == 0 {
		var n int
		if err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tenants'`).Scan(&n); err == nil && n > 0 {
			// Pre-versioning database: every file runs once in tolerant mode (exactly the
			// historical boot behaviour), then is recorded so it never runs again.
			log.Println("bootstrapping schema_migrations on pre-versioning database")
		}
	}

	for _, mf := range migFiles {
		if applied[mf.name] {
			continue
		}
		if err := applyMigrationFile(database, mf); err != nil {
			return err
		}
		log.Printf("Applied migration: %s", mf.name)
	}

	log.Println("All migrations applied successfully")

	// Data-integrity diagnostic (clause 2.2 / 3.1). Deliberately non-fatal: refusing to start
	// over a referential violation would stop a school from running its exam, which is worse
	// than running with a warning. The fail-fast check that belongs with the DSN targets
	// CONFIGURATION, which is deterministic and always fixable in code; this one targets DATA,
	// which is installation-dependent. Staying silent is not an option — that is exactly what
	// let the disabled-pragma defect hide for so long.
	logForeignKeyViolations(database)

	return nil
}

type migrationFile struct {
	name    string
	content []byte
}

// loadMigrationFiles returns the .sql files in lexical order. An empty migrationsDir
// selects the embedded set; a non-empty one must exist on disk (no silent fallback).
func loadMigrationFiles(migrationsDir string) ([]migrationFile, error) {
	var (
		names []string
		read  func(string) ([]byte, error)
	)
	if migrationsDir != "" {
		fi, err := os.Stat(migrationsDir)
		if err != nil {
			return nil, fmt.Errorf("migrations dir %q: %w", migrationsDir, err)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("migrations dir %q is not a directory", migrationsDir)
		}
		entries, err := os.ReadDir(migrationsDir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				names = append(names, e.Name())
			}
		}
		read = func(f string) ([]byte, error) { return os.ReadFile(filepath.Join(migrationsDir, f)) }
	} else {
		entries, err := embeddedMigrations.ReadDir("migrations")
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				names = append(names, e.Name())
			}
		}
		read = func(f string) ([]byte, error) { return embeddedMigrations.ReadFile("migrations/" + f) }
	}
	sort.Strings(names)
	files := make([]migrationFile, 0, len(names))
	for _, f := range names {
		content, err := read(f)
		if err != nil {
			return nil, err
		}
		files = append(files, migrationFile{name: f, content: content})
	}
	return files, nil
}

// applyMigrationFile runs one file inside a single transaction and records it in
// schema_migrations. SQLITE_BUSY rolls back and retries the whole file (max 3 times).
func applyMigrationFile(database *sql.DB, mf migrationFile) error {
	const maxRetries = 3
	for attempt := 0; ; attempt++ {
		err := applyMigrationFileOnce(database, mf)
		if err == nil {
			return nil
		}
		if !isBusyErr(err) || attempt >= maxRetries {
			return fmt.Errorf("migration %s: %w", mf.name, err)
		}
		time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
	}
}

func applyMigrationFileOnce(database *sql.DB, mf migrationFile) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range splitSQLStatements(string(mf.content)) {
		if err := execMigrationStatement(tx, mf.name, stmt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, mf.name); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// logForeignKeyViolations runs PRAGMA foreign_key_check and logs one warning line per
// offending table/rowid. It never returns an error and never aborts startup.
func logForeignKeyViolations(database *sql.DB) {
	rows, err := database.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		log.Printf("WARNING: foreign key diagnostic could not run: %v", err)
		return
	}
	defer rows.Close()

	const maxReported = 50
	var total int
	for rows.Next() {
		var table, parent sql.NullString
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			log.Printf("WARNING: foreign key diagnostic could not read a row: %v", err)
			return
		}
		total++
		if total <= maxReported {
			log.Printf("WARNING: foreign key violation: table %q rowid %d references missing row in %q (fk index %d)",
				table.String, rowid.Int64, parent.String, fkid.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("WARNING: foreign key diagnostic ended early: %v", err)
		return
	}
	if total > maxReported {
		log.Printf("WARNING: %d further foreign key violations were not listed", total-maxReported)
	}
	if total > 0 {
		log.Printf("WARNING: %d foreign key violation(s) present after migrations; the application will keep running, but these rows must be repaired", total)
	}
}

// execMigrationStatement executes a single migration statement. Idempotency errors
// that legitimate re-runnable migrations are expected to surface ("duplicate column
// name" / "already exists") are swallowed (SQLite rolls back only the failing
// statement, the transaction stays usable) so a database whose schema was partly
// created before versioning still converges. Any other error is returned; the caller
// rolls back the whole file. SQLITE_BUSY is retried per file by applyMigrationFile.
func execMigrationStatement(ex execer, file, stmt string) error {
	_, err := ex.Exec(stmt)
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if strings.Contains(errStr, "duplicate column name") ||
		strings.Contains(errStr, "already exists") {
		log.Printf("Migration %s statement skipped (idempotent): %v", file, err)
		return nil
	}
	log.Printf("Migration %s statement failed: %v", file, err)
	return err
}

// execer is satisfied by *sql.DB and *sql.Tx.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// isBusyErr reports whether err is a SQLite SQLITE_BUSY error.
func isBusyErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "SQLITE_BUSY")
}

// splitSQLStatements splits migration SQL into individual statements. It respects
// single-quoted string literals (so a ';' inside a value does not split the statement)
// and strips SQL line comments ('--' to end of line). Empty / whitespace-only
// statements are dropped.
//
// This keeps the splitter robust against values containing ';' (a future INSERT or
// DEFAULT) while remaining proportional to the simple DDL used by the migrations in
// this repository.
func splitSQLStatements(content string) []string {
	var (
		stmts []string
		buf   strings.Builder
		inStr bool
	)

	flush := func() {
		if s := strings.TrimSpace(buf.String()); s != "" {
			stmts = append(stmts, s)
		}
		buf.Reset()
	}

	for i := 0; i < len(content); {
		ch := content[i]

		// Line comment: skip to end of line (only when not inside a string literal).
		if !inStr && ch == '-' && i+1 < len(content) && content[i+1] == '-' {
			for i < len(content) && content[i] != '\n' {
				i++
			}
			continue
		}

		// Single-quoted string literal: track state, treating '' as an escaped quote.
		if ch == '\'' {
			if inStr && i+1 < len(content) && content[i+1] == '\'' {
				buf.WriteByte(ch)
				buf.WriteByte(content[i+1])
				i += 2
				continue
			}
			inStr = !inStr
			buf.WriteByte(ch)
			i++
			continue
		}

		// Statement terminator outside a string literal.
		if !inStr && ch == ';' {
			flush()
			i++
			continue
		}

		buf.WriteByte(ch)
		i++
	}
	flush()
	return stmts
}
