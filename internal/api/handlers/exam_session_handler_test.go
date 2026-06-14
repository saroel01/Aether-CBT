package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

func TestCreateExamSession_HappyThenTokenConflict(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/exam-sessions", adminOnly, CreateExamSession)
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedSoalPackage(t, database, 10, 1, "Pkg", "u10")
	testutil.SeedExam(t, database, 1, 1, 1, intPtr(10)) // exam has a package -> can be terjadwal

	body := `{"exam_id":1,"waktu_mulai":"2026-06-01T08:00:00Z","waktu_selesai":"2026-06-01T10:00:00Z","token":"TOK","status":"terjadwal"}`
	resp := doJSON(t, app, "POST", "/api/admin/exam-sessions", strings.NewReader(body))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d, want 200", resp.StatusCode)
	}

	// Same token, overlapping window -> 409 (Requirement 4.4).
	overlap := `{"exam_id":1,"waktu_mulai":"2026-06-01T09:00:00Z","waktu_selesai":"2026-06-01T11:00:00Z","token":"TOK","status":"terjadwal"}`
	resp = doJSON(t, app, "POST", "/api/admin/exam-sessions", strings.NewReader(overlap))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("overlap status = %d, want 409", resp.StatusCode)
	}
}

func TestCreateExamSession_PackageRequiredForActive(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/exam-sessions", adminOnly, CreateExamSession)
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil) // exam WITHOUT a package

	body := `{"exam_id":1,"waktu_mulai":"2026-06-01T08:00:00Z","waktu_selesai":"2026-06-01T10:00:00Z","token":"TOK","status":"terjadwal"}`
	resp := doJSON(t, app, "POST", "/api/admin/exam-sessions", strings.NewReader(body))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (package required)", resp.StatusCode)
	}
}

func TestLinkSessionClasses_CrossTenantRejected(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/exam-sessions/:id/classes", adminOnly, LinkSessionClasses)
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedTenant(t, database, 2, "other", "Other School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	testutil.SeedExamSession(t, database, 1, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK", "draft")
	testutil.SeedKelas(t, database, 1, 1, "Kelas A")      // tenant 1
	testutil.SeedKelas(t, database, 2, 2, "Kelas Other")  // tenant 2

	// Linking a class from another tenant -> 400 (Requirement 4.7).
	resp := doJSON(t, app, "POST", "/api/admin/exam-sessions/1/classes", strings.NewReader(`{"ids":[1,2]}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cross-tenant link status = %d, want 400", resp.StatusCode)
	}
}
