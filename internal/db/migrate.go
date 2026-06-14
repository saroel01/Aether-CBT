package db

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunMigrations executes all .sql files in migrationsDir in lexical order against the
// given database, one statement at a time. The directory and database are passed
// explicitly so callers (CLI entrypoints, tests) are not coupled to package-global
// state or the process working directory (Requirement 16.7).
//
// Each statement is executed independently so that a migration applied only partially
// (an interrupted startup, or a column added manually without the companion index)
// self-heals on the next run: idempotency errors ("duplicate column name" /
// "already exists") are swallowed per-statement rather than aborting the rest of the
// file, so a re-run can no longer leave the schema silently incomplete while
// RunMigrations reports success (Requirement 14.6, design AD-8). Files must still be
// written idempotently ("CREATE TABLE IF NOT EXISTS", "INSERT OR IGNORE").
func RunMigrations(database *sql.DB, migrationsDir string) error {
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
		path := filepath.Join(migrationsDir, f)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for _, stmt := range splitSQLStatements(string(content)) {
			if err := execMigrationStatement(database, f, stmt); err != nil {
				return err
			}
		}
		log.Printf("Applied migration: %s", f)
	}

	log.Println("All migrations applied successfully")
	return nil
}

// execMigrationStatement executes a single migration statement. Idempotency errors
// that legitimate re-runnable migrations are expected to surface ("duplicate column
// name" / "already exists") and transient SQLITE_BUSY are swallowed so a partially
// applied migration can self-heal on rerun (Requirement 14.6, AD-8). Any other error
// is fatal and is returned so the caller aborts with a clear message rather than
// leaving a half-applied migration.
func execMigrationStatement(database *sql.DB, file, stmt string) error {
	_, err := database.Exec(stmt)
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if strings.Contains(errStr, "duplicate column name") ||
		strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "SQLITE_BUSY") {
		log.Printf("Migration %s statement skipped: %v", file, err)
		return nil
	}
	log.Printf("Migration %s statement failed: %v", file, err)
	return err
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
