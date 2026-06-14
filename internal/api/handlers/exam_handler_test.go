package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

func intPtr(i int) *int { return &i }

func TestCreateExam_HappyThenInvalidReference(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/exams", adminOnly, CreateExam)
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")

	resp := doJSON(t, app, "POST", "/api/admin/exams", strings.NewReader(`{"mapel_id":1,"durasi_menit":90,"kkm":70,"shuffle_questions":true}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d, want 200", resp.StatusCode)
	}

	// Referencing a mapel that does not belong to the tenant -> 400 (Requirement 2.2).
	resp = doJSON(t, app, "POST", "/api/admin/exams", strings.NewReader(`{"mapel_id":999}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid ref status = %d, want 400", resp.StatusCode)
	}
}

func TestListExams_TenantScoped(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Get("/api/admin/exams", adminOnly, ListExams)
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)

	resp := doJSON(t, app, "GET", "/api/admin/exams", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDeleteExam_ConflictWithActiveSession(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Delete("/api/admin/exams/:id", adminOnly, DeleteExam)
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	testutil.SeedExamSession(t, database, 1, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK", "terjadwal")

	resp := doJSON(t, app, "DELETE", "/api/admin/exams/1", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (has active session)", resp.StatusCode)
	}
}
