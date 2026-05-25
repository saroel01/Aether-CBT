package main

import (
	"fmt"
	"log"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

func main() {
	// Connect to database
	if err := db.Connect("data/cbt_aether.db"); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Ensure tables exist
	if err := db.RunMigrations(); err != nil {
		log.Fatal(err)
	}

	// Create admin user
	// PERINGATAN: Jangan pernah menggunakan password default ini di produksi!
	password := "admin123"
	hash, err := utils.HashPassword(password)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.DB.Exec(`
		INSERT OR REPLACE INTO users (tenant_id, username, password_hash, role, full_name, is_active)
		VALUES (1, 'admin', ?, 'admin', 'System Administrator', TRUE)
	`, hash)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Admin user created successfully!")
	fmt.Println("Username: admin")
	fmt.Println("Password: admin123")
}
