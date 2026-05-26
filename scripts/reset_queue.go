//go:build ignore
// +build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "data/cbt_aether.db?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`DELETE FROM submission_queue`,
		`DELETE FROM failed_submissions`,
		`DELETE FROM hasil_tes_detail WHERE hasil_tes_id IN (
			SELECT id FROM hasil_tes WHERE peserta_id IN (
				SELECT id FROM peserta
				WHERE no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
				   OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%'
			)
		)`,
		`DELETE FROM hasil_tes WHERE peserta_id IN (
			SELECT id FROM peserta
			WHERE no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
			   OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%'
		)`,
		`DELETE FROM cek_login WHERE peserta_id IN (
			SELECT id FROM peserta
			WHERE no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
			   OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%'
		)`,
		`DELETE FROM peserta WHERE no_id LIKE 'E2E%' OR no_id LIKE 'WB%' OR no_id LIKE 'LT%'
			OR no_id LIKE 'LB%' OR no_id LIKE 'SB%' OR no_id LIKE 'DX%' OR no_id LIKE 'FC%'`,
		`PRAGMA wal_checkpoint(TRUNCATE)`,
	}

	for _, s := range statements {
		res, err := db.Exec(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: %v (sql=%s)\n", err, s[:min(60, len(s))])
			continue
		}
		if res != nil {
			n, _ := res.RowsAffected()
			fmt.Printf("ok: %d rows  (%s...)\n", n, s[:min(50, len(s))])
		}
	}
	fmt.Println("Reset complete.")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
