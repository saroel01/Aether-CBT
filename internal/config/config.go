package config

import (
	"log"
	"os"
)

type Config struct {
	Port                string
	DatabaseURL         string
	Environment         string
	JWTSecret           string
	CORSAllowedOrigins  string
}

func Load() *Config {
	cfg := &Config{
		Port:               getEnv("PORT", "3000"),
		DatabaseURL:        getEnv("DATABASE_URL", "data/cbt_aether.db"),
		Environment:        getEnv("ENV", "development"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", ""),
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
