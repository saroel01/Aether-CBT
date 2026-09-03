package main

import (
	"fmt"
	"log"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

func main() {
	if err := db.Connect("data/cbt_aether.db", db.DefaultPoolConfig()); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.RunMigrations(db.DB, "internal/db/migrations"); err != nil {
		log.Fatal(err)
	}

	tenantID := 1

	// Sample Classes
	classes := []string{"XII IPA 1", "XII IPA 2", "XII IPS 1"}
	for _, c := range classes {
		_, _ = db.DB.Exec(`INSERT OR IGNORE INTO kelas (tenant_id, nama_kelas) VALUES (?, ?)`, tenantID, c)
	}

	// Sample Subjects
	subjects := []struct {
		nama string
		kode string
	}{
		{"Matematika", "MTK"},
		{"Bahasa Indonesia", "BID"},
		{"Bahasa Inggris", "BIG"},
		{"Fisika", "FIS"},
	}
	for _, s := range subjects {
		_, _ = db.DB.Exec(`INSERT OR IGNORE INTO mapel (tenant_id, nama_mapel, kode_mapel) VALUES (?, ?, ?)`,
			tenantID, s.nama, s.kode)
	}

	// Sample Rooms + hashed password for room supervisor
	// PERINGATAN: Password di bawah ini hanya untuk data contoh.
	// WAJIB diganti sebelum digunakan di lingkungan nyata (lihat docs/credential-rotation.md)
	rooms := []struct {
		nama, user, pass string
	}{
		{"Ruang A", "ruang_a", "ruang123"},
		{"Ruang B", "ruang_b", "ruang123"},
	}
	for _, r := range rooms {
		hash, _ := utils.HashPassword(r.pass)
		_, _ = db.DB.Exec(`INSERT OR IGNORE INTO ruang (tenant_id, nama_ruang, username, password_hash) VALUES (?, ?, ?, ?)`,
			tenantID, r.nama, r.user, hash)
	}

	// Get IDs for relations. A failed lookup used to leave these at 0 and that 0 was written
	// straight into peserta.kelas_id / peserta.ruang_id, which are FOREIGN KEY columns with no
	// parent row at id 0 (clause 2.2). Fail loudly instead: the seed inserted these rows a few
	// lines above, so a missing id means something is genuinely wrong.
	kelas1 := mustLookupID(`SELECT id FROM kelas WHERE tenant_id = 1 AND nama_kelas = 'XII IPA 1'`)
	kelas2 := mustLookupID(`SELECT id FROM kelas WHERE tenant_id = 1 AND nama_kelas = 'XII IPA 2'`)
	ruangA := mustLookupID(`SELECT id FROM ruang WHERE tenant_id = 1 AND nama_ruang = 'Ruang A'`)
	ruangB := mustLookupID(`SELECT id FROM ruang WHERE tenant_id = 1 AND nama_ruang = 'Ruang B'`)

	// Sample Students. Password is bcrypt-hashed (Task 6 removed the plaintext fallback, so a
	// plaintext seed would never authenticate). The default password is "siswa123" for all
	// sample students; change per-student before production use.
	students := []struct {
		no_id, nama  string
		kelas, ruang int64
	}{
		{"2024001", "Ahmad Fauzi", kelas1, ruangA},
		{"2024002", "Siti Nurhaliza", kelas1, ruangA},
		{"2024003", "Budi Santoso", kelas1, ruangA},
		{"2024004", "Dewi Lestari", kelas2, ruangB},
		{"2024005", "Rizki Ramadhan", kelas2, ruangB},
		{"2024006", "Putri Ayu", kelas2, ruangB},
		{"2024007", "Andi Wijaya", kelas1, ruangB},
		{"2024008", "Maya Putri", kelas2, ruangA},
	}
	studentPWHash, err := utils.HashPassword("siswa123")
	if err != nil {
		log.Fatalf("hash seed student password: %v", err)
	}
	for _, st := range students {
		// db.NullableFK maps a non-positive id to SQL NULL so the seed can never write the
		// sentinel 0 into a FOREIGN KEY column (clause 2.2).
		if _, err := db.DB.Exec(`
			INSERT OR IGNORE INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
			VALUES (?, ?, ?, ?, ?, ?)
		`, tenantID, st.no_id, studentPWHash, st.nama, db.NullableFK(st.kelas), db.NullableFK(st.ruang)); err != nil {
			log.Fatalf("seed peserta %s: %v", st.no_id, err)
		}
	}

	// Ensure settings token exists. A random per-tenant token is generated so the seed is
	// not a known constant (review H22, Task 18). The generated token is printed below.
	token, _ := utils.GenerateSecureToken(16)
	_, _ = db.DB.Exec(`
		INSERT OR IGNORE INTO settings (tenant_id, exam_title, token, is_exam_active)
		VALUES (1, 'Ujian Akhir Semester 2025/2026', ?, TRUE)
	`, token)
	// If a row already existed (INSERT OR IGNORE no-op), read its token so we print the truth.
	var actualToken string
	_ = db.DB.QueryRow(`SELECT token FROM settings WHERE tenant_id = 1`).Scan(&actualToken)
	if actualToken == "" {
		actualToken = token
	}

	fmt.Println("✅ Sample data seeded successfully for tenant 'default'")
	fmt.Println("   - 3 Classes")
	fmt.Println("   - 4 Subjects")
	fmt.Println("   - 2 Rooms (supervisor login: ruang_a / ruang123)")
	fmt.Println("   - 8 Students (student login: no_id + password 'siswa123')")
	fmt.Printf("   - Settings with random exam token '%s'\n", actualToken)
}

// mustLookupID reads a single id, aborting the seed when the row is missing. Swallowing the
// lookup error is what let the sentinel 0 reach a FOREIGN KEY column (clause 2.2).
func mustLookupID(query string) int64 {
	var id int64
	if err := db.DB.QueryRow(query).Scan(&id); err != nil {
		log.Fatalf("seed lookup failed (%s): %v", query, err)
	}
	if id <= 0 {
		log.Fatalf("seed lookup returned a non-positive id (%s)", query)
	}
	return id
}
