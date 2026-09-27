package config

import (
	"os"
	"strings"
	"testing"
)

func TestValidateJWTSecret_RejectsEmpty(t *testing.T) {
	for _, env := range []string{"development", "production", "staging"} {
		if err := ValidateJWTSecret("", env); err == nil {
			t.Errorf("ValidateJWTSecret(\"\", %q) = nil, want error", env)
		}
		if err := ValidateJWTSecret("   ", env); err == nil {
			t.Errorf("ValidateJWTSecret(\"   \", %q) = nil, want error", env)
		}
	}
}

func TestValidateJWTSecret_RejectsDenylist(t *testing.T) {
	denylist := []string{
		"supersecurejwtkey2026",
		"aether-cbt-secret-key-change-in-production",
		"secret",
		"secretkey",
		"changeme",
		"jwt_secret",
		"jwtsecret",
		"admin",
		"admin123",
		"default",
		"password",
		"123456",
		"12345678",
		"your_secure_secret_here",
		"IsiDenganSecretPanjangAcakMinimal32Karakter",
		"KunciSangatPanjangDanAcak2026Min32Karakter",
	}

	for _, sec := range denylist {
		// Should fail in development
		if err := ValidateJWTSecret(sec, "development"); err == nil {
			t.Errorf("ValidateJWTSecret(%q, \"development\") = nil, want error", sec)
		}
		// Should fail in production
		if err := ValidateJWTSecret(sec, "production"); err == nil {
			t.Errorf("ValidateJWTSecret(%q, \"production\") = nil, want error", sec)
		}
		// Case insensitive check
		if err := ValidateJWTSecret(strings.ToUpper(sec), "development"); err == nil {
			t.Errorf("ValidateJWTSecret(%q, \"development\") = nil, want error", strings.ToUpper(sec))
		}
	}
}

func TestValidateJWTSecret_ProductionLengthRequirement(t *testing.T) {
	shortSecret := "short-custom-secret-key-12345" // 29 chars < 32
	if err := ValidateJWTSecret(shortSecret, "production"); err == nil {
		t.Errorf("ValidateJWTSecret(%q, \"production\") = nil, want error for len < 32", shortSecret)
	}

	// But in development it can be accepted
	if err := ValidateJWTSecret(shortSecret, "development"); err != nil {
		t.Errorf("ValidateJWTSecret(%q, \"development\") = %v, want nil", shortSecret, err)
	}

	// Valid production secret (>= 32 chars)
	validProdSecret := "a-very-strong-and-secure-random-jwt-key-2026-production"
	if err := ValidateJWTSecret(validProdSecret, "production"); err != nil {
		t.Errorf("ValidateJWTSecret(%q, \"production\") = %v, want nil", validProdSecret, err)
	}
}

func TestLoadWithError_RejectsInsecureInProduction(t *testing.T) {
	origSecret := os.Getenv("JWT_SECRET")
	origEnv := os.Getenv("ENV")
	defer func() {
		os.Setenv("JWT_SECRET", origSecret)
		os.Setenv("ENV", origEnv)
	}()

	os.Setenv("ENV", "production")
	os.Setenv("JWT_SECRET", "supersecurejwtkey2026")

	_, err := LoadWithError()
	if err == nil {
		t.Fatal("LoadWithError() with supersecurejwtkey2026 in production = nil, want error")
	}

	// Now with a valid 32+ character secret
	os.Setenv("JWT_SECRET", "super-strong-jwt-production-secret-min32-chars!!")
	cfg, err := LoadWithError()
	if err != nil {
		t.Fatalf("LoadWithError() with valid secret failed: %v", err)
	}
	if cfg.JWTSecret != "super-strong-jwt-production-secret-min32-chars!!" {
		t.Errorf("cfg.JWTSecret = %q, want expected", cfg.JWTSecret)
	}
}
