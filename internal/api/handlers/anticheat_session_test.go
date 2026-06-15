package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// TestRecordInfraction_LocksAtThreshold: session-based infraction increments the count and,
// on reaching AntiCheatLockThreshold, server-locks the session and reports locked=true
// (Requirement 10.1, 10.2, 10.6).
func TestRecordInfraction_LocksAtThreshold(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	SetAntiCheatLockThreshold(3)
	defer SetAntiCheatLockThreshold(3)
	app.Post("/api/student/infraction", RecordInfraction)

	sid := seedContentGraph(t, database, defaultTenant1Seed("ignored"))
	cekRepo := repository.NewCekLoginRepository(database)

	body := func() *strings.Reader {
		return strings.NewReader(fmt.Sprintf(`{"peserta_id":1,"session_id":%d}`, sid))
	}

	for i := 0; i < 2; i++ {
		resp := doJSON(t, app, "POST", "/api/student/infraction", body())
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("infraction %d: status = %d, want 200", i+1, resp.StatusCode)
		}
		if locked, _ := cekRepo.IsLocked(1, 1, sid); locked {
			t.Fatalf("session locked before reaching the threshold (after %d)", i+1)
		}
	}

	resp := doJSON(t, app, "POST", "/api/student/infraction", body())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("threshold infraction: status = %d, want 200", resp.StatusCode)
	}
	body0 := decodeJSON(t, resp)
	data, _ := body0["data"].(map[string]interface{})
	if data["locked"] != true {
		t.Errorf("expected locked=true at threshold, got %v", data["locked"])
	}
	if locked, _ := cekRepo.IsLocked(1, 1, sid); !locked {
		t.Errorf("expected cek_login.locked = 1 after threshold infraction")
	}
}

// TestRecordInfraction_NonOwnerRejected: a student may only record their own infractions
// (mirrors StartExamSession's owner check).
func TestRecordInfraction_NonOwnerRejected(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/infraction", RecordInfraction)
	sid := seedContentGraph(t, database, defaultTenant1Seed("ignored"))

	// test middleware sets user_id = 1; peserta_id 999 is someone else.
	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":999,"session_id":%d}`, sid))
	resp := doJSON(t, app, "POST", "/api/student/infraction", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (non-owner)", resp.StatusCode)
	}
}

// TestUpdateStudentProgress_LockedRejected: a server-locked session refuses progress updates
// (Requirement 10.3, Property 11).
func TestUpdateStudentProgress_LockedRejected(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/progress", UpdateStudentProgress)
	sid := seedContentGraph(t, database, defaultTenant1Seed("ignored"))
	if err := repository.NewCekLoginRepository(database).Lock(1, 1, sid); err != nil {
		t.Fatalf("lock: %v", err)
	}

	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":1,"session_id":%d,"answered_count":5,"total_questions":10}`, sid))
	resp := doJSON(t, app, "POST", "/api/student/progress", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (progress on locked session)", resp.StatusCode)
	}
}

// TestUpdateStudentProgress_NonOwnerRejected: a student may only update their OWN progress.
// The test middleware authenticates user_id = 1; posting peserta_id 999 (someone else) must
// be rejected with 403, mirroring the RecordInfraction ownership check (review H2, Task 12).
func TestUpdateStudentProgress_NonOwnerRejected(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/progress", UpdateStudentProgress)
	sid := seedContentGraph(t, database, defaultTenant1Seed("ignored"))

	// user_id = 1 (set by test middleware); peserta_id 999 is a different student.
	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":999,"session_id":%d,"answered_count":5,"total_questions":10}`, sid))
	resp := doJSON(t, app, "POST", "/api/student/progress", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (non-owner progress update)", resp.StatusCode)
	}
}

// TestUpdateStudentProgress_AllowedWhenUnlocked: the happy path still records progress when
// the session is not locked (regression guard for the lock check).
func TestUpdateStudentProgress_AllowedWhenUnlocked(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/progress", UpdateStudentProgress)
	sid := seedContentGraph(t, database, defaultTenant1Seed("ignored"))

	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":1,"session_id":%d,"answered_count":5,"total_questions":10}`, sid))
	resp := doJSON(t, app, "POST", "/api/student/progress", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (unlocked progress)", resp.StatusCode)
	}
}

// TestStartExamSession_LockedRejected completes Property 11 coverage: a server-locked session
// is refused on the start path too (the serve and progress paths are covered above and in
// content_serving_test.go). The peserta is made eligible first so the 403 is specifically for
// the lock, not for ineligibility.
func TestStartExamSession_LockedRejected(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/start", StartExamSession)

	now := time.Now()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedSoalPackage(t, database, 10, 1, "Pkg", "u10")
	testutil.SeedExam(t, database, 1, 1, 1, intPtr(10))
	testutil.SeedExamSession(t, database, 1, 1, 1, fmtTime(now.Add(-time.Hour)), fmtTime(now.Add(time.Hour)), "TOK", "aktif")
	if _, err := database.Exec(`INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (1, 1)`); err != nil {
		t.Fatal(err)
	}
	// Pre-existing locked session row (the student was locked earlier this attempt).
	cekRepo := repository.NewCekLoginRepository(database)
	if err := cekRepo.Start(1, 1, 1, "prev-token"); err != nil {
		t.Fatalf("seed cek_login: %v", err)
	}
	if err := cekRepo.Lock(1, 1, 1); err != nil {
		t.Fatalf("lock: %v", err)
	}

	resp := doJSON(t, app, "POST", "/api/student/start", strings.NewReader(`{"peserta_id":1,"session_id":1}`))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (locked)", resp.StatusCode)
	}
}
