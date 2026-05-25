package main

import (
	"fmt"
	"log"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

func main() {
	if err := db.Connect("data/cbt_aether.db"); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.RunMigrations(); err != nil {
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

	// Get IDs for relations
	var kelas1, kelas2, ruangA, ruangB int
	db.DB.QueryRow(`SELECT id FROM kelas WHERE tenant_id = 1 AND nama_kelas = 'XII IPA 1'`).Scan(&kelas1)
	db.DB.QueryRow(`SELECT id FROM kelas WHERE tenant_id = 1 AND nama_kelas = 'XII IPA 2'`).Scan(&kelas2)
	db.DB.QueryRow(`SELECT id FROM ruang WHERE tenant_id = 1 AND nama_ruang = 'Ruang A'`).Scan(&ruangA)
	db.DB.QueryRow(`SELECT id FROM ruang WHERE tenant_id = 1 AND nama_ruang = 'Ruang B'`).Scan(&ruangB)

	// Sample Students (password is plaintext for easy student login in this version)
	students := []struct {
		no_id, nama string
		kelas, ruang int
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
	for _, st := range students {
		_, _ = db.DB.Exec(`
			INSERT OR IGNORE INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
			VALUES (?, ?, ?, ?, ?, ?)
		`, tenantID, st.no_id, "siswa123", st.nama, st.kelas, st.ruang)
	}

	// Ensure settings token exists
	_, _ = db.DB.Exec(`
		INSERT OR IGNORE INTO settings (tenant_id, exam_title, token, is_exam_active)
		VALUES (1, 'Ujian Akhir Semester 2025/2026', 'ujian2026', TRUE)
	`)

	fmt.Println("✅ Sample data seeded successfully for tenant 'default'")
	fmt.Println("   - 3 Classes")
	fmt.Println("   - 4 Subjects")
	fmt.Println("   - 2 Rooms (supervisor login: ruang_a / ruang123)")
	fmt.Println("   - 8 Students (student login: no_id + password 'siswa123')")
	fmt.Println("   - Settings with exam token 'ujian2026'")
}
