package repository

import (
	"database/sql"
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

// Local unexported wrappers around the shared testutil seeders so repository tests read
// naturally (seedTenant(...)) while the seeding logic lives in one place.

func seedTenant(t *testing.T, database *sql.DB, id int, slug, name string) {
	t.Helper()
	testutil.SeedTenant(t, database, id, slug, name)
}
func seedKelas(t *testing.T, database *sql.DB, id, tenantID int, nama string) {
	t.Helper()
	testutil.SeedKelas(t, database, id, tenantID, nama)
}
func seedMapel(t *testing.T, database *sql.DB, id, tenantID int, nama, kode string) {
	t.Helper()
	testutil.SeedMapel(t, database, id, tenantID, nama, kode)
}
func seedRuang(t *testing.T, database *sql.DB, id, tenantID int, nama, username string) {
	t.Helper()
	testutil.SeedRuang(t, database, id, tenantID, nama, username)
}
func seedPeserta(t *testing.T, database *sql.DB, id, tenantID, kelasID, ruangID int, noID, nama string) {
	t.Helper()
	testutil.SeedPeserta(t, database, id, tenantID, kelasID, ruangID, noID, nama)
}
func seedSoalPackage(t *testing.T, database *sql.DB, id, tenantID int, nama, packageUUID string) {
	t.Helper()
	testutil.SeedSoalPackage(t, database, id, tenantID, nama, packageUUID)
}
func seedExam(t *testing.T, database *sql.DB, id, tenantID, mapelID int, soalPackageID *int) {
	t.Helper()
	testutil.SeedExam(t, database, id, tenantID, mapelID, soalPackageID)
}
func seedExamSession(t *testing.T, database *sql.DB, id, tenantID, examID int, mulai, selesai, token, status string) {
	t.Helper()
	testutil.SeedExamSession(t, database, id, tenantID, examID, mulai, selesai, token, status)
}
