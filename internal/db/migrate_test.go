package db

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// newTestDB opens a fresh in-process SQLite database for a single test, using the same DSN as
// production (db.DSN) so migrations are exercised with WAL and FOREIGN KEY enforcement actually
// on — see codebase-bug-sweep clause 2.1. Each test owns its own *sql.DB; the package-global DB
// is never mutated (Requirement 16.7).
func newTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "aether-test.db")
	testDB, err := sql.Open("sqlite", DSN(databasePath))
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := VerifyPragmas(testDB); err != nil {
		testDB.Close()
		t.Fatalf("test database does not have the required pragmas: %v", err)
	}
	return testDB, func() { testDB.Close() }
}

// newLegacyTestDB opens a database with FOREIGN KEY enforcement OFF, which is the only way to
// fabricate the referential violations that legacy installations really contain: those rows were
// written by a build whose pragmas were silently dropped (clause 1.1), and with enforcement on
// they cannot be inserted at all. Tests that assert repair or diagnostics on dirty data need
// this; nothing else should use it.
func newLegacyTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "aether-legacy-test.db")
	testDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	return testDB, func() { testDB.Close() }
}

// runMigrationsInLegacyTempDB is runMigrationsInTempDB against a connection with FOREIGN KEY
// enforcement off (see newLegacyTestDB).
func runMigrationsInLegacyTempDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	testDB, cleanup := newLegacyTestDB(t)
	if err := RunMigrations(testDB, migrationsDir(t)); err != nil {
		cleanup()
		t.Fatalf("run migrations: %v", err)
	}
	return testDB, cleanup
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

// assertSchedulingObjectsExist asserts that migrations 020-026 created the scheduling
// tables, the columns added to existing tables, and the supporting indexes — including
// the session-based unique index idx_cek_login_unique_session, the content-token lookup
// index idx_cek_login_content_token, and the unique capability-key index
// idx_cek_login_content_token_unique (Requirements 1.2, 2.1, 3.6, 4.1, 5.1, 7.2, 8.1,
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
		"idx_cek_login_content_token_unique",
	} {
		if !objectExists(t, database, "index", idx) {
			t.Errorf("expected index %q to exist after migrations", idx)
		}
	}
}

// TestSchedulingMigrationsCreateExpectedObjects verifies that migrations 020-026
// add the new tables, columns, and indexes for exam scheduling and iSpring
// delivery (Requirements 1.2, 2.1, 3.6, 4.1, 5.1, 7.2, 8.1, 10.2, 14.1).
func TestSchedulingMigrationsCreateExpectedObjects(t *testing.T) {
	testDB, cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	assertSchedulingObjectsExist(t, testDB)
}

// TestContentTokenUniqueIndexEnforcesOnePerSession verifies migration 026's unique partial
// index: a content_token (the capability key for content serving) is held by at most one
// cek_login row, while NULL tokens do not conflict (sessions that have not started content).
// This is the data-layer invariant behind the cookie-authorized content-serving path.
func TestContentTokenUniqueIndexEnforcesOnePerSession(t *testing.T) {
	testDB, cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	// Parent rows required by cek_login FKs (peserta -> kelas/ruang -> tenant).
	for _, q := range []string{
		`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`,
		`INSERT INTO kelas (id, tenant_id, nama_kelas) VALUES (1, 1, 'XII IPA 1')`,
		`INSERT INTO ruang (id, tenant_id, nama_ruang, username, password_hash) VALUES (1, 1, 'Ruang A', 'ruang_a', 'hash')`,
		`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (1, 1, 'p1', 'x', 'P1', 1, 1)`,
	} {
		if _, err := testDB.Exec(q); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	insert := func(id int, token sql.NullString) error {
		_, err := testDB.Exec(`INSERT INTO cek_login (id, tenant_id, peserta_id, content_token) VALUES (?, 1, 1, ?)`, id, token)
		return err
	}

	// Two NULL tokens coexist (sessions that have not started content).
	if err := insert(1, sql.NullString{}); err != nil {
		t.Fatalf("insert NULL token 1: %v", err)
	}
	if err := insert(2, sql.NullString{}); err != nil {
		t.Fatalf("insert NULL token 2 (NULLs must coexist): %v", err)
	}
	// First non-null token succeeds.
	if err := insert(3, sql.NullString{String: "tok-A", Valid: true}); err != nil {
		t.Fatalf("insert first tok-A: %v", err)
	}
	// A second row claiming the same non-null token is rejected.
	if err := insert(4, sql.NullString{String: "tok-A", Valid: true}); err == nil {
		t.Errorf("expected duplicate content_token to be rejected by the unique index")
	}
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

// foreignKeyCheckViolations returns one description per PRAGMA foreign_key_check row.
func foreignKeyCheckViolations(t *testing.T, database *sql.DB) []string {
	t.Helper()
	rows, err := database.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var table, parent sql.NullString
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			t.Fatalf("scan foreign_key_check: %v", err)
		}
		out = append(out, fmt.Sprintf("%s rowid=%d -> %s (fk #%d)",
			table.String, rowid.Int64, parent.String, fkid.Int64))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign_key_check: %v", err)
	}
	return out
}

