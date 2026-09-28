package main

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/db"
)

func TestBackupScript_CleanDatabase(t *testing.T) {
	tempDir := t.TempDir()
	sourceDBPath := filepath.Join(tempDir, "source.db")
	backupOutDir := filepath.Join(tempDir, "backups")

	// Create and migrate source db
	if err := db.Connect(sourceDBPath, db.DefaultPoolConfig()); err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	migrationsDir := filepath.Join("..", "internal", "db", "migrations")
	if err := db.RunMigrations(db.DB, migrationsDir); err != nil {
		db.Close()
		t.Fatalf("RunMigrations: %v", err)
	}

	// Tenant 1 is already seeded by migration 001. Close db so backup can run against it.
	db.Close()

	// Execute backup script
	cmd := exec.Command("go", "run", "./backup.go", "-db", sourceDBPath, "-out", backupOutDir)
	cmd.Dir = "." // runs inside scripts/
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("backup.go failed: %v\nOutput: %s", err, string(out))
	}

	outStr := string(out)
	if !strings.Contains(outStr, "Backup berhasil dan terverifikasi!") {
		t.Errorf("expected success message, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Integrity  : ok (diverifikasi pada file backup)") {
		t.Errorf("expected integrity check message, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Foreign Key: ok (0 violations)") {
		t.Errorf("expected foreign key ok message, got: %s", outStr)
	}

	// Verify backup file exists
	entries, err := os.ReadDir(backupOutDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected backup file in %s, found %d entries", backupOutDir, len(entries))
	}

	backupFile := filepath.Join(backupOutDir, entries[0].Name())
	bConn, err := sql.Open("sqlite", db.DSN(backupFile))
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer bConn.Close()

	var integrity string
	if err := bConn.QueryRow("PRAGMA integrity_check;").Scan(&integrity); err != nil || integrity != "ok" {
		t.Errorf("backup file integrity = %q, err = %v", integrity, err)
	}
}

func TestBackupScript_RejectsFKViolations(t *testing.T) {
	tempDir := t.TempDir()
	sourceDBPath := filepath.Join(tempDir, "fk_violation.db")
	backupOutDir := filepath.Join(tempDir, "backups_corrupt")

	// Connect and migrate
	if err := db.Connect(sourceDBPath, db.DefaultPoolConfig()); err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	migrationsDir := filepath.Join("..", "internal", "db", "migrations")
	if err := db.RunMigrations(db.DB, migrationsDir); err != nil {
		db.Close()
		t.Fatalf("RunMigrations: %v", err)
	}
	db.Close()

	// Intentionally create a foreign key violation by inserting orphaned row with FK disabled
	rawConn, err := sql.Open("sqlite", sourceDBPath) // bare connection without FK pragma
	if err != nil {
		t.Fatalf("rawConn: %v", err)
	}
	_, err = rawConn.Exec("INSERT INTO hasil_tes_detail (id, hasil_tes_id, question_id) VALUES (9999, 99999, 'q1')")
	if err != nil {
		rawConn.Close()
		t.Fatalf("insert orphaned detail: %v", err)
	}
	rawConn.Close()

	// Execute backup script - must fail verification and exit 1
	cmd := exec.Command("go", "run", "./backup.go", "-db", sourceDBPath, "-out", backupOutDir)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected backup.go to fail on FK violations, but succeeded! Output: %s", string(out))
	}

	outStr := string(out)
	if !strings.Contains(outStr, "pelanggaran foreign key") {
		t.Errorf("expected FK violation error, got: %s", outStr)
	}

	// Corrupt backup file must have been deleted
	entries, _ := os.ReadDir(backupOutDir)
	if len(entries) != 0 {
		t.Errorf("corrupt backup file was not removed, found %d files", len(entries))
	}
}

func TestResetQueueScript_RequiresConfirmationOrForce(t *testing.T) {
	tempDir := t.TempDir()
	sourceDBPath := filepath.Join(tempDir, "queue_test.db")

	if err := db.Connect(sourceDBPath, db.DefaultPoolConfig()); err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	migrationsDir := filepath.Join("..", "internal", "db", "migrations")
	if err := db.RunMigrations(db.DB, migrationsDir); err != nil {
		db.Close()
		t.Fatalf("RunMigrations: %v", err)
	}
	db.Close()

	// Running without --yes or --force on non-interactive stdin should fail (exit non-zero)
	cmd := exec.Command("go", "run", "./reset_queue.go", "-db", sourceDBPath)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected reset_queue.go without --yes/--force to fail without confirmation, got success: %s", string(out))
	}

	// Running with --force should succeed
	cmdForce := exec.Command("go", "run", "./reset_queue.go", "-db", sourceDBPath, "--force")
	cmdForce.Dir = "."
	outForce, errForce := cmdForce.CombinedOutput()
	if errForce != nil {
		t.Fatalf("reset_queue.go --force failed: %v, output: %s", errForce, string(outForce))
	}
	if !strings.Contains(string(outForce), "Reset complete.") {
		t.Errorf("expected 'Reset complete.', got: %s", string(outForce))
	}

	// Running with -y shorthand should also succeed
	cmdY := exec.Command("go", "run", "./reset_queue.go", "-db", sourceDBPath, "-y")
	cmdY.Dir = "."
	outY, errY := cmdY.CombinedOutput()
	if errY != nil {
		t.Fatalf("reset_queue.go -y failed: %v, output: %s", errY, string(outY))
	}
	if !strings.Contains(string(outY), "Reset complete.") {
		t.Errorf("expected 'Reset complete.', got: %s", string(outY))
	}
}

