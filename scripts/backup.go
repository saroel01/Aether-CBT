//go:build ignore
// +build ignore

package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "data/cbt_aether.db", "Path ke file database utama")
	outDir := flag.String("out", "backups", "Folder untuk menyimpan backup")
	flag.Parse()

	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ERROR: Database tidak ditemukan: %s\n", *dbPath)
		os.Exit(1)
	}

	// Buat folder backup jika belum ada
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal membuat folder backup: %v\n", err)
		os.Exit(1)
	}

	// Buat nama file backup dengan timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(*outDir, fmt.Sprintf("cbt_aether_%s.db", timestamp))

	fmt.Printf("Memulai backup database...\n")
	fmt.Printf("  Source : %s\n", *dbPath)
	fmt.Printf("  Target : %s\n", backupFile)

	// Buka koneksi database (WAL mode sudah di-set di connection string aplikasi)
	dsn := *dbPath + "?_journal_mode=WAL&_foreign_keys=on"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal membuka database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Gunakan VACUUM INTO untuk backup atomik (paling aman untuk WAL)
	_, err = db.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal melakukan VACUUM INTO: %v\n", err)
		os.Exit(1)
	}

	// Verifikasi integritas backup
	var integrity string
	err = db.QueryRow(fmt.Sprintf("PRAGMA integrity_check;")).Scan(&integrity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal cek integritas backup: %v\n", err)
		os.Exit(1)
	}

	if integrity != "ok" {
		fmt.Fprintf(os.Stderr, "ERROR: Backup corrupt! Integrity check: %s\n", integrity)
		os.Remove(backupFile) // hapus file rusak
		os.Exit(1)
	}

	// Cek ukuran file backup
	info, _ := os.Stat(backupFile)
	fmt.Printf("\n✅ Backup berhasil!\n")
	fmt.Printf("   File     : %s\n", backupFile)
	fmt.Printf("   Ukuran   : %.2f MB\n", float64(info.Size())/1024/1024)
	fmt.Printf("   Integrity: %s\n", integrity)
}
