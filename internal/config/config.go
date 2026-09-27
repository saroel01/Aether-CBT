package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/saroel01/aether-cbt/internal/db"
)

// InsecureJWTSecrets contains known default or weak secrets that must never be used.
var InsecureJWTSecrets = map[string]struct{}{
	"supersecurejwtkey2026":                      {},
	"aether-cbt-secret-key-change-in-production": {},
	"secret":                  {},
	"secretkey":               {},
	"changeme":                {},
	"jwt_secret":              {},
	"jwtsecret":               {},
	"admin":                   {},
	"admin123":                {},
	"default":                 {},
	"password":                {},
	"123456":                  {},
	"12345678":                {},
	"your_secure_secret_here": {},
	"isidengansecretpanjangacakminimal32karakter": {},
	"kuncisangatpanjangdanacak2026min32karakter":  {},
}

// ValidateJWTSecret validates that the JWT secret is non-empty, not in the insecure denylist,
// and meets the minimum length requirement (>= 32 chars) when in production.
func ValidateJWTSecret(secret, env string) error {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return errors.New("FATAL: JWT_SECRET wajib diisi melalui environment variable. Jangan gunakan secret default yang lemah.")
	}

	lower := strings.ToLower(trimmed)
	if _, bad := InsecureJWTSecrets[lower]; bad {
		return fmt.Errorf("FATAL: JWT_SECRET menggunakan secret default atau tidak aman: %q", secret)
	}

	isProd := strings.EqualFold(env, "production") || strings.EqualFold(env, "prod")
	if isProd {
		if len(trimmed) < 32 {
			return fmt.Errorf("FATAL: JWT_SECRET di environment production harus memiliki panjang minimal 32 karakter (saat ini: %d karakter)", len(trimmed))
		}
	}
	return nil
}

type Config struct {
	Port               string
	DatabaseURL        string
	Environment        string
	JWTSecret          string
	CORSAllowedOrigins string

	// Database connection pool (SQLite WAL tuning for concurrency, Requirement 13.1).
	// SQLite allows many concurrent readers but only one writer at a time; combined
	// with _busy_timeout=5000 and a single serialized queue worker for result writes,
	// a modest pool serves parallel reads safely.
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	// Soal package upload limits (Requirement 3.2, anti zip-bomb).
	SoalUploadMaxBytes  int64
	SoalPackageMaxFiles int

	// Anti-cheat lock threshold (Requirement 10.6).
	AntiCheatLockThreshold int

	// Content session cookie security (Requirement 8 / AD-2).
	// When empty (auto), the server enables Secure based on request scheme/environment.
	ContentCookieSecure string
}

// Validate checks the configuration for security requirements.
func (c *Config) Validate() error {
	return ValidateJWTSecret(c.JWTSecret, c.Environment)
}

// LoadWithError loads configuration from environment and returns an error if validation fails.
func LoadWithError() (*Config, error) {
	// Pool defaults come from a single canonical source (db.DefaultPoolConfig) so the
	// 25/10/30m values are not duplicated across packages and cannot drift silently
	// when one side changes (Requirement 13.7). Environment values override them.
	poolDefaults := db.DefaultPoolConfig()

	cfg := &Config{
		Port:               getEnv("PORT", "3000"),
		DatabaseURL:        getEnv("DATABASE_URL", "data/cbt_aether.db"),
		Environment:        getEnv("ENV", "development"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", ""),

		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", poolDefaults.MaxOpenConns),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", poolDefaults.MaxIdleConns),
		DBConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MIN", int(poolDefaults.ConnMaxLifetime/time.Minute))) * time.Minute,

		SoalUploadMaxBytes:  getEnvInt64("SOAL_UPLOAD_MAX_BYTES", 100*1024*1024),
		SoalPackageMaxFiles: getEnvInt("SOAL_PACKAGE_MAX_FILES", 5000),

		AntiCheatLockThreshold: getEnvInt("ANTICHEAT_LOCK_THRESHOLD", 3),

		ContentCookieSecure: getEnv("CONTENT_COOKIE_SECURE", "auto"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Load loads configuration from environment, terminating the process on error.
func Load() *Config {
	cfg, err := LoadWithError()
	if err != nil {
		log.Fatal(err)
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvInt returns the integer value of an environment variable, or the fallback
// when unset or invalid (non-numeric / non-positive).
func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Printf("config: invalid %s=%q, using fallback %d", key, value, fallback)
		return fallback
	}
	return parsed
}

// getEnvInt64 returns the int64 value of an environment variable, or the fallback
// when unset or invalid (non-numeric / non-positive).
func getEnvInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		log.Printf("config: invalid %s=%q, using fallback %d", key, value, fallback)
		return fallback
	}
	return parsed
}
