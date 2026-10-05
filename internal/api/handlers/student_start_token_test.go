package handlers

import (
	"database/sql"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

// seedTwoSessions seeds one student eligible for two active sessions with different tokens.
func seedTwoSessions(t *testing.T, database *sql.DB) {
	t.Helper()
	now := time.Now()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedMapel(t, database, 2, 1, "Fisika", "FIS")
	testutil.SeedSoalPackage(t, database, 10, 1, "Pkg", "u10")
	testutil.SeedExam(t, database, 1, 1, 1, intPtr(10))
	testutil.SeedExam(t, database, 2, 1, 2, intPtr(10))
	testutil.SeedExamSession(t, database, 1, 1, 1, fmtTime(now.Add(-time.Hour)), fmtTime(now.Add(time.Hour)), "TOKA", "aktif")
	testutil.SeedExamSession(t, database, 2, 1, 2, fmtTime(now.Add(-time.Hour)), fmtTime(now.Add(time.Hour)), "TOKB", "aktif")
	for _, sid := range []int{1, 2} {
		if _, err := database.Exec(`INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (?, 1)`, sid); err != nil {
			t.Fatal(err)
		}
	}
}

// M1: /student/start requires the token of the requested session.
func TestStartExamSessionRequiresSessionToken(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/student/start", StartExamSession)
	seedTwoSessions(t, database)

	cases := []struct {
		name, body string
		want       int
	}{
		{"no token", `{"peserta_id":1,"session_id":1}`, http.StatusForbidden},
		{"other session token", `{"peserta_id":1,"session_id":1,"token":"TOKB"}`, http.StatusForbidden},
		{"correct token", `{"peserta_id":1,"session_id":1,"token":"TOKA"}`, http.StatusOK},
	}
	for _, tc := range cases {
		resp := doJSON(t, app, "POST", "/api/student/start", strings.NewReader(tc.body))
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.name, resp.StatusCode, tc.want)
			continue
		}
		if tc.want == http.StatusOK {
			data, _ := decodeJSON(t, resp)["data"].(map[string]interface{})
			if data["attempt_token"] == nil {
				t.Errorf("%s: missing attempt_token", tc.name)
			}
		}
	}
}

// M1: MySessions must not disclose session tokens.
func TestMySessionsHidesSessionTokens(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Get("/api/student/my-sessions", MySessions)
	seedTwoSessions(t, database)

	resp := doJSON(t, app, "GET", "/api/student/my-sessions", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(raw), `"id":1`) {
		t.Fatalf("expected sessions in response, got %s", raw)
	}
	if strings.Contains(string(raw), "TOKA") || strings.Contains(string(raw), "TOKB") {
		t.Errorf("session tokens leaked: %s", raw)
	}
}

// L6: a student cannot read another student's class mapping via ?peserta_id.
func TestGetAvailableMapelsPinsStudentToSelf(t *testing.T) {
	_, _, database, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "X-1")
	testutil.SeedKelas(t, database, 2, 1, "X-2")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "S1", "Satu")
	testutil.SeedPeserta(t, database, 2, 1, 2, 1, "S2", "Dua")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedMapel(t, database, 2, 1, "Fisika", "FIS")
	if _, err := database.Exec(`INSERT INTO kelas_mapel (tenant_id, kelas_id, mapel_id, is_active) VALUES (1, 1, 1, TRUE), (1, 2, 2, TRUE)`); err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "student")
		c.Locals("user_id", 1)
		return c.Next()
	})
	app.Get("/api/student/mapels", GetAvailableMapels)
	resp := doJSON(t, app, "GET", "/api/student/mapels?peserta_id=2", nil)
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(raw), "Kimia") || strings.Contains(string(raw), "Fisika") {
		t.Errorf("status %d body %s: want only student 1's mapel (Kimia)", resp.StatusCode, raw)
	}
}