// TestMigrationsRepairSentinelZeroForeignKeys is the Gate 0 exit criterion of
// codebase-bug-sweep, expressed as a test (clause 2.2, bug condition C2).
//
// Scenario: an existing installation whose peserta rows carry the sentinel 0 in the
// FOREIGN KEY columns kelas_id / ruang_id — written by CreateStudent while foreign_keys
// enforcement was silently off (clause 1.1). The operator upgrades the binary, migrations
// run again, and from that point PRAGMA foreign_key_check must be clean, because Gate 1
// turns enforcement on for the first time.
//
// On F (no repair migration) foreign_key_check reports the sentinel rows and this FAILS.
//
// The fixture deliberately has enforcement off: sentinel-0 rows could only ever be created by a
// connection without it, and with it on they are simply rejected on INSERT.
func TestMigrationsRepairSentinelZeroForeignKeys(t *testing.T) {
	testDB, cleanup := runMigrationsInLegacyTempDB(t)
	defer cleanup()
	dir := migrationsDir(t)

	// Referentially valid parents, plus a control row that must be left untouched.
	for _, q := range []string{
		`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`,
		`INSERT INTO kelas (id, tenant_id, nama_kelas) VALUES (7, 1, 'XII IPA 1')`,
		`INSERT INTO ruang (id, tenant_id, nama_ruang, username, password_hash) VALUES (9, 1, 'Ruang A', 'ruang_a', 'hash')`,
		// Control: a fully valid row (¬C) — preservation clause 3.2.
		`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
		 VALUES (1, 1, 'valid', 'hash', 'Siswa Lengkap', 7, 9)`,
		// C: legacy sentinel rows in every combination the handler could have produced.
		`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
		 VALUES (2, 1, 'both-zero', 'hash', 'Sentinel Dua', 0, 0)`,
		`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
		 VALUES (3, 1, 'kelas-zero', 'hash', 'Sentinel Kelas', 0, 9)`,
		`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
		 VALUES (4, 1, 'ruang-zero', 'hash', 'Sentinel Ruang', 7, 0)`,
	} {
		if _, err := testDB.Exec(q); err != nil {
			t.Fatalf("seed legacy sentinel data: %v\nquery: %s", err, q)
		}
	}

	// Precondition: the sentinel rows really are referential violations. If this ever stops
	// holding, the test below would pass for the wrong reason.
	if before := foreignKeyCheckViolations(t, testDB); len(before) == 0 {
		t.Fatal("precondition: expected the sentinel-0 rows to violate foreign keys before repair")
	} else {
		t.Logf("pre-repair violations: %v", before)
	}

	// The upgrade: migrations run again on the existing database.
	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("rerun migrations over legacy sentinel data: %v", err)
	}

	if after := foreignKeyCheckViolations(t, testDB); len(after) != 0 {
		t.Errorf("PRAGMA foreign_key_check reported %d violation(s) after migrations: %v\n"+
			"Gate 0 requires a referentially clean database before foreign_keys is enabled",
			len(after), after)
	}

	// The sentinel columns must now be NULL, and only those.
	for _, tc := range []struct {
		noID      string
		wantKelas sql.NullInt64
		wantRuang sql.NullInt64
	}{
		{"both-zero", sql.NullInt64{}, sql.NullInt64{}},
		{"kelas-zero", sql.NullInt64{}, sql.NullInt64{Int64: 9, Valid: true}},
		{"ruang-zero", sql.NullInt64{Int64: 7, Valid: true}, sql.NullInt64{}},
	} {
		var kelasID, ruangID sql.NullInt64
		if err := testDB.QueryRow(
			`SELECT kelas_id, ruang_id FROM peserta WHERE no_id = ?`, tc.noID,
		).Scan(&kelasID, &ruangID); err != nil {
			t.Fatalf("read peserta %s: %v", tc.noID, err)
		}
		if kelasID != tc.wantKelas {
			t.Errorf("peserta[%s].kelas_id = %v, want %v", tc.noID, kelasID, tc.wantKelas)
		}
		if ruangID != tc.wantRuang {
			t.Errorf("peserta[%s].ruang_id = %v, want %v", tc.noID, ruangID, tc.wantRuang)
		}
	}

	// ... and the valid control row must be untouched (clause 3.2).
	var kelasID, ruangID sql.NullInt64
	if err := testDB.QueryRow(
		`SELECT kelas_id, ruang_id FROM peserta WHERE no_id = 'valid'`,
	).Scan(&kelasID, &ruangID); err != nil {
		t.Fatalf("read control peserta: %v", err)
	}
	if kelasID != (sql.NullInt64{Int64: 7, Valid: true}) || ruangID != (sql.NullInt64{Int64: 9, Valid: true}) {
		t.Errorf("valid peserta references were altered: kelas_id = %v, ruang_id = %v, want 7 / 9", kelasID, ruangID)
	}

	// Idempotency: a third run must neither fail nor re-dirty the database.
	if err := RunMigrations(testDB, dir); err != nil {
		t.Fatalf("third migration run over repaired data: %v", err)
	}
	if after := foreignKeyCheckViolations(t, testDB); len(after) != 0 {
		t.Errorf("foreign_key_check dirty after idempotent rerun: %v", after)
	}
}

