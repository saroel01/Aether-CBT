package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunMigrationsCreatesConflictTargetsUsedByExamFlow(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("change to repo root: %v", err)
	}
	defer os.Chdir(originalDir)

	databasePath := filepath.Join(t.TempDir(), "aether-test.db")
	DB, err = sql.Open("sqlite", databasePath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	defer Close()

	if err := RunMigrations(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	_, _ = DB.Exec(`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`)
	_, err = DB.Exec(`INSERT INTO kelas (tenant_id, nama_kelas) VALUES (1, 'XII IPA')`)
	if err != nil {
		t.Fatalf("seed kelas: %v", err)
	}
	_, err = DB.Exec(`INSERT INTO ruang (tenant_id, nama_ruang, username, password_hash) VALUES (1, 'Ruang A', 'ruang_a', 'hash')`)
	if err != nil {
		t.Fatalf("seed ruang: %v", err)
	}
	_, err = DB.Exec(`INSERT INTO mapel (tenant_id, nama_mapel, kode_mapel) VALUES (1, 'Matematika', 'MTK')`)
	if err != nil {
		t.Fatalf("seed mapel: %v", err)
	}
	_, err = DB.Exec(`INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (1, '2026001', 'siswa123', 'Siswa Tes', 1, 1)`)
	if err != nil {
		t.Fatalf("seed peserta: %v", err)
	}

	_, err = DB.Exec(`
		INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, login_time, last_activity)
		VALUES (1, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, peserta_id, mapel_id) DO UPDATE SET
			login_time = CURRENT_TIMESTAMP,
			last_activity = CURRENT_TIMESTAMP
	`)
	if err != nil {
		t.Fatalf("cek_login upsert conflict target is missing: %v", err)
	}

	_, err = DB.Exec(`
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, skor, skor_maks, detail_xml, status, validasi, waktu_selesai)
		VALUES (1, 1, 1, 80, 100, '', 'submitted', '1_2026001_1', CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, validasi) DO UPDATE SET
			skor = excluded.skor,
			skor_maks = excluded.skor_maks,
			detail_xml = excluded.detail_xml,
			status = 'submitted',
			waktu_selesai = CURRENT_TIMESTAMP
	`)
	if err != nil {
		t.Fatalf("hasil_tes upsert conflict target is missing: %v", err)
	}
}
