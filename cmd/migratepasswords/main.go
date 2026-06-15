// Command migratepasswords performs a one-shot, idempotent rehash of any legacy
// plaintext peserta.password values to bcrypt. Run it once per install before enabling
// strict (no-plaintext-fallback) auth:
//
//	go run ./cmd/migratepasswords
//
// It is safe to run repeatedly: rows that already hold a bcrypt hash are skipped.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/migratepasswords"
)

func main() {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "data/cbt_aether.db"
	}

	// Connect with the project's standard pool config (WAL, busy_timeout, FK on).
	if err := db.Connect(dbPath, db.DefaultPoolConfig()); err != nil {
		log.Fatalf("connect db %s: %v", dbPath, err)
	}
	defer db.Close()

	// Ensure the schema is current (peserta.password column exists, etc.).
	if err := db.RunMigrations(db.DB, "internal/db/migrations"); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	stats, err := migratepasswords.Migrate(db.DB)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	fmt.Printf("password migration complete: %d hashed, %d already-hashed skipped, %d empty\n",
		stats.Hashed, stats.Skipped, stats.Empty)
}
