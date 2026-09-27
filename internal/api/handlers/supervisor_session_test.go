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

// TestResetStudentSession_TargetsSpecificSession: resetting clears the named session's
// cek_login row (lock cleared by row removal). Task 14 enforces one active session per
// peserta via a partial unique index, so the previous "second concurrent session survives"
// scenario can no longer be seeded; the reset's targeted-removal property is still asserted.
func TestResetStudentSession_TargetsSpecificSession(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor")
	defer cleanup()
	app.Post("/api/supervisor/reset", ResetStudentSession)

	sid := seedContentGraph(t, database, defaultTenant1Seed("ct-reset"))
	cekRepo := repository.NewCekLoginRepository(database)
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
}

// TestRecordInfraction_SupervisorScopedToOwnRoom: a supervisor (user_id = ruang_id) may
// only record infractions for peserta in their own room. A peserta in another room must be
// rejected with 403 (review H3, Task 16).
func TestRecordInfraction_SupervisorScopedToOwnRoom(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor")
	defer cleanup()
	app.Post("/api/student/infraction", RecordInfraction)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a") // supervisor's room (user_id=1)
	testutil.SeedRuang(t, database, 2, 1, "Ruang B", "ruang_b") // a DIFFERENT room
	// Peserta in room 2 — NOT the supervisor's room 1.
	testutil.SeedPeserta(t, database, 1, 1, 1, 2, "2026001", "Other Room Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	cekRepo := repository.NewCekLoginRepository(database)
	if err := cekRepo.Start(1, 1, 1, "tok-other-room"); err != nil {
		t.Fatalf("seed cek_login: %v", err)
	}

	// Supervisor (room 1) records an infraction on a peserta in room 2.
	resp := doJSON(t, app, "POST", "/api/student/infraction", strings.NewReader(`{"peserta_id":1,"session_id":1}`))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("supervisor infraction on other room: status = %d, want 403", resp.StatusCode)
	}
}

// TestRecordInfraction_SupervisorAllowedInOwnRoom: a supervisor CAN record infractions for peserta in their own room.
func TestRecordInfraction_SupervisorAllowedInOwnRoom(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor")
	defer cleanup()
	app.Post("/api/student/infraction", RecordInfraction)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a") // supervisor's room (user_id=1)
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Room 1 Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	cekRepo := repository.NewCekLoginRepository(database)
	if err := cekRepo.Start(1, 1, 1, "tok-own-room"); err != nil {
		t.Fatalf("seed cek_login: %v", err)
	}

	resp := doJSON(t, app, "POST", "/api/student/infraction", strings.NewReader(`{"peserta_id":1,"session_id":1}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("supervisor infraction on own room: status = %d, want 200", resp.StatusCode)
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

// TestResetStudentSession_SupervisorScopedToOwnRoom (P1-6 regression test)
// Supervisor in room 1 cannot reset a student in room 2.
// Assert: 403 Forbidden and the cek_login row for room 2 student remains intact.
func TestResetStudentSession_SupervisorScopedToOwnRoom(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor") // user_id = 1 (Ruang 1)
	defer cleanup()
	app.Post("/api/supervisor/reset", ResetStudentSession)

	// Seed room 1 first (supervisor's room)
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")

	// Seed student in room 2 via seedContentGraph
	s := defaultTenant1Seed("tok-room-b")
	s.pesertaID = 2
	s.ruangID = 2
	s.noID = "2026002"
	s.nama = "Room B Student"
	sid := seedContentGraph(t, database, s)

	cekRepo := repository.NewCekLoginRepository(database)

	// Supervisor from Room 1 tries to reset student in Room 2
	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":2,"session_id":%d}`, sid))
	resp := doJSON(t, app, "POST", "/api/supervisor/reset", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 Forbidden (supervisor cannot reset other room)", resp.StatusCode)
	}

	// Verify cek_login was NOT deleted
	active, err := cekRepo.GetBySession(1, 2, sid)
	if err != nil {
		t.Fatalf("expected student session in room 2 to still exist, got err: %v", err)
	}
	if active.PesertaID != 2 {
		t.Errorf("peserta_id = %d, want 2", active.PesertaID)
	}
}

// TestResetStudentSession_AdminCanResetAnyRoom verifies admins are not restricted by room.
func TestResetStudentSession_AdminCanResetAnyRoom(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/supervisor/reset", ResetStudentSession)

	s := defaultTenant1Seed("tok-admin-reset")
	s.pesertaID = 2
	s.ruangID = 2
	s.noID = "2026002"
	s.nama = "Room B Student"
	sid := seedContentGraph(t, database, s)

	cekRepo := repository.NewCekLoginRepository(database)

	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":2,"session_id":%d}`, sid))
	resp := doJSON(t, app, "POST", "/api/supervisor/reset", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK (admin can reset any room)", resp.StatusCode)
	}

	// Verify cek_login was deleted
	if _, err := cekRepo.GetBySession(1, 2, sid); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected session to be deleted, got err: %v", err)
	}
}

// TestResetStudentSession_SessionRoomRestriction (P1-6) verifies that even if student's default ruang_id
// matches supervisor's room, if the specific session is restricted to another room via exam_session_ruang,
// the supervisor cannot manage or reset that session (returns 403 Forbidden).
func TestResetStudentSession_SessionRoomRestriction(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor") // user_id = 1 (Ruang 1)
	defer cleanup()
	app.Post("/api/supervisor/reset", ResetStudentSession)

	// Seed student in room 1 (seedContentGraph seeds room 1)
	s := defaultTenant1Seed("tok-session-room-test")
	s.pesertaID = 1
	s.ruangID = 1
	sid := seedContentGraph(t, database, s)

	// Seed room 2
	testutil.SeedRuang(t, database, 2, 1, "Ruang B", "ruang_b")

	// Explicitly restrict this exam session to room 2 ONLY
	_, err := database.Exec(`INSERT INTO exam_session_ruang (session_id, ruang_id) VALUES (?, 2)`, sid)
	if err != nil {
		t.Fatalf("link session to room 2: %v", err)
	}

	// Supervisor from Room 1 tries to reset the session restricted to Room 2
	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":1,"session_id":%d}`, sid))
	resp := doJSON(t, app, "POST", "/api/supervisor/reset", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for supervisor whose room is not linked to session, got: %d", resp.StatusCode)
	}
}
