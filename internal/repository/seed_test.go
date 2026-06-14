package repository

import (
	"database/sql"
	"testing"
)

// Shared seed helpers for repository tests. Each inserts a tenant-scoped row with an
// explicit id so tests can reference deterministic identifiers. Seeding goes directly
// through the migrated test database (no global state).

func seedTenant(t *testing.T, database *sql.DB, id int, slug, name string) {
	t.Helper()
	if _, err := database.Exec(`INSERT OR IGNORE INTO tenants (id, slug, name) VALUES (?, ?, ?)`, id, slug, name); err != nil {
		t.Fatalf("seed tenant %d: %v", id, err)
	}
}

func seedKelas(t *testing.T, database *sql.DB, id, tenantID int, nama string) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO kelas (id, tenant_id, nama_kelas) VALUES (?, ?, ?)`, id, tenantID, nama); err != nil {
		t.Fatalf("seed kelas %d: %v", id, err)
	}
}

func seedMapel(t *testing.T, database *sql.DB, id, tenantID int, nama, kode string) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO mapel (id, tenant_id, nama_mapel, kode_mapel) VALUES (?, ?, ?, ?)`, id, tenantID, nama, kode); err != nil {
		t.Fatalf("seed mapel %d: %v", id, err)
	}
}

func seedRuang(t *testing.T, database *sql.DB, id, tenantID int, nama, username string) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO ruang (id, tenant_id, nama_ruang, username, password_hash) VALUES (?, ?, ?, ?, ?)`, id, tenantID, nama, username, "hash"); err != nil {
		t.Fatalf("seed ruang %d: %v", id, err)
	}
}

func seedPeserta(t *testing.T, database *sql.DB, id, tenantID, kelasID, ruangID int, noID, nama string) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (?, ?, ?, ?, ?, ?, ?)`, id, tenantID, noID, "siswa123", nama, kelasID, ruangID); err != nil {
		t.Fatalf("seed peserta %d: %v", id, err)
	}
}

func seedSoalPackage(t *testing.T, database *sql.DB, id, tenantID int, nama, packageUUID string) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO soal_package (id, tenant_id, nama, package_uuid) VALUES (?, ?, ?, ?)`, id, tenantID, nama, packageUUID); err != nil {
		t.Fatalf("seed soal_package %d: %v", id, err)
	}
}

// seedExam inserts a minimal exam row. soalPackageID may be nil for a draft exam.
func seedExam(t *testing.T, database *sql.DB, id, tenantID, mapelID int, soalPackageID *int) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO exam (id, tenant_id, mapel_id, soal_package_id) VALUES (?, ?, ?, ?)`, id, tenantID, mapelID, soalPackageID); err != nil {
		t.Fatalf("seed exam %d: %v", id, err)
	}
}

// seedExamSession inserts a session row. status must satisfy the CHECK constraint.
func seedExamSession(t *testing.T, database *sql.DB, id, tenantID, examID int, mulai, selesai, token, status string) {
	t.Helper()
	if _, err := database.Exec(`INSERT INTO exam_session (id, tenant_id, exam_id, waktu_mulai, waktu_selesai, token, status) VALUES (?, ?, ?, ?, ?, ?, ?)`, id, tenantID, examID, mulai, selesai, token, status); err != nil {
		t.Fatalf("seed exam_session %d: %v", id, err)
	}
}
