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

// runMigrationsInTempDB changes into the repo root (so RunMigrations can find the
// relative migrations directory), opens a fresh SQLite database, and applies all
// migrations. It returns a cleanup function that restores the working directory
// and closes the database.
func runMigrationsInTempDB(t *testing.T) func() {
	t.Helper()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("change to repo root: %v", err)
	}

	databasePath := filepath.Join(t.TempDir(), "aether-test.db")
	DB, err = sql.Open("sqlite", databasePath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		os.Chdir(originalDir)
		t.Fatalf("open sqlite database: %v", err)
	}

	if err := RunMigrations(); err != nil {
		Close()
		os.Chdir(originalDir)
		t.Fatalf("run migrations: %v", err)
	}

	return func() {
		Close()
		os.Chdir(originalDir)
	}
}

// tableHasColumn reports whether the given table exposes the named column.
func tableHasColumn(t *testing.T, table, column string) bool {
	t.Helper()
	rows, err := DB.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column name: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}

// objectExists reports whether a sqlite_master object (table/index) exists by name.
func objectExists(t *testing.T, objType, name string) bool {
	t.Helper()
	var count int
	if err := DB.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?`, objType, name,
	).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master for %s %s: %v", objType, name, err)
	}
	return count > 0
}

// TestRunMigrationsIsIdempotentOnRerun verifies that applying all migrations a
// second time on an already-migrated database succeeds without error, matching
// the RunMigrations idempotency contract (Requirements 14.1, 14.5).
func TestRunMigrationsIsIdempotentOnRerun(t *testing.T) {
	cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	// Second run must also succeed (idempotent rerun).
	if err := RunMigrations(); err != nil {
		t.Fatalf("rerun migrations is not idempotent: %v", err)
	}
	// Third run for good measure.
	if err := RunMigrations(); err != nil {
		t.Fatalf("third migration run failed: %v", err)
	}
}

// TestSchedulingMigrationsCreateExpectedObjects verifies that migrations 020-025
// add the new tables, columns, and indexes for exam scheduling and iSpring
// delivery (Requirements 1.2, 2.1, 3.6, 4.1, 5.1, 7.2, 10.2, 14.1).
func TestSchedulingMigrationsCreateExpectedObjects(t *testing.T) {
	cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	// New tables.
	for _, table := range []string{
		"soal_package",
		"exam",
		"exam_session",
		"exam_session_kelas",
		"exam_session_ruang",
	} {
		if !objectExists(t, "table", table) {
			t.Errorf("expected table %q to exist after migrations", table)
		}
	}

	// New columns on existing tables.
	if !tableHasColumn(t, "kelas", "tingkat") {
		t.Error("expected kelas.tingkat column to exist")
	}
	for _, col := range []string{"session_id", "locked", "content_token"} {
		if !tableHasColumn(t, "cek_login", col) {
			t.Errorf("expected cek_login.%s column to exist", col)
		}
	}

	// New indexes, including the session-based unique index.
	for _, idx := range []string{
		"idx_kelas_tingkat",
		"idx_soal_package_tenant",
		"idx_exam_tenant",
		"idx_exam_session_token",
		"idx_session_kelas_session",
		"idx_session_ruang_session",
		"idx_cek_login_unique_session",
		"idx_cek_login_content_token",
	} {
		if !objectExists(t, "index", idx) {
			t.Errorf("expected index %q to exist after migrations", idx)
		}
	}
}

// TestSchedulingSchemaSupportsTenantScopedInserts performs a minimal end-to-end
// insert across the new scheduling tables to confirm foreign keys and columns
// line up as designed (Requirements 2.1, 4.1, 5.1).
func TestSchedulingSchemaSupportsTenantScopedInserts(t *testing.T) {
	cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	if _, err := DB.Exec(`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO kelas (id, tenant_id, nama_kelas, tingkat) VALUES (1, 1, 'XII IPA 1', 'XII')`); err != nil {
		t.Fatalf("seed kelas with tingkat: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO ruang (id, tenant_id, nama_ruang, username, password_hash) VALUES (1, 1, 'Ruang A', 'ruang_a', 'hash')`); err != nil {
		t.Fatalf("seed ruang: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO mapel (id, tenant_id, nama_mapel, kode_mapel) VALUES (1, 1, 'Kimia', 'KIM')`); err != nil {
		t.Fatalf("seed mapel: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO soal_package (id, tenant_id, nama, package_uuid) VALUES (1, 1, 'Kimia XII UAS', 'uuid-1')`); err != nil {
		t.Fatalf("seed soal_package: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO exam (id, tenant_id, mapel_id, tingkat, soal_package_id, durasi_menit, kkm) VALUES (1, 1, 1, 'XII', 1, 90, 70)`); err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO exam_session (id, tenant_id, exam_id, nama, waktu_mulai, waktu_selesai, token, status) VALUES (1, 1, 1, 'Sesi 1', '2026-06-01 08:00:00', '2026-06-01 10:00:00', 'TOKEN1', 'terjadwal')`); err != nil {
		t.Fatalf("seed exam_session: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (1, 1)`); err != nil {
		t.Fatalf("seed exam_session_kelas: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO exam_session_ruang (session_id, ruang_id) VALUES (1, 1)`); err != nil {
		t.Fatalf("seed exam_session_ruang: %v", err)
	}

	// The exam_session status CHECK constraint must reject invalid statuses.
	if _, err := DB.Exec(`INSERT INTO exam_session (tenant_id, exam_id, waktu_mulai, waktu_selesai, token, status) VALUES (1, 1, '2026-06-01 08:00:00', '2026-06-01 10:00:00', 'TOKEN2', 'invalid_status')`); err == nil {
		t.Error("expected exam_session status CHECK constraint to reject invalid status")
	}
}

// TestLegacyCekLoginConflictTargetStillWorks ensures the legacy mapel-based
// upsert path keeps functioning after the 025 migration, since StartExamSession
// is converted to the session-based path in a later task. This guards against
// breaking the running app mid-transition (Requirement 14.4).
func TestLegacyCekLoginConflictTargetStillWorks(t *testing.T) {
	cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	if !objectExists(t, "index", "idx_cek_login_unique_exam_session") {
		t.Fatal("legacy index idx_cek_login_unique_exam_session must remain until StartExamSession is migrated to sessions")
	}

	if _, err := DB.Exec(`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO kelas (id, tenant_id, nama_kelas) VALUES (1, 1, 'XII IPA')`); err != nil {
		t.Fatalf("seed kelas: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO ruang (id, tenant_id, nama_ruang, username, password_hash) VALUES (1, 1, 'Ruang A', 'ruang_a', 'hash')`); err != nil {
		t.Fatalf("seed ruang: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (1, 1, '2026001', 'siswa123', 'Siswa Tes', 1, 1)`); err != nil {
		t.Fatalf("seed peserta: %v", err)
	}

	upsert := `
		INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, attempt_token, login_time, last_activity)
		VALUES (1, 1, 1, 'tok-a', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, peserta_id, mapel_id) DO UPDATE SET
			attempt_token = excluded.attempt_token,
			login_time = CURRENT_TIMESTAMP,
			last_activity = CURRENT_TIMESTAMP`
	if _, err := DB.Exec(upsert); err != nil {
		t.Fatalf("first legacy upsert failed: %v", err)
	}
	if _, err := DB.Exec(upsert); err != nil {
		t.Fatalf("second legacy upsert (conflict path) failed: %v", err)
	}

	var count int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM cek_login WHERE tenant_id = 1 AND peserta_id = 1 AND mapel_id = 1`).Scan(&count); err != nil {
		t.Fatalf("count cek_login: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 cek_login row after idempotent upserts, got %d", count)
	}
}
