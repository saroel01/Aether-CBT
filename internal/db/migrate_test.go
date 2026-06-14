package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// migrationsDir returns the absolute path to the migrations directory. The Go test
// runner executes package tests with the working directory set to the package
// directory (internal/db), so the repository root is two levels up. Resolving to an
// absolute path lets RunMigrations execute without os.Chdir, so tests can run in
// parallel without clobbering process-wide working-directory state (Requirement 16.7).
func migrationsDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "internal", "db", "migrations"))
	if err != nil {
		t.Fatalf("resolve migrations dir: %v", err)
	}
	return abs
}

// newTestDB opens a fresh in-process SQLite database for a single test. Each test owns
// its own *sql.DB; the package-global DB is never mutated (Requirement 16.7).
func newTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "aether-test.db")
	testDB, err := sql.Open("sqlite", databasePath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	return testDB, func() { testDB.Close() }
}

// runMigrationsInTempDB opens a fresh database and applies all migrations against it,
// returning the per-test *sql.DB together with a cleanup. It neither touches the
// package-global DB nor changes the process working directory (Requirement 16.7).
func runMigrationsInTempDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	testDB, cleanup := newTestDB(t)
	if err := RunMigrations(testDB, migrationsDir(t)); err != nil {
		cleanup()
		t.Fatalf("run migrations: %v", err)
	}
	return testDB, cleanup
}

