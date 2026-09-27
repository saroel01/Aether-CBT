package utils

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestCheckPasswordRejectsPlaintextStoredValue ensures the password check has no
// plaintext fallback: a stored value that is NOT a bcrypt hash must never validate,
// even when it matches the supplied password byte-for-byte (review Critical #2, Task 6).
func TestCheckPasswordRejectsPlaintextStoredValue(t *testing.T) {
	hashed, err := HashPassword("siswa123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !CheckPasswordHash("siswa123", hashed) {
		t.Fatal("valid bcrypt password rejected")
	}
	// Plaintext-stored value MUST be rejected — no fallback.
	if CheckPasswordHash("siswa123", "siswa123") {
		t.Fatal("plaintext-stored value was accepted — fallback must be removed")
	}
	// Constant-time: must not panic and must reject a wrong password.
	if CheckPasswordHash("wrong", hashed) {
		t.Fatal("wrong password accepted")
	}
}

// TestCheckPasswordHashRejectsEmptyAndShortStored ensures malformed stored values
// can never reach bcrypt's comparison (which would panic/err on garbage).
func TestCheckPasswordHashRejectsEmptyAndShortStored(t *testing.T) {
	if CheckPasswordHash("anything", "") {
		t.Fatal("empty stored value accepted")
	}
	if CheckPasswordHash("anything", "x") {
		t.Fatal("short stored value accepted")
	}
	if CheckPasswordHash("anything", "$2a$") {
		t.Fatal("truncated bcrypt prefix accepted")
	}
}

// TestBcryptCostIs10 ensures the cost is lowered to 10 (P1-8) to prevent CPU starvation.
func TestBcryptCostIs10(t *testing.T) {
	if BcryptCost != 10 {
		t.Fatalf("expected BcryptCost 10, got %d", BcryptCost)
	}

	hashed, err := HashPassword("testpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	cost, err := bcrypt.Cost([]byte(hashed))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost != 10 {
		t.Fatalf("expected cost 10, got %d", cost)
	}
}
