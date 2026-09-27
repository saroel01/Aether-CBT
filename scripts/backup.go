//go:build ignore
// +build ignore

package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/db"
)

func cleanupBackupFiles(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
}

func main() {
	defaultDB := os.Getenv("DATABASE_PATH")
	if defaultDB == "" {
		defaultDB = os.Getenv("DATABASE_URL")
	}
	if defaultDB == "" {
		defaultDB = "data/cbt_aether.db"
	}

	dbPath := flag.String("db", defaultDB, "Path ke file database utama")
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

	// Buka koneksi database sumber menggunakan format DSN standar (P1-15)
	sourceDB, err := sql.Open("sqlite", db.DSN(*dbPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal membuka database sumber: %v\n", err)
		os.Exit(1)
	}
	defer sourceDB.Close()

	if err := db.VerifyPragmas(sourceDB); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Pragma database sumber tidak sesuai: %v\n", err)
		os.Exit(1)
	}

	// P0-5: Periksa integritas database sumber sebagai observabilitas awal.
	// PENTING: Jangan gagalkan/hapus backup jika pemeriksaan sumber melaporkan warning,
	// karena backup snapshot mungkin tetap dapat diselamatkan atau dianalisis (P0-5).
	var sourceIntegrity string
	if err := sourceDB.QueryRow("PRAGMA integrity_check;").Scan(&sourceIntegrity); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Pemeriksaan integritas sumber gagal dijalankan: %v (backup tetap dilanjutkan)\n", err)
	} else if sourceIntegrity != "ok" {
		fmt.Fprintf(os.Stderr, "WARNING: Integritas database sumber melaporkan masalah: %s (backup tetap dilanjutkan)\n", sourceIntegrity)
	}

	// Gunakan VACUUM INTO untuk backup atomik (paling aman untuk WAL)
	targetPath := filepath.ToSlash(backupFile)
	targetPath = strings.ReplaceAll(targetPath, "'", "''")
	_, err = sourceDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", targetPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal melakukan VACUUM INTO: %v\n", err)
		os.Exit(1)
	}

	// P0-5 & P1-15: Verifikasi integritas dan relasi foreign key pada FILE BACKUP (bukan database sumber)
	backupDB, err := sql.Open("sqlite", db.DSN(backupFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal membuka file backup untuk verifikasi: %v\n", err)
		cleanupBackupFiles(backupFile)
		os.Exit(1)
	}

	if err := db.VerifyPragmas(backupDB); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Pragma file backup tidak sesuai: %v\n", err)
		backupDB.Close()
		cleanupBackupFiles(backupFile)
		os.Exit(1)
	}

	// 1. Jalankan PRAGMA integrity_check pada database backup
	var integrity string
	if err := backupDB.QueryRow("PRAGMA integrity_check;").Scan(&integrity); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal cek integritas backup: %v\n", err)
		backupDB.Close()
		cleanupBackupFiles(backupFile)
		os.Exit(1)
	}

	if integrity != "ok" {
		fmt.Fprintf(os.Stderr, "ERROR: Backup corrupt! Integrity check: %s\n", integrity)
		backupDB.Close()
		cleanupBackupFiles(backupFile)
		os.Exit(1)
	}

	// 2. Jalankan PRAGMA foreign_key_check pada database backup
	fkRows, err := backupDB.Query("PRAGMA foreign_key_check;")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal cek foreign keys backup: %v\n", err)
		backupDB.Close()
		cleanupBackupFiles(backupFile)
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
		fmt.Fprintf(os.Stderr, "ERROR: File backup memiliki %d pelanggaran foreign key!\n", fkViolations)
		backupDB.Close()
		cleanupBackupFiles(backupFile)
		os.Exit(1)
	}

	// Checkpoint WAL dan tutup koneksi backupDB agar backup menjadi file mandiri yang bersih
	_, _ = backupDB.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
	backupDB.Close()
	_ = os.Remove(backupFile + "-wal")
	_ = os.Remove(backupFile + "-shm")

	// Cek ukuran file backup
	info, err := os.Stat(backupFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Gagal membaca info file backup: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Backup berhasil dan terverifikasi!\n")
	fmt.Printf("   File       : %s\n", backupFile)
	fmt.Printf("   Ukuran     : %.2f MB\n", float64(info.Size())/1024/1024)
	fmt.Printf("   Integrity  : %s (diverifikasi pada file backup)\n", integrity)
	fmt.Printf("   Foreign Key: ok (0 violations)\n")
}
