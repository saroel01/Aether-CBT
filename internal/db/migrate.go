package db

import (
	"database/sql"
	"embed"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// RunMigrations executes all .sql files in migrationsDir in lexical order against the
// given database, one statement at a time. The directory and database are passed
// explicitly so callers (CLI entrypoints, tests) are not coupled to package-global
// state or the process working directory (Requirement 16.7).
//
// If migrationsDir does not exist on disk (e.g. running standalone binary), it falls
// back to embedded migrations automatically.
//
// Each statement is executed independently so that a migration applied only partially
// (an interrupted startup, or a column added manually without the companion index)
// self-heals on the next run: idempotency errors ("duplicate column name" /
// "already exists") are swallowed per-statement rather than aborting the rest of the
// file, so a re-run can no longer leave the schema silently incomplete while
// RunMigrations reports success (Requirement 14.6, design AD-8). Files must still be
// written idempotently ("CREATE TABLE IF NOT EXISTS", "INSERT OR IGNORE").
func RunMigrations(database *sql.DB, migrationsDir string) error {
	// Gate 0 prerequisite (codebase-bug-sweep clause 2.2): an existing database may still
	// declare peserta.kelas_id / peserta.ruang_id NOT NULL, which is what forced every write
	// path to express "not assigned" as the sentinel 0 — a FOREIGN KEY violation. Relaxing
	// that is a table rebuild and must happen before migration 032 nulls the sentinels.
	if err := repairPesertaNullableFK(database); err != nil {
		return err
	}

	type migrationFile struct {
		name    string
		content []byte
	}
	var migFiles []migrationFile

	// Check if migrationsDir exists on the filesystem (e.g. during local tests or dev)
	if migrationsDir != "" {
		if fi, err := os.Stat(migrationsDir); err == nil && fi.IsDir() {
			entries, err := os.ReadDir(migrationsDir)
			if err != nil {
				return err
			}
			var files []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
					files = append(files, e.Name())
				}
			}
			sort.Strings(files)
			for _, f := range files {
				content, err := os.ReadFile(filepath.Join(migrationsDir, f))
				if err != nil {
					return err
				}
				migFiles = append(migFiles, migrationFile{name: f, content: content})
			}
		}
	}

	// Fallback to embedded migrations if no files loaded from disk
	if len(migFiles) == 0 {
		entries, err := embeddedMigrations.ReadDir("migrations")
		if err != nil {
			return err
		}
		var files []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				files = append(files, e.Name())
			}
		}
		sort.Strings(files)
		for _, f := range files {
			content, err := embeddedMigrations.ReadFile("migrations/" + f)
			if err != nil {
				return err
			}
			migFiles = append(migFiles, migrationFile{name: f, content: content})
		}
	}

	for _, mf := range migFiles {
		for _, stmt := range splitSQLStatements(string(mf.content)) {
			if err := execMigrationStatement(database, mf.name, stmt); err != nil {
				return err
			}
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
// name" / "already exists") and transient SQLITE_BUSY are swallowed so a partially
// applied migration can self-heal on rerun (Requirement 14.6, AD-8). Any other error
// is fatal and is returned so the caller aborts with a clear message rather than
// leaving a half-applied migration.
func execMigrationStatement(database *sql.DB, file, stmt string) error {
	err := execWithBusyRetry(database, stmt)
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

// execWithBusyRetry runs stmt, retrying transient SQLITE_BUSY errors with a short backoff so
// a concurrent writer cannot make a migration silently skip (review data finding #2, Task 32).
func execWithBusyRetry(database *sql.DB, stmt string) error {
	const maxRetries = 3
	var err error
	for i := 0; ; i++ {
		_, err = database.Exec(stmt)
		if err == nil || !isBusyErr(err) {
			return err
		}
		if i >= maxRetries {
			return err
		}
		time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
	}
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