// TestPesertaFKRepairPreservesDataAndChildReferences guards the riskiest part of the Gate 0
// prerequisite: relaxing NOT NULL on peserta.kelas_id / peserta.ruang_id requires a full table
// rebuild (SQLite has no ALTER COLUMN), and a rebuild done carelessly either loses participant
// rows or leaves hasil_tes / cek_login pointing at a table name that no longer exists.
//
// The database is built with the pre-repair shape, populated with participants and children,
// then migrated. Afterwards: every row survives, the child REFERENCES clauses still name
// "peserta", and enforcement behaves correctly on a connection with foreign_keys ON.
func TestPesertaFKRepairPreservesDataAndChildReferences(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "fkrepair.db")
	testDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer testDB.Close()
	testDB.SetMaxOpenConns(1)

	// Recreate the historical peserta shape (kelas_id / ruang_id NOT NULL) so the repair has
	// something to convert, then let the remaining migrations fill in the rest of the schema.
	for _, q := range []string{
		`CREATE TABLE tenants (id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, deleted_at DATETIME)`,
		`INSERT INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`,
		`CREATE TABLE kelas (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, nama_kelas TEXT NOT NULL, deleted_at DATETIME)`,
		`INSERT INTO kelas (id, tenant_id, nama_kelas) VALUES (5, 1, 'XII IPA 1')`,
		`CREATE TABLE ruang (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, nama_ruang TEXT NOT NULL, username TEXT, password_hash TEXT)`,
		`INSERT INTO ruang (id, tenant_id, nama_ruang, username, password_hash) VALUES (6, 1, 'Ruang A', 'ruang_a', 'hash')`,
		`CREATE TABLE peserta (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			no_id TEXT NOT NULL,
			password TEXT NOT NULL,
			nama_peserta TEXT NOT NULL,
			kelas_id INTEGER NOT NULL,
			jenis_kelamin TEXT CHECK(jenis_kelamin IN ('L', 'P')),
			ruang_id INTEGER NOT NULL,
			foto TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME,
			FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
			FOREIGN KEY (kelas_id) REFERENCES kelas(id),
			FOREIGN KEY (ruang_id) REFERENCES ruang(id),
			UNIQUE(tenant_id, no_id)
		)`,
		`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, jenis_kelamin, ruang_id, foto, created_at)
		 VALUES (11, 1, 'keep-me', 'hash', 'Siswa Bertahan', 5, 'L', 6, 'foto.jpg', '2026-01-05 07:00:00')`,
		`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
		 VALUES (12, 1, 'sentinel', 'hash', 'Siswa Sentinel', 0, 0)`,
	} {
		if _, err := testDB.Exec(q); err != nil {
			t.Fatalf("build pre-repair schema: %v\nquery: %s", err, q)
		}
	}

	var notNullBefore int
	if err := testDB.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('peserta') WHERE name IN ('kelas_id','ruang_id') AND "notnull" = 1`,
	).Scan(&notNullBefore); err != nil {
		t.Fatalf("inspect peserta columns: %v", err)
	}
	if notNullBefore != 2 {
		t.Fatalf("precondition: expected kelas_id and ruang_id to be NOT NULL, got %d NOT NULL column(s)", notNullBefore)
	}

	if err := RunMigrations(testDB, migrationsDir(t)); err != nil {
		t.Fatalf("run migrations over pre-repair schema: %v", err)
	}

	// Children created after the schema is complete, so they exercise the post-repair table.
	for _, q := range []string{
		`INSERT INTO mapel (id, tenant_id, nama_mapel, kode_mapel) VALUES (3, 1, 'Matematika', 'MTK')`,
		`INSERT INTO hasil_tes (id, tenant_id, peserta_id, mapel_id, skor, skor_maks, validasi)
		 VALUES (21, 1, 11, 3, 80, 100, '1_keep-me_3')`,
		`INSERT INTO cek_login (id, tenant_id, peserta_id, mapel_id) VALUES (31, 1, 11, 3)`,
	} {
		if _, err := testDB.Exec(q); err != nil {
			t.Fatalf("seed child rows: %v\nquery: %s", err, q)
		}
	}

	// 1. Both reference columns are nullable now.
	var notNullAfter int
	if err := testDB.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('peserta') WHERE name IN ('kelas_id','ruang_id') AND "notnull" = 1`,
	).Scan(&notNullAfter); err != nil {
		t.Fatalf("inspect peserta columns after repair: %v", err)
	}
	if notNullAfter != 0 {
		t.Errorf("peserta still has %d NOT NULL reference column(s) after the repair", notNullAfter)
	}

	// 2. Every column of the surviving row is intact, and the sentinel row was nulled.
	var noID, nama, foto, createdAt, jenisKelamin string
	var kelasID, ruangID sql.NullInt64
	if err := testDB.QueryRow(
		`SELECT no_id, nama_peserta, kelas_id, jenis_kelamin, ruang_id, foto, created_at FROM peserta WHERE id = 11`,
	).Scan(&noID, &nama, &kelasID, &jenisKelamin, &ruangID, &foto, &createdAt); err != nil {
		t.Fatalf("read preserved peserta: %v", err)
	}
	// created_at is read back through the driver's DATETIME conversion, so only the instant is
	// compared, not its textual rendering.
	if noID != "keep-me" || nama != "Siswa Bertahan" || jenisKelamin != "L" || foto != "foto.jpg" ||
		!strings.HasPrefix(createdAt, "2026-01-05") {
		t.Errorf("peserta row was altered by the rebuild: no_id=%q nama=%q jk=%q foto=%q created_at=%q",
			noID, nama, jenisKelamin, foto, createdAt)
	}
	if kelasID != (sql.NullInt64{Int64: 5, Valid: true}) || ruangID != (sql.NullInt64{Int64: 6, Valid: true}) {
		t.Errorf("valid references were altered: kelas_id=%v ruang_id=%v, want 5 / 6", kelasID, ruangID)
	}

	// 3. Child tables still reference "peserta", not the temporary rebuild name.
	for _, child := range []string{"hasil_tes", "cek_login"} {
		var ddl string
		if err := testDB.QueryRow(
			`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, child,
		).Scan(&ddl); err != nil {
			t.Fatalf("read %s ddl: %v", child, err)
		}
		if !strings.Contains(ddl, "REFERENCES peserta(id)") {
			t.Errorf("%s no longer references peserta(id) after the rebuild; ddl:\n%s", child, ddl)
		}
		if strings.Contains(ddl, "peserta_fk_repair_tmp") {
			t.Errorf("%s references the temporary rebuild table; ddl:\n%s", child, ddl)
		}
	}

	// 4. AUTOINCREMENT continues past the copied ids rather than restarting at 1.
	res, err := testDB.Exec(
		`INSERT INTO peserta (tenant_id, no_id, password, nama_peserta) VALUES (1, 'after-repair', 'hash', 'Siswa Baru')`)
	if err != nil {
		t.Fatalf("insert after repair: %v", err)
	}
	if newID, err := res.LastInsertId(); err != nil {
		t.Fatalf("LastInsertId: %v", err)
	} else if newID <= 12 {
		t.Errorf("new peserta id = %d, want > 12 (AUTOINCREMENT sequence was not preserved)", newID)
	}

	// 5. Enforcement behaves correctly once foreign_keys is actually on (what Gate 1 flips).
	if _, err := testDB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign_keys: %v", err)
	}
	if _, err := testDB.Exec(
		`INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id) VALUES (1, 'bad-ref', 'hash', 'Siswa', 999)`,
	); err == nil {
		t.Error("expected a dangling kelas_id to be rejected under foreign_keys = ON")
	}
	if _, err := testDB.Exec(
		`INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, validasi) VALUES (1, 999, 3, 'dangling')`,
	); err == nil {
		t.Error("expected hasil_tes.peserta_id -> peserta(id) to still be enforced after the rebuild")
	}
	if _, err := testDB.Exec(
		`INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, validasi) VALUES (1, 11, 3, 'valid-after-repair')`,
	); err != nil {
		t.Errorf("a valid child row was rejected after the rebuild: %v", err)
	}
	if violations := foreignKeyCheckViolations(t, testDB); len(violations) != 0 {
		t.Errorf("foreign_key_check dirty after the repair: %v", violations)
	}
}

// TestRunMigrationsWarnsAboutForeignKeyViolationsWithoutFailing covers the post-migration data
// diagnostic (clause 2.2 / 3.1). A violation that the Gate 0 repair does not cover — here a
// hasil_tes row pointing at a peserta that does not exist — must be REPORTED with its table and
// rowid, and must NOT abort startup: refusing to boot over a data problem would stop a school
// from running its exam, which is worse than running with a warning. Staying silent is the one
// unacceptable option, since that is precisely how the disabled-pragma defect stayed hidden.
func TestRunMigrationsWarnsAboutForeignKeyViolationsWithoutFailing(t *testing.T) {
	// Enforcement off, because the dangling row below is exactly what enforcement prevents.
	testDB, cleanup := runMigrationsInLegacyTempDB(t)
	defer cleanup()

	for _, q := range []string{
		`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (1, 'default', 'Default School')`,
		`INSERT INTO mapel (id, tenant_id, nama_mapel, kode_mapel) VALUES (2, 1, 'Matematika', 'MTK')`,
		// peserta_id 4242 does not exist: a violation no data-repair migration addresses.
		`INSERT INTO hasil_tes (id, tenant_id, peserta_id, mapel_id, validasi) VALUES (55, 1, 4242, 2, 'dangling')`,
	} {
		if _, err := testDB.Exec(q); err != nil {
			t.Fatalf("seed dangling child row: %v\nquery: %s", err, q)
		}
	}

	var logged bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logged)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	}()

	if err := RunMigrations(testDB, migrationsDir(t)); err != nil {
		t.Fatalf("RunMigrations must not fail on a data-level foreign key violation, got: %v", err)
	}

	output := logged.String()
	for _, want := range []string{"WARNING", "foreign key violation", `"hasil_tes"`, "55", `"peserta"`} {
		if !strings.Contains(output, want) {
			t.Errorf("migration log does not mention %q; the diagnostic must identify the offending table and rowid.\nlog:\n%s", want, output)
		}
	}
}

// TestRunMigrationsLogsNoForeignKeyWarningOnCleanDatabase is the ¬C side of the diagnostic: a
// referentially clean database must produce no warning at all, so the signal stays meaningful.
func TestRunMigrationsLogsNoForeignKeyWarningOnCleanDatabase(t *testing.T) {
	testDB, cleanup := runMigrationsInTempDB(t)
	defer cleanup()

	var logged bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logged)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	}()

	if err := RunMigrations(testDB, migrationsDir(t)); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}
	if strings.Contains(logged.String(), "foreign key violation") {
		t.Errorf("clean database produced a foreign key warning:\n%s", logged.String())
	}
}
