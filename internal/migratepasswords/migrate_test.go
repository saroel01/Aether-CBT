package migratepasswords

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/utils"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	// modernc.org/sqlite gives each connection its own private in-memory DB, so pin a
	// single connection to make the table visible across all statements in this test.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE peserta (id INTEGER PRIMARY KEY, password TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestMigrateHashesPlaintextRows(t *testing.T) {
	db := openTestDB(t)
	// Seed one plaintext and one already-hashed row.
	hashed, err := utils.HashPassword("already-hashed-pw")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO peserta (id, password) VALUES (1, ?), (2, ?), (3, '')`,
		"siswa123", hashed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stats, err := Migrate(db)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if stats.Hashed != 1 {
		t.Fatalf("hashed = %d, want 1 (only the plaintext row)", stats.Hashed)
	}
	if stats.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1 (already-hashed row; empty password ignored)", stats.Skipped)
	}
	if stats.Empty != 1 {
		t.Fatalf("empty = %d, want 1", stats.Empty)
	}

	// The plaintext row must now be a bcrypt hash that validates the same password.
	var stored string
	if err := db.QueryRow(`SELECT password FROM peserta WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("select: %v", err)
	}
	if !utils.CheckPasswordHash("siswa123", stored) {
		t.Fatalf("rehashed password does not validate the original plaintext: %q", stored)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(`INSERT INTO peserta (id, password) VALUES (1, 'plaintext'), (2, 'other')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	first, err := Migrate(db)
	if err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if first.Hashed != 2 {
		t.Fatalf("first pass hashed = %d, want 2", first.Hashed)
	}

	// Second run: every row is now a hash, nothing to do.
	second, err := Migrate(db)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if second.Hashed != 0 {
		t.Fatalf("second pass hashed = %d, want 0 (idempotent)", second.Hashed)
	}
	if second.Skipped != 2 {
		t.Fatalf("second pass skipped = %d, want 2", second.Skipped)
	}
}
