package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// roomStatusOf fetches /api/supervisor/room-status and returns the per-student statuses keyed
// by peserta id. Supervisor role => user_id maps to ruang_id (here room 1).
func roomStatusOf(t *testing.T, app *fiber.App) map[int]LiveStudentStatus {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", "/api/supervisor/room-status", nil), -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out struct {
		Success bool                `json:"success"`
		Data    []LiveStudentStatus `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	m := make(map[int]LiveStudentStatus, len(out.Data))
	for _, s := range out.Data {
		m[s.ID] = s
	}
	return m
}

// TestGetRoomStatus_StudentRoleForbidden: only supervisor/admin may view room status
// (Requirement 11.4).
func TestGetRoomStatus_StudentRoleForbidden(t *testing.T) {
	app, _, _, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Get("/api/supervisor/room-status", GetRoomStatus)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/supervisor/room-status", nil), -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (student forbidden)", resp.StatusCode)
	}
}

// TestGetRoomStatus_ComputesSessionStatus: the room overview reports a per-student status —
// "locked" for a server-locked session, "not_logged_in" for a student with no active session
// (Requirement 11.1, 11.2).
func TestGetRoomStatus_ComputesSessionStatus(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor")
	defer cleanup()
	app.Get("/api/supervisor/room-status", GetRoomStatus)

	sid := seedContentGraph(t, database, defaultTenant1Seed("ct-locked"))
	// peserta 1 is locked on session sid.
	if err := repository.NewCekLoginRepository(database).Lock(1, 1, sid); err != nil {
		t.Fatalf("lock: %v", err)
	}
	// peserta 2 is in the same room (kelas 1, ruang 1) but has NOT started: no cek_login.
	testutil.SeedPeserta(t, database, 2, 1, 1, 1, "p2", "Siswa2")

	statuses := roomStatusOf(t, app)
	if s, ok := statuses[1]; !ok {
		t.Fatalf("peserta 1 missing from room status; got %v", statuses)
	} else if s.Status != "locked" {
		t.Errorf("peserta 1 status = %q, want locked", s.Status)
	}
	if s, ok := statuses[2]; !ok {
		t.Fatalf("peserta 2 missing from room status; got %v", statuses)
	} else if s.Status != "not_logged_in" {
		t.Errorf("peserta 2 status = %q, want not_logged_in", s.Status)
	}
}

// TestResetStudentSession_TargetsSpecificSession: resetting targets only the named session,
// clearing its lock and leaving any other active session intact (Requirement 10.4, 11.3).
func TestResetStudentSession_TargetsSpecificSession(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor")
	defer cleanup()
	app.Post("/api/supervisor/reset", ResetStudentSession)

	sid := seedContentGraph(t, database, defaultTenant1Seed("ct-reset"))
	cekRepo := repository.NewCekLoginRepository(database)
	// A second active session for the same peserta (different session id) that must survive.
	if err := cekRepo.Start(1, 1, 999, "att-other"); err != nil {
		t.Fatalf("seed second session: %v", err)
	}
	if err := cekRepo.Lock(1, 1, sid); err != nil {
		t.Fatalf("lock: %v", err)
	}

	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":1,"session_id":%d}`, sid))
	resp := doJSON(t, app, "POST", "/api/supervisor/reset", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// The targeted session's cek_login is gone (lock cleared by row removal).
	if _, err := cekRepo.GetBySession(1, 1, sid); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("targeted session still present: %v (want ErrNotFound)", err)
	}
	// The sibling session survives.
	if _, err := cekRepo.GetBySession(1, 1, 999); err != nil {
		t.Errorf("sibling session was removed; want it to survive: %v", err)
	}
}

// TestResetStudentSession_StudentRoleForbidden (Requirement 11.4).
func TestResetStudentSession_StudentRoleForbidden(t *testing.T) {
	app, _, _, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/supervisor/reset", ResetStudentSession)

	body := strings.NewReader(`{"peserta_id":1,"session_id":1}`)
	resp := doJSON(t, app, "POST", "/api/supervisor/reset", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (student forbidden)", resp.StatusCode)
	}
}
