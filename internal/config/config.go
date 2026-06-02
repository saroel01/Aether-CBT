package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

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

func Load() *Config {
	cfg := &Config{
		Port:               getEnv("PORT", "3000"),
		DatabaseURL:        getEnv("DATABASE_URL", "data/cbt_aether.db"),
		Environment:        getEnv("ENV", "development"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", ""),

		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute,

		SoalUploadMaxBytes:  getEnvInt64("SOAL_UPLOAD_MAX_BYTES", 100*1024*1024),
		SoalPackageMaxFiles: getEnvInt("SOAL_PACKAGE_MAX_FILES", 5000),

		AntiCheatLockThreshold: getEnvInt("ANTICHEAT_LOCK_THRESHOLD", 3),

		ContentCookieSecure: getEnv("CONTENT_COOKIE_SECURE", "auto"),
	}

	if cfg.JWTSecret == "" {
		log.Fatal("FATAL: JWT_SECRET wajib diisi melalui environment variable. Jangan gunakan secret default yang lemah.")
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
