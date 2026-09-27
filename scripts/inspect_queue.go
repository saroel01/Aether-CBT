//go:build ignore
// +build ignore

package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
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

	dbPath := flag.String("db", defaultDB, "Path ke file database")
	flag.Parse()

	conn, err := sql.Open("sqlite", db.DSN(*dbPath))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := db.VerifyPragmas(conn); err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== submission_queue counts by status ===")
	rows, err := conn.Query("SELECT status, COUNT(*) FROM submission_queue GROUP BY status")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var s string
			var n int
			rows.Scan(&s, &n)
			fmt.Printf("  %-12s %d\n", s, n)
		}
	}

	fmt.Println("\n=== stuck rows (processing or failed) ===")
	rows2, err := conn.Query(`
		SELECT id, no_id, status, retry_count, COALESCE(last_error,''),
		       created_at, updated_at, next_retry_at
		FROM submission_queue
		WHERE status IN ('processing','failed')
		ORDER BY id LIMIT 20`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id int64
			var noID, status, errMsg, created, updated, next string
			var retry int
			rows2.Scan(&id, &noID, &status, &retry, &errMsg, &created, &updated, &next)
			fmt.Printf("  id=%d no_id=%s status=%s retry=%d created=%s updated=%s next=%s\n", id, noID, status, retry, created, updated, next)
			if errMsg != "" {
				if len(errMsg) > 100 {
					errMsg = errMsg[:100] + "..."
				}
				fmt.Printf("    last_error: %s\n", errMsg)
			}
		}
	}

	fmt.Println("\n=== failed_submissions count ===")
	var failed int
	conn.QueryRow("SELECT COUNT(*) FROM failed_submissions").Scan(&failed)
	fmt.Printf("  total: %d\n", failed)

	fmt.Println("\n=== current peserta with E2E* prefix and active sessions ===")
	var pesertaCount, sessionCount, hasilCount int
	conn.QueryRow("SELECT COUNT(*) FROM peserta WHERE no_id LIKE 'E2E%'").Scan(&pesertaCount)
	conn.QueryRow("SELECT COUNT(*) FROM cek_login WHERE peserta_id IN (SELECT id FROM peserta WHERE no_id LIKE 'E2E%')").Scan(&sessionCount)
	conn.QueryRow("SELECT COUNT(*) FROM hasil_tes WHERE peserta_id IN (SELECT id FROM peserta WHERE no_id LIKE 'E2E%')").Scan(&hasilCount)
	fmt.Printf("  E2E* peserta: %d\n  E2E* cek_login: %d\n  E2E* hasil_tes: %d\n", pesertaCount, sessionCount, hasilCount)
}
