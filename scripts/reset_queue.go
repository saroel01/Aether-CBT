//go:build ignore
// +build ignore

package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

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

	dbPath := flag.String("db", defaultDB, "Path ke file database")
	yes := flag.Bool("yes", false, "Lewati prompt konfirmasi (sama dengan --force)")
	force := flag.Bool("force", false, "Lewati prompt konfirmasi")
	flag.BoolVar(yes, "y", false, "Lewati prompt konfirmasi (shorthand)")
	flag.BoolVar(force, "f", false, "Lewati prompt konfirmasi (shorthand)")
	tenantID := flag.Int("tenant", 0, "ID tenant yang direset (wajib, > 0)")
	flag.Parse()
	// L8: never wipe every tenant's queue; the operator must name one tenant.
	if *tenantID <= 0 {
		log.Fatal("ERROR: flag -tenant wajib diisi dengan ID tenant (> 0)")
	}

	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("ERROR: Database tidak ditemukan: %s", *dbPath)
	}

	// P1-16: Konfirmasi sebelum melakukan reset kecuali jika diberikan flag --yes atau --force
	if !*yes && !*force {
		fmt.Printf("PERINGATAN: Operasi ini akan mengosongkan antrean submission dan menghapus data peserta uji tenant %d pada %s.\n", *tenantID, *dbPath)
		fmt.Print("Apakah Anda yakin ingin melanjutkan? (ketik 'yes' untuk konfirmasi): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("ERROR: Gagal membaca input: %v", err)
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "yes" && input != "y" {
			fmt.Println("Operasi reset dibatalkan.")
			os.Exit(1)
		}
	}

	// Buka koneksi database menggunakan DSN standar ber-pragma modern (P1-15)
	database, err := sql.Open("sqlite", db.DSN(*dbPath))
	if err != nil {
		log.Fatalf("ERROR: Gagal membuka database: %v", err)
	}
	defer database.Close()

	if err := db.VerifyPragmas(database); err != nil {
		log.Fatalf("ERROR: Pragma database tidak sesuai: %v", err)
	}

	// Every statement is scoped to the requested tenant (L8); ?1 is the tenant id.
	const testPeserta = `SELECT id FROM peserta
			WHERE tenant_id = ?1 AND (no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
			   OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%')`
	statements := []string{
		`DELETE FROM submission_queue WHERE tenant_id = ?1`,
		`DELETE FROM failed_submissions WHERE tenant_id = ?1`,
		`DELETE FROM hasil_tes_detail WHERE hasil_tes_id IN (
			SELECT id FROM hasil_tes WHERE tenant_id = ?1 AND peserta_id IN (` + testPeserta + `))`,
		`DELETE FROM hasil_tes WHERE tenant_id = ?1 AND peserta_id IN (` + testPeserta + `)`,
		`DELETE FROM cek_login WHERE tenant_id = ?1 AND peserta_id IN (` + testPeserta + `)`,
		`DELETE FROM peserta WHERE id IN (` + testPeserta + `)`,
	}

	for _, s := range statements {
		res, err := database.Exec(s, *tenantID)
		if err != nil {
			log.Fatalf("ERROR: Gagal mengeksekusi statement: %v (sql=%s)", err, s[:min(60, len(s))])
		}
		if res != nil {
			n, _ := res.RowsAffected()
			fmt.Printf("ok: %d rows  (%s...)\n", n, s[:min(50, len(s))])
		}
	}
	if _, err := database.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		log.Printf("WARNING: wal_checkpoint gagal: %v", err)
	}
	fmt.Println("Reset complete.")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
