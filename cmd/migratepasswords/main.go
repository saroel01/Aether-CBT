// Command migratepasswords performs a one-shot, idempotent rehash of any legacy
// plaintext peserta.password values to bcrypt. Run it once per install before enabling
// strict (no-plaintext-fallback) auth:
//
//	go run ./cmd/migratepasswords
//
// It is safe to run repeatedly: rows that already hold a bcrypt hash are skipped.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/migratepasswords"
)

func main() {
	defaultDB := os.Getenv("DATABASE_PATH")
	if defaultDB == "" {
		defaultDB = os.Getenv("DATABASE_URL")
	}
	if defaultDB == "" {
		defaultDB = "data/cbt_aether.db"
	}

	dbPath := flag.String("db", defaultDB, "Path to sqlite database")
	flag.Parse()

	// Connect with the project's standard pool config (WAL, busy_timeout, FK on via db.DSN).
	if err := db.Connect(*dbPath, db.DefaultPoolConfig()); err != nil {
		log.Fatalf("connect db %s: %v", *dbPath, err)
	}
	defer db.Close()

	// Ensure the schema is current (peserta.password column exists, etc.).
	if err := db.RunMigrations(db.DB, os.Getenv("MIGRATIONS_DIR")); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	stats, err := migratepasswords.Migrate(db.DB)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	fmt.Printf("password migration complete: %d hashed, %d already-hashed skipped, %d empty\n",
		stats.Hashed, stats.Skipped, stats.Empty)
}
