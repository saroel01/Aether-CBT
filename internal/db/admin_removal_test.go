package db

import (
	"testing"
)

// TestMigration033_PurgesDefaultAdminAccount (P0-4, AUDIT_REPO_2026-09-26.md:237)
// After RunMigrations on an empty DB, SELECT COUNT(*) FROM users WHERE role='admin' must be 0.
func TestMigration033_PurgesDefaultAdminAccount(t *testing.T) {
	testDB, cleanup := newTestDB(t)
	defer cleanup()

	if err := RunMigrations(testDB, migrationsDir(t)); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var adminCount int
	err := testDB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&adminCount)
	if err != nil {
		t.Fatalf("query admin count: %v", err)
	}

	if adminCount != 0 {
		t.Errorf("adminCount = %d, want 0 (default admin account must be removed by migration 033)", adminCount)
	}
}

// TestMigration033_PreservesCustomAdminAccount verifies that an admin with a custom password
// is NOT deleted by migration 033.
func TestMigration033_PreservesCustomAdminAccount(t *testing.T) {
	testDB, cleanup := newTestDB(t)
	defer cleanup()

	// Run migrations
	if err := RunMigrations(testDB, migrationsDir(t)); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert an admin with custom hash
	customHash := "$2a$14$customPasswordHashThatDoesNotMatchDefault1234567890"
	_, err := testDB.Exec(`
		INSERT INTO users (tenant_id, username, password_hash, role, full_name, is_active)
		VALUES (1, 'admin', ?, 'admin', 'System Administrator', TRUE)
	`, customHash)
	if err != nil {
		t.Fatalf("insert custom admin: %v", err)
	}

	// Re-run migration statement from 033
	_, err = testDB.Exec(`
		DELETE FROM users
		WHERE role = 'admin'
		  AND username = 'admin'
		  AND (
		    password_hash = '$2a$14$ZWg8M9q80U7P9MaoOFunseFWwQFM2nQsamPDBneEtxrUkIMdpwuMm'
		    OR password_hash LIKE '$2a$10$wTqS%'
		  );
	`)
	if err != nil {
		t.Fatalf("exec 033: %v", err)
	}

	var count int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 1 {
		t.Errorf("custom admin count = %d, want 1 (custom admin should not be deleted)", count)
	}
}