func TestCheckpointScript(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "checkpoint_test.db")

	if err := db.Connect(dbPath, db.DefaultPoolConfig()); err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	migrationsDir := filepath.Join("..", "internal", "db", "migrations")
	if err := db.RunMigrations(db.DB, migrationsDir); err != nil {
		db.Close()
		t.Fatalf("RunMigrations: %v", err)
	}
	db.Close()

	// 1. Standard checkpoint on clean DB
	cmd := exec.Command("go", "run", "./checkpoint.go", "-db", dbPath)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("checkpoint.go failed: %v, output: %s", err, string(out))
	}
	if !strings.Contains(string(out), "WAL checkpoint (TRUNCATE) berhasil diselesaikan") {
		t.Errorf("expected success message, got: %s", string(out))
	}

	// 2. Verify-only on clean DB
	cmdVerify := exec.Command("go", "run", "./checkpoint.go", "-db", dbPath, "-verify-only")
	cmdVerify.Dir = "."
	outVerify, errVerify := cmdVerify.CombinedOutput()
	if errVerify != nil {
		t.Fatalf("checkpoint.go -verify-only failed: %v, output: %s", errVerify, string(outVerify))
	}
	if !strings.Contains(string(outVerify), "terverifikasi (ok)") {
		t.Errorf("expected verified ok message, got: %s", string(outVerify))
	}

	// 3. Non-existent DB without verify-only should report gracefully with exit 0
	nonExistent := filepath.Join(tempDir, "does_not_exist.db")
	cmdMissing := exec.Command("go", "run", "./checkpoint.go", "-db", nonExistent)
	cmdMissing.Dir = "."
	outMissing, errMissing := cmdMissing.CombinedOutput()
	if errMissing != nil {
		t.Fatalf("checkpoint.go on non-existent DB failed with error: %v", errMissing)
	}
	if !strings.Contains(string(outMissing), "tidak ditemukan, tidak perlu checkpoint") {
		t.Errorf("expected non-existent message, got: %s", string(outMissing))
	}

	// 3b. Non-existent DB with -verify-only must fail with non-zero exit code
	cmdMissingVerify := exec.Command("go", "run", "./checkpoint.go", "-db", nonExistent, "-verify-only")
	cmdMissingVerify.Dir = "."
	outMissingVerify, errMissingVerify := cmdMissingVerify.CombinedOutput()
	if errMissingVerify == nil {
		t.Fatalf("expected checkpoint.go -verify-only on missing file to fail, but succeeded! Output: %s", string(outMissingVerify))
	}
	if !strings.Contains(string(outMissingVerify), "tidak ditemukan") {
		t.Errorf("expected 'tidak ditemukan' in error output, got: %s", string(outMissingVerify))
	}

	// 4. Corrupt file should fail integrity check with non-zero exit code
	corruptPath := filepath.Join(tempDir, "corrupt.db")
	if err := os.WriteFile(corruptPath, []byte("NOT A VALID SQLITE DATABASE CONTENT"), 0644); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}
	cmdCorrupt := exec.Command("go", "run", "./checkpoint.go", "-db", corruptPath, "-verify-only")
	cmdCorrupt.Dir = "."
	outCorrupt, errCorrupt := cmdCorrupt.CombinedOutput()
	if errCorrupt == nil {
		t.Fatalf("expected checkpoint.go to fail on corrupt file, but succeeded! Output: %s", string(outCorrupt))
	}
	if !strings.Contains(string(outCorrupt), "ERROR") {
		t.Errorf("expected ERROR in output, got: %s", string(outCorrupt))
	}

	// 5. Database with Foreign Key violation must fail verification
	fkPath := filepath.Join(tempDir, "fk_broken.db")
	if err := db.Connect(fkPath, db.DefaultPoolConfig()); err != nil {
		t.Fatalf("db.Connect fkPath: %v", err)
	}
	if err := db.RunMigrations(db.DB, migrationsDir); err != nil {
		db.Close()
		t.Fatalf("RunMigrations fkPath: %v", err)
	}
	db.Close()
	rawConn, err := sql.Open("sqlite", fkPath)
	if err != nil {
		t.Fatalf("rawConn fkPath: %v", err)
	}
	_, err = rawConn.Exec("INSERT INTO hasil_tes_detail (id, hasil_tes_id, question_id) VALUES (8888, 88888, 'q1')")
	if err != nil {
		rawConn.Close()
		t.Fatalf("insert fk orphan: %v", err)
	}
	rawConn.Close()

	cmdFK := exec.Command("go", "run", "./checkpoint.go", "-db", fkPath, "-verify-only")
	cmdFK.Dir = "."
	outFK, errFK := cmdFK.CombinedOutput()
	if errFK == nil {
		t.Fatalf("expected checkpoint.go -verify-only to fail on FK violation, but passed! Output: %s", string(outFK))
	}
	if !strings.Contains(string(outFK), "pelanggaran foreign key") {
		t.Errorf("expected 'pelanggaran foreign key' in output, got: %s", string(outFK))
	}
}

func TestInspectQueueScript(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "inspect_test.db")

	if err := db.Connect(dbPath, db.DefaultPoolConfig()); err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	migrationsDir := filepath.Join("..", "internal", "db", "migrations")
	if err := db.RunMigrations(db.DB, migrationsDir); err != nil {
		db.Close()
		t.Fatalf("RunMigrations: %v", err)
	}
	db.Close()

	cmd := exec.Command("go", "run", "./inspect_queue.go", "-db", dbPath)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect_queue.go failed: %v, output: %s", err, string(out))
	}
	outStr := string(out)
	if !strings.Contains(outStr, "submission_queue counts by status") {
		t.Errorf("expected header in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "failed_submissions count") {
		t.Errorf("expected failed_submissions header in output, got: %s", outStr)
	}
}
