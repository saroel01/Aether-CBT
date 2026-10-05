package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/saroel01/aether-cbt/internal/config"
	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// isWeakAdminPassword checks if a password is too short or matches known weak/default passwords (case-insensitive).
func isWeakAdminPassword(password string) bool {
	trimmed := strings.TrimSpace(password)
	if len(trimmed) < 8 {
		return true
	}
	lower := strings.ToLower(trimmed)
	if _, bad := config.InsecureJWTSecrets[lower]; bad {
		return true
	}
	switch lower {
	case "adminadmin", "password123", "administrator", "123456789", "qwertyui", "admin2026":
		return true
	}
	return false
}

func main() {
	defaultDB := os.Getenv("DATABASE_URL")
	if defaultDB == "" {
		defaultDB = os.Getenv("DATABASE_PATH")
	}
	if defaultDB == "" {
		defaultDB = "data/cbt_aether.db"
	}

	dbPath := flag.String("db", defaultDB, "Path to sqlite database")
	force := flag.Bool("force", false, "Overwrite existing admin password if user already exists")
	passwordFlag := flag.String("password", "", "Password for admin user (or set SETUP_ADMIN_PASSWORD / ADMIN_PASSWORD)")
	usernameFlag := flag.String("username", "admin", "Username for admin account")
	flag.Parse()

	env := strings.ToLower(os.Getenv("ENV"))
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	isProd := env == "production" || env == "prod" || appEnv == "production" || appEnv == "prod"

	password := strings.TrimSpace(*passwordFlag)
	if password == "" {
		password = strings.TrimSpace(os.Getenv("SETUP_ADMIN_PASSWORD"))
	}
	if password == "" {
		password = strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	}

	generated := false
	if password == "" {
		if isProd {
			log.Fatal("FATAL: Di environment production, password admin wajib ditentukan via --password atau env ADMIN_PASSWORD / SETUP_ADMIN_PASSWORD.")
		}
		// Non-production fallback: generate secure random password
		genPW, err := utils.GenerateSecureToken(12)
		if err != nil {
			log.Fatalf("generate secure password: %v", err)
		}
		password = genPW
		generated = true
	} else if isProd && isWeakAdminPassword(password) {
		log.Fatal("FATAL: Password admin di environment production minimal 8 karakter dan tidak boleh menggunakan password default/lemah.")
	}

	// Connect to database using standard pool & DSN with verified pragmas (P1-15, P2-37)
	if err := db.Connect(*dbPath, db.DefaultPoolConfig()); err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	// Ensure tables exist
	if err := db.RunMigrations(db.DB, os.Getenv("MIGRATIONS_DIR")); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	// Check if admin already exists
	var existingID int
	err := db.DB.QueryRow(`SELECT id FROM users WHERE tenant_id = 1 AND username = ?`, *usernameFlag).Scan(&existingID)
	if err == nil && !*force {
		fmt.Printf("Admin user '%s' already exists. Use --force to update credentials.\n", *usernameFlag)
		return
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	if existingID > 0 {
		_, err = db.DB.Exec(`
			UPDATE users 
			SET password_hash = ?, is_active = TRUE, deleted_at = NULL, token_version = token_version + 1, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, hash, existingID)
	} else {
		_, err = db.DB.Exec(`
			INSERT INTO users (tenant_id, username, password_hash, role, full_name, is_active)
			VALUES (1, ?, ?, 'admin', 'System Administrator', TRUE)
		`, *usernameFlag, hash)
	}

	if err != nil {
		log.Fatalf("save admin user: %v", err)
	}

	fmt.Println("Admin user created/updated successfully!")
	fmt.Printf("Username: %s\n", *usernameFlag)
	if generated {
		fmt.Printf("Generated password: %s\n", password)
		fmt.Println("PENTING: Simpan dan ubah password ini setelah login.")
	} else {
		fmt.Println("Password: [configured securely]")
	}
}
