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
		if *verifyOnly {
			fmt.Fprintf(os.Stderr, "ERROR: File database %s tidak ditemukan.\n", *dbPath)
			os.Exit(1)
		}
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

	fkRows, err := conn.Query("PRAGMA foreign_key_check;")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal cek foreign keys %s: %v\n", *dbPath, err)
		os.Exit(1)
	}
	fkViolations := 0
	for fkRows.Next() {
		fkViolations++
		var table, parent string
		var rowid, fkid int64
		_ = fkRows.Scan(&table, &rowid, &parent, &fkid)
		fmt.Fprintf(os.Stderr, "  FK violation: table=%s, rowid=%d, target=%s\n", table, rowid, parent)
	}
	fkRows.Close()

	if fkViolations > 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Database %s memiliki %d pelanggaran foreign key!\n", *dbPath, fkViolations)
		os.Exit(1)
	}

	if *verifyOnly {
		fmt.Printf("✅ Integritas database %s terverifikasi (ok).\n", *dbPath)
		fmt.Println("   Foreign Key: ok (0 violations)")
		return
	}

	if _, err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal menjalankan PRAGMA wal_checkpoint(TRUNCATE): %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ WAL checkpoint (TRUNCATE) berhasil diselesaikan.")
}
