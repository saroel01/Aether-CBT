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
	"github.com/saroel01/aether-cbt/internal/utils"
)

func TestIsWeakAdminPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantWeak bool
	}{
		{"empty", "", true},
		{"short", "1234567", true},
		{"admin lowercase", "admin", true},
		{"admin mixed", "Admin", true},
		{"admin123 lowercase", "admin123", true},
		{"admin123 mixed", "Admin123", true},
		{"admin123 upper", "ADMIN123", true},
		{"password default", "password", true},
		{"password mixed", "Password", true},
		{"password123", "Password123", true},
		{"12345678", "12345678", true},
		{"administrator", "Administrator", true},
		{"changeme", "ChangeMe", true},
		{"secret", "secret", true},
		{"supersecurejwtkey2026", "supersecurejwtkey2026", true},
		{"valid strong", "SuperAdmin2026!Key", false},
		{"valid strong 8 chars", "A9#xK2!z", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isWeakAdminPassword(tc.password)
			if got != tc.wantWeak {
				t.Errorf("isWeakAdminPassword(%q) = %v, want %v", tc.password, got, tc.wantWeak)
			}
		})
	}
}

func TestCreateAdminCLI_ProductionWeakPasswordRejection(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_admin.db")

	weakPasswords := []string{"Admin123", "ADMIN123", "admin123", "12345678", "Password", "short"}
	for _, pw := range weakPasswords {
		t.Run("reject_"+pw, func(t *testing.T) {
			cmd := exec.Command("go", "run", ".", "-db", dbPath, "-password", pw)
			cmd.Dir = "."
			cmd.Env = append(os.Environ(), "ENV=production")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected createadmin to fail in production for weak password %q, but succeeded! Output: %s", pw, string(out))
			}
			if !strings.Contains(string(out), "FATAL: Password admin di environment production") {
				t.Errorf("expected FATAL error message for weak password %q, got: %s", pw, string(out))
			}
		})
	}
}

func TestCreateAdminCLI_ProductionRequiresPassword(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_admin_prod.db")

	cmd := exec.Command("go", "run", ".", "-db", dbPath)
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "ENV=production")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected createadmin to fail in production with no password, but succeeded! Output: %s", string(out))
	}
	if !strings.Contains(string(out), "FATAL: Di environment production, password admin wajib ditentukan") {
		t.Errorf("expected FATAL error message for missing password, got: %s", string(out))
	}
}

func TestCreateAdminCLI_SuccessAndIdempotency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_admin_success.db")

	// 1. First run creates the admin user
	cmd := exec.Command("go", "run", ".", "-db", dbPath, "-password", "SuperAdminSecretKey2026!")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "ENV=production")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("createadmin initial creation failed: %v, output: %s", err, string(out))
	}
	if !strings.Contains(string(out), "Admin user created/updated successfully!") {
		t.Errorf("expected success message, got: %s", string(out))
	}

	// Verify user is in DB and active
	rawConn, err := sql.Open("sqlite", db.DSN(dbPath))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer rawConn.Close()

	var hash string
	var isActive bool
	err = rawConn.QueryRow("SELECT password_hash, is_active FROM users WHERE username = 'admin' AND tenant_id = 1").Scan(&hash, &isActive)
	if err != nil {
		t.Fatalf("query created admin: %v", err)
	}
	if !isActive {
		t.Error("expected admin user to be active")
	}
	if !utils.CheckPasswordHash("SuperAdminSecretKey2026!", hash) {
		t.Error("password hash does not match configured password")
	}

	// 2. Second run without -force should report existing without modifying
	cmdSecond := exec.Command("go", "run", ".", "-db", dbPath, "-password", "AnotherDifferentPassword123!")
	cmdSecond.Dir = "."
	cmdSecond.Env = append(os.Environ(), "ENV=production")
	outSecond, errSecond := cmdSecond.CombinedOutput()
	if errSecond != nil {
		t.Fatalf("createadmin idempotent run failed: %v, output: %s", errSecond, string(outSecond))
	}
	if !strings.Contains(string(outSecond), "already exists. Use --force to update credentials") {
		t.Errorf("expected already exists warning, got: %s", string(outSecond))
	}

	// 3. Third run with -force should update password
	cmdThird := exec.Command("go", "run", ".", "-db", dbPath, "-password", "UpdatedStrongPassword2026!", "-force")
	cmdThird.Dir = "."
	cmdThird.Env = append(os.Environ(), "ENV=production")
	outThird, errThird := cmdThird.CombinedOutput()
	if errThird != nil {
		t.Fatalf("createadmin force update run failed: %v, output: %s", errThird, string(outThird))
	}
	if !strings.Contains(string(outThird), "Admin user created/updated successfully!") {
		t.Errorf("expected success message on force update, got: %s", string(outThird))
	}

	var newHash string
	err = rawConn.QueryRow("SELECT password_hash FROM users WHERE username = 'admin' AND tenant_id = 1").Scan(&newHash)
	if err != nil {
		t.Fatalf("query updated admin: %v", err)
	}
	if !utils.CheckPasswordHash("UpdatedStrongPassword2026!", newHash) {
		t.Error("password hash was not updated with new password")
	}
}
