package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

// fmtTime formats a time as the canonical SQLite datetime string (UTC), matching how the
// repository stores exam_session timestamps so comparisons against the real clock hold.
func fmtTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") }

func decodeJSON(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(body, &m)
	return m
}

func TestStudentLogin_EffectiveSessionToken(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/auth/student-login", StudentLogin)

	now := time.Now()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	testutil.SeedExamSession(t, database, 1, 1, 1, fmtTime(now.Add(-time.Hour)), fmtTime(now.Add(time.Hour)), "EFFECTIVE", "terjadwal")

	resp := doJSON(t, app, "POST", "/api/auth/student-login", strings.NewReader(`{"no_id":"2026001","password":"siswa123","token":"EFFECTIVE"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	data, _ := body["data"].(map[string]interface{})
	if data["session_id"] == nil {
		t.Errorf("expected session_id in response data, got %v", body)
	}
}

func TestStudentLogin_SessionNotStartedYet(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/auth/student-login", StudentLogin)

	now := time.Now()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	testutil.SeedExamSession(t, database, 1, 1, 1, fmtTime(now.Add(time.Hour)), fmtTime(now.Add(2*time.Hour)), "FUTURE", "terjadwal")

	resp := doJSON(t, app, "POST", "/api/auth/student-login", strings.NewReader(`{"no_id":"2026001","password":"siswa123","token":"FUTURE"}`))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (not started)", resp.StatusCode)
	}
	if msg, _ := decodeJSON(t, resp)["error"].(string); !strings.Contains(strings.ToLower(msg), "start") {
		t.Errorf("error message %q should indicate not started", msg)
	}
}

func TestStudentLogin_LegacyTokenFallback(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/auth/student-login", StudentLogin)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	if _, err := database.Exec(`INSERT INTO settings (tenant_id, exam_title, token, is_exam_active) VALUES (1, 'T', 'LEGACY', 1) ON CONFLICT(tenant_id) DO UPDATE SET token = 'LEGACY'`); err != nil {
		t.Fatal(err)
	}

	resp := doJSON(t, app, "POST", "/api/auth/student-login", strings.NewReader(`{"no_id":"2026001","password":"siswa123","token":"LEGACY"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("legacy login status = %d, want 200", resp.StatusCode)
	}
}

func TestMySessions(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Get("/api/student/my-sessions", MySessions)

	now := time.Now()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	testutil.SeedExamSession(t, database, 1, 1, 1, fmtTime(now.Add(-time.Hour)), fmtTime(now.Add(time.Hour)), "TOK", "terjadwal")
	if _, err := database.Exec(`INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (1, 1)`); err != nil {
		t.Fatal(err)
	}

	resp := doJSON(t, app, "GET", "/api/student/my-sessions", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	data, _ := body["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 session in data, got %d (%v)", len(data), body)
	}
}

func TestStartExamSession_EligibleSetsAttemptTokenAndCookie(t *testing.T) {
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

	resp := doJSON(t, app, "POST", "/api/student/start", strings.NewReader(`{"peserta_id":1,"session_id":1}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	data, _ := body["data"].(map[string]interface{})
	if data["attempt_token"] == nil {
		t.Errorf("expected attempt_token in response data, got %v", body)
	}
	// Content cookie issued (AD-2).
	if c := resp.Header.Get("Set-Cookie"); !strings.Contains(c, "aether_exam=") {
		t.Errorf("expected aether_exam cookie, got %q", c)
	}
}

func TestStartExamSession_NotEligibleForbidden(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/start", StartExamSession)

	now := time.Now()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedKelas(t, database, 2, 1, "XII IPA 2") // a class the student is NOT in
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	testutil.SeedExamSession(t, database, 1, 1, 1, fmtTime(now.Add(-time.Hour)), fmtTime(now.Add(time.Hour)), "TOK", "aktif")
	if _, err := database.Exec(`INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (1, 2)`); err != nil { // linked to kelas 2, not the student's kelas 1
		t.Fatal(err)
	}

	resp := doJSON(t, app, "POST", "/api/student/start", strings.NewReader(`{"peserta_id":1,"session_id":1}`))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (not eligible)", resp.StatusCode)
	}
}

func TestUpdateStudentProgress_SessionBased(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/start", StartExamSession)
	app.Post("/api/student/progress", UpdateStudentProgress)

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
	// Start the session first so a progress row exists to update.
	startResp := doJSON(t, app, "POST", "/api/student/start", strings.NewReader(`{"peserta_id":1,"session_id":1}`))
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d, want 200 (body=%v)", startResp.StatusCode, decodeJSON(t, startResp))
	}

	resp := doJSON(t, app, "POST", "/api/student/progress", strings.NewReader(`{"peserta_id":1,"session_id":1,"answered_count":5,"total_questions":10}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("progress status = %d, want 200", resp.StatusCode)
	}
}

func TestGetRemainingTime_SessionBasedClamp(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/start", StartExamSession)
	app.Get("/api/student/remaining-time", GetRemainingTime)

	now := time.Now()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedSoalPackage(t, database, 10, 1, "Pkg", "u10")
	testutil.SeedExam(t, database, 1, 1, 1, intPtr(10))
	// 30-minute window remaining; exam duration 90 min -> remaining must clamp to ~30 min.
	testutil.SeedExamSession(t, database, 1, 1, 1, fmtTime(now.Add(-time.Hour)), fmtTime(now.Add(30*time.Minute)), "TOK", "aktif")
	if _, err := database.Exec(`INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (1, 1)`); err != nil {
		t.Fatal(err)
	}
	doJSON(t, app, "POST", "/api/student/start", strings.NewReader(`{"peserta_id":1,"session_id":1}`))

	resp := doJSON(t, app, "GET", "/api/student/remaining-time?session_id=1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	data, _ := body["data"].(map[string]interface{})
	remaining, _ := data["remaining_seconds"].(float64)
	// Duration is 90 min (5400s) but only ~30 min remains in the window -> clamp to window.
	if remaining <= 0 || remaining > 35*60 {
		t.Errorf("remaining_seconds = %v, expected clamped to ~30 min (<= 2100)", remaining)
	}
}
