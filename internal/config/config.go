package config

import (
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
	Environment string
	JWTSecret   string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "3000"),
		DatabaseURL: getEnv("DATABASE_URL", "data/cbt_aether.db"),
		Environment: getEnv("ENV", "development"),
		JWTSecret:   getEnv("JWT_SECRET", "aether-cbt-secret-key-change-in-production"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
