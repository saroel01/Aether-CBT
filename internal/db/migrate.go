package db

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunMigrations executes all .sql files in internal/db/migrations in lexical order.
// Files must use "CREATE TABLE IF NOT EXISTS" and "INSERT OR IGNORE" for idempotency.
func RunMigrations() error {
	migrationsDir := "internal/db/migrations"

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

		// Execute the entire file (migrations are written to be re-runnable)
		_, err = DB.Exec(string(content))
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "duplicate column name") || strings.Contains(errStr, "already exists") || strings.Contains(errStr, "SQLITE_BUSY") {
				log.Printf("Migration %s skipped: %v", f, err)
			} else {
				log.Printf("Migration %s failed: %v", f, err)
				return err
			}
		} else {
			log.Printf("Applied migration: %s", f)
		}
	}

	log.Println("All migrations applied successfully")
	return nil
}
