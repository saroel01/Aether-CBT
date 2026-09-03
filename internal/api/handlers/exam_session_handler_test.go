package handlers

import (
	"database/sql"
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

// TestLinkSessionClasses_CrossTenantSessionRejected covers codebase-bug-sweep clause
// 1.5/2.5 (C5) at the handler boundary: the admin is authenticated in tenant 1 and links
// kelas/ruang they legitimately own, but the :id in the path is a session owned by tenant 2.
//
// The expected status is 404, not 403: confirming that another tenant's session EXISTS is
// itself a cross-tenant information leak.
//
// On F the link succeeds with 200 and rows appear in exam_session_kelas/exam_session_ruang.
//
// Validates: Requirements 2.5, 3.7
func TestLinkSessionClasses_CrossTenantSessionRejected(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/exam-sessions/:id/classes", adminOnly, LinkSessionClasses)
	app.Post("/api/admin/exam-sessions/:id/rooms", adminOnly, LinkSessionRooms)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedTenant(t, database, 2, "other", "Other School")
	testutil.SeedMapel(t, database, 2, 2, "Biologi", "BIO")
	testutil.SeedExam(t, database, 2, 2, 2, nil)
	// Session 99 belongs to tenant 2 — the caller's tenant is 1.
	testutil.SeedExamSession(t, database, 99, 2, 2, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK-B", "draft")
	// Caller's OWN kelas and ruang.
	testutil.SeedKelas(t, database, 1, 1, "Kelas A")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")

	resp := doJSON(t, app, "POST", "/api/admin/exam-sessions/99/classes", strings.NewReader(`{"ids":[1]}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("link classes to other tenant's session: status = %d, want 404", resp.StatusCode)
	}
	resp = doJSON(t, app, "POST", "/api/admin/exam-sessions/99/rooms", strings.NewReader(`{"ids":[1]}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("link rooms to other tenant's session: status = %d, want 404", resp.StatusCode)
	}

	assertNoLinkRows(t, database, `SELECT COUNT(*) FROM exam_session_kelas WHERE session_id = 99`)
	assertNoLinkRows(t, database, `SELECT COUNT(*) FROM exam_session_ruang WHERE session_id = 99`)
}

// TestLinkSessionClasses_OwnTenantStillSucceeds is the preservation half of clause 3.7: the
// tenancy guard must not disturb the normal path.
//
// Validates: Requirements 3.7
func TestLinkSessionClasses_OwnTenantStillSucceeds(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/exam-sessions/:id/classes", adminOnly, LinkSessionClasses)
	app.Post("/api/admin/exam-sessions/:id/rooms", adminOnly, LinkSessionRooms)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	testutil.SeedExamSession(t, database, 1, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK", "draft")
	testutil.SeedKelas(t, database, 1, 1, "Kelas A")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")

	resp := doJSON(t, app, "POST", "/api/admin/exam-sessions/1/classes", strings.NewReader(`{"ids":[1]}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("own-tenant link classes: status = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	if body["message"] != "classes linked" {
		t.Errorf("own-tenant link classes message = %v, want %q", body["message"], "classes linked")
	}

	resp = doJSON(t, app, "POST", "/api/admin/exam-sessions/1/rooms", strings.NewReader(`{"ids":[1]}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("own-tenant link rooms: status = %d, want 200", resp.StatusCode)
	}
	body = decodeJSON(t, resp)
	if body["message"] != "rooms linked" {
		t.Errorf("own-tenant link rooms message = %v, want %q", body["message"], "rooms linked")
	}

	var kelasRows, ruangRows int
	_ = database.QueryRow(`SELECT COUNT(*) FROM exam_session_kelas WHERE session_id = 1 AND kelas_id = 1`).Scan(&kelasRows)
	_ = database.QueryRow(`SELECT COUNT(*) FROM exam_session_ruang WHERE session_id = 1 AND ruang_id = 1`).Scan(&ruangRows)
	if kelasRows != 1 || ruangRows != 1 {
		t.Errorf("own-tenant link rows: kelas=%d ruang=%d, want 1 and 1", kelasRows, ruangRows)
	}
}

// assertNoLinkRows fails when a COUNT(*) query returns anything but zero.
func assertNoLinkRows(t *testing.T, database *sql.DB, query string) {
	t.Helper()
	var n int
	if err := database.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	if n != 0 {
		t.Errorf("cross-tenant write leaked %d row(s): %s", n, query)
	}
}