// tableHasColumn reports whether the given table exposes the named column.
func tableHasColumn(t *testing.T, database *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := database.Query(`SELECT name FROM pragma_table_info(?)`, table)
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
func objectExists(t *testing.T, database *sql.DB, objType, name string) bool {
	t.Helper()
	var count int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?`, objType, name,
	).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master for %s %s: %v", objType, name, err)
	}
	return count > 0
}

func TestRunMigrationsCreatesConflictTargetsUsedByExamFlow(t *testing.T) {
	testDB, cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	_, err := testDB.Exec(`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO kelas (tenant_id, nama_kelas) VALUES (1, 'XII IPA')`); err != nil {
		t.Fatalf("seed kelas: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO ruang (tenant_id, nama_ruang, username, password_hash) VALUES (1, 'Ruang A', 'ruang_a', 'hash')`); err != nil {
		t.Fatalf("seed ruang: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO mapel (tenant_id, nama_mapel, kode_mapel) VALUES (1, 'Matematika', 'MTK')`); err != nil {
		t.Fatalf("seed mapel: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (1, '2026001', 'siswa123', 'Siswa Tes', 1, 1)`); err != nil {
		t.Fatalf("seed peserta: %v", err)
	}

	_, err = testDB.Exec(`
		INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, login_time, last_activity)
		VALUES (1, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, peserta_id, mapel_id) DO UPDATE SET
			login_time = CURRENT_TIMESTAMP,
			last_activity = CURRENT_TIMESTAMP
	`)
	if err != nil {
		t.Fatalf("cek_login upsert conflict target is missing: %v", err)
	}

	_, err = testDB.Exec(`
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

// TestRunMigrationsIsIdempotentOnRerun verifies that applying all migrations a second
// and third time on an already-migrated database succeeds without error AND leaves the
// full scheduling schema (migrations 020-025) intact. Asserting object presence — not
// merely the absence of error — guards against a partially applied migration silently
// self-reporting success while leaving the schema incomplete (Requirements 14.1, 14.5,
// 14.7).
func TestRunMigrationsIsIdempotentOnRerun(t *testing.T) {
	testDB, cleanup := runMigrationsInTempDB(t)
	defer cleanup()
	dir := migrationsDir(t)

	// Second run must succeed (idempotent rerun) ...
	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("rerun migrations is not idempotent: %v", err)
	}
	// ... and every schema object from migrations 020-025 must still be present.
	assertSchedulingObjectsExist(t, testDB)

	// Third run for good measure, again asserting the full object set survives.
	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("third migration run failed: %v", err)
	}
	assertSchedulingObjectsExist(t, testDB)
}

// assertSchedulingObjectsExist asserts that migrations 020-025 created the scheduling
// tables, the columns added to existing tables, and the supporting indexes — including
// the session-based unique index idx_cek_login_unique_session and the content-token
// lookup index idx_cek_login_content_token (Requirements 1.2, 2.1, 3.6, 4.1, 5.1, 7.2,
// 10.2, 14.1, 14.7). Shared between the schema-object test and the idempotency-rerun
// test so both assert the same comprehensive set rather than drifting apart.
func assertSchedulingObjectsExist(t *testing.T, database *sql.DB) {
	t.Helper()

	for _, table := range []string{
		"soal_package",
		"exam",
		"exam_session",
		"exam_session_kelas",
		"exam_session_ruang",
	} {
		if !objectExists(t, database, "table", table) {
			t.Errorf("expected table %q to exist after migrations", table)
		}
	}

	if !tableHasColumn(t, database, "kelas", "tingkat") {
		t.Error("expected kelas.tingkat column to exist")
	}
	for _, col := range []string{"session_id", "locked", "content_token"} {
		if !tableHasColumn(t, database, "cek_login", col) {
			t.Errorf("expected cek_login.%s column to exist", col)
		}
	}

	for _, idx := range []string{
		"idx_kelas_tingkat",
		"idx_soal_package_tenant",
		"idx_exam_tenant",
		"idx_exam_mapel",
		"idx_exam_session_tenant",
		"idx_exam_session_exam",
		"idx_exam_session_token",
		"idx_session_kelas_session",
		"idx_session_ruang_session",
		"idx_cek_login_unique_session",
		"idx_cek_login_content_token",
	} {
		if !objectExists(t, database, "index", idx) {
			t.Errorf("expected index %q to exist after migrations", idx)
		}
	}
}

// TestSchedulingMigrationsCreateExpectedObjects verifies that migrations 020-025
// add the new tables, columns, and indexes for exam scheduling and iSpring
// delivery (Requirements 1.2, 2.1, 3.6, 4.1, 5.1, 7.2, 10.2, 14.1).
func TestSchedulingMigrationsCreateExpectedObjects(t *testing.T) {
	testDB, cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	assertSchedulingObjectsExist(t, testDB)
}

// TestSchedulingSchemaSupportsTenantScopedInserts performs a minimal end-to-end
// insert across the new scheduling tables to confirm foreign keys and columns
// line up as designed (Requirements 2.1, 4.1, 5.1).
func TestSchedulingSchemaSupportsTenantScopedInserts(t *testing.T) {
	testDB, cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	if _, err := testDB.Exec(`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO kelas (id, tenant_id, nama_kelas, tingkat) VALUES (1, 1, 'XII IPA 1', 'XII')`); err != nil {
		t.Fatalf("seed kelas with tingkat: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO ruang (id, tenant_id, nama_ruang, username, password_hash) VALUES (1, 1, 'Ruang A', 'ruang_a', 'hash')`); err != nil {
		t.Fatalf("seed ruang: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO mapel (id, tenant_id, nama_mapel, kode_mapel) VALUES (1, 1, 'Kimia', 'KIM')`); err != nil {
		t.Fatalf("seed mapel: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO soal_package (id, tenant_id, nama, package_uuid) VALUES (1, 1, 'Kimia XII Uas', 'uuid-1')`); err != nil {
		t.Fatalf("seed soal_package: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO exam (id, tenant_id, mapel_id, tingkat, soal_package_id, durasi_menit, kkm) VALUES (1, 1, 1, 'XII', 1, 90, 70)`); err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO exam_session (id, tenant_id, exam_id, nama, waktu_mulai, waktu_selesai, token, status) VALUES (1, 1, 1, 'Sesi 1', '2026-06-01 08:00:00', '2026-06-01 10:00:00', 'TOKEN1', 'terjadwal')`); err != nil {
		t.Fatalf("seed exam_session: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (1, 1)`); err != nil {
		t.Fatalf("seed exam_session_kelas: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO exam_session_ruang (session_id, ruang_id) VALUES (1, 1)`); err != nil {
		t.Fatalf("seed exam_session_ruang: %v", err)
	}

	// The exam_session status CHECK constraint must reject invalid statuses.
	if _, err := testDB.Exec(`INSERT INTO exam_session (tenant_id, exam_id, waktu_mulai, waktu_selesai, token, status) VALUES (1, 1, '2026-06-01 08:00:00', '2026-06-01 10:00:00', 'TOKEN2', 'invalid_status')`); err == nil {
		t.Error("expected exam_session status CHECK constraint to reject invalid status")
	}
}

// TestLegacyCekLoginConflictTargetStillWorks ensures the legacy mapel-based
// upsert path keeps functioning after the 025 migration, since StartExamSession
// is converted to the session-based path in a later task. This guards against
// breaking the running app mid-transition (Requirement 14.4).
func TestLegacyCekLoginConflictTargetStillWorks(t *testing.T) {
	testDB, cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	if !objectExists(t, testDB, "index", "idx_cek_login_unique_exam_session") {
		t.Fatal("legacy index idx_cek_login_unique_exam_session must remain until StartExamSession is migrated to sessions")
	}

	if _, err := testDB.Exec(`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO kelas (id, tenant_id, nama_kelas) VALUES (1, 1, 'XII IPA')`); err != nil {
		t.Fatalf("seed kelas: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO ruang (id, tenant_id, nama_ruang, username, password_hash) VALUES (1, 1, 'Ruang A', 'ruang_a', 'hash')`); err != nil {
		t.Fatalf("seed ruang: %v", err)
	}
	if _, err := testDB.Exec(`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (1, 1, '2026001', 'siswa123', 'Siswa Tes', 1, 1)`); err != nil {
		t.Fatalf("seed peserta: %v", err)
	}

	upsert := `
		INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, attempt_token, login_time, last_activity)
		VALUES (1, 1, 1, 'tok-a', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, peserta_id, mapel_id) DO UPDATE SET
			attempt_token = excluded.attempt_token,
			login_time = CURRENT_TIMESTAMP,
			last_activity = CURRENT_TIMESTAMP`
	if _, err := testDB.Exec(upsert); err != nil {
		t.Fatalf("first legacy upsert failed: %v", err)
	}
	if _, err := testDB.Exec(upsert); err != nil {
		t.Fatalf("second legacy upsert (conflict path) failed: %v", err)
	}

	var count int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM cek_login WHERE tenant_id = 1 AND peserta_id = 1 AND mapel_id = 1`).Scan(&count); err != nil {
		t.Fatalf("count cek_login: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 cek_login row after idempotent upserts, got %d", count)
	}
}

// TestRunMigrationsSelfHealsPartiallyAppliedMigration verifies that when a migration
// file was applied only partially — its first statement ran but a later statement did
// not (e.g. an interrupted startup, or a column added manually without the companion
// index) — re-running RunMigrations completes the missing statements instead of
// aborting the whole file at the already-applied statement (Requirement 14.6, AD-8).
//
// With per-file execution the duplicate-column ALTER aborts the file and the
// independent CREATE INDEX never runs, leaving the schema silently incomplete. With
// per-statement execution the ALTER error is swallowed per-statement and the index is
// created — self-healing.
func TestRunMigrationsSelfHealsPartiallyAppliedMigration(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "001_setup.sql"),
		[]byte("CREATE TABLE IF NOT EXISTS selfheal_t (id INTEGER, tenant_id INTEGER);\n"),
		0o644); err != nil {
		t.Fatalf("write setup migration: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_selfheal.sql"), []byte(
		"-- idempotency-fragile ALTER followed by an independent CREATE INDEX\n"+
			"ALTER TABLE selfheal_t ADD COLUMN c TEXT;\n"+
			"CREATE INDEX IF NOT EXISTS idx_selfheal_t_c ON selfheal_t(tenant_id, c);\n",
	), 0o644); err != nil {
		t.Fatalf("write selfheal migration: %v", err)
	}

	testDB, cleanup := newTestDB(t)
	defer cleanup()

	// Simulate a partially-applied state: statement 1 of 002 (the ALTER) already ran,
	// but statement 2 (the index) did not. The table exists with the column but the
	// index is absent.
	if _, err := testDB.Exec(`CREATE TABLE selfheal_t (id INTEGER, tenant_id INTEGER, c TEXT)`); err != nil {
		t.Fatalf("pre-apply partial state: %v", err)
	}
	if objectExists(t, testDB, "index", "idx_selfheal_t_c") {
		t.Fatal("precondition: idx_selfheal_t_c must not exist before rerun")
	}

	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}

	if !objectExists(t, testDB, "index", "idx_selfheal_t_c") {
		t.Fatal("expected idx_selfheal_t_c to be created on rerun (self-healing); migration is not executed per-statement")
	}
	if !tableHasColumn(t, testDB, "selfheal_t", "c") {
		t.Error("expected selfheal_t.c to remain present after self-healing rerun")
	}
}
