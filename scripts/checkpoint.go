//go:build ignore
// +build ignore

package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/db"
)

func main() {
	defaultDB := os.Getenv("DATABASE_PATH")
	if defaultDB == "" {
		defaultDB = os.Getenv("DATABASE_URL")
	}
	if defaultDB == "" {
		defaultDB = "data/cbt_aether.db"
	}

	dbPath := flag.String("db", defaultDB, "Path ke file database sqlite")
	verifyOnly := flag.Bool("verify-only", false, "Hanya verifikasi integritas tanpa menjalankan checkpoint")
	flag.Parse()

	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		fmt.Printf("File database %s tidak ditemukan, tidak perlu checkpoint.\n", *dbPath)
		return
	}

	conn, err := sql.Open("sqlite", db.DSN(*dbPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal membuka database untuk checkpoint: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := db.VerifyPragmas(conn); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Pragma database tidak sesuai: %v\n", err)
		os.Exit(1)
	}

	var integrity string
	if err := conn.QueryRow("PRAGMA integrity_check;").Scan(&integrity); err != nil || integrity != "ok" {
		fmt.Fprintf(os.Stderr, "ERROR: Integritas database %s tidak valid: %v (integrity=%s)\n", *dbPath, err, integrity)
		os.Exit(1)
	}

	if *verifyOnly {
		fmt.Printf("✅ Integritas database %s terverifikasi (ok).\n", *dbPath)
		return
	}

	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal menjalankan PRAGMA wal_checkpoint(TRUNCATE): %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ WAL checkpoint (TRUNCATE) berhasil diselesaikan.")
}
