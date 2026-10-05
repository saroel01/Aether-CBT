package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// seedScopeFixture: student A (NIS-A) in ruang 1, student B (NIS-B) in ruang 2, each with
// one result containing one essay detail. Both results belong to sessions of exams that
// use different soal packages but share question_id "q1" (M9).
func seedScopeFixture(t *testing.T) *sql.DB {
	t.Helper()
	database, cleanup := testutil.NewMigratedDB(t)
	t.Cleanup(cleanup)
	prev := db.DB
	db.DB = database
	t.Cleanup(func() { db.DB = prev })

	testutil.SeedTenant(t, database, 1, "default", "Default")
	testutil.SeedKelas(t, database, 1, 1, "X")
	testutil.SeedRuang(t, database, 1, 1, "R1", "r1")
	testutil.SeedRuang(t, database, 2, 1, "R2", "r2")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "NIS-A", "Siswa A")
	testutil.SeedPeserta(t, database, 2, 1, 1, 2, "NIS-B", "Siswa B")
	testutil.SeedMapel(t, database, 1, 1, "Informatika", "INF")
	testutil.SeedSoalPackage(t, database, 10, 1, "Paket A", "ua")
	testutil.SeedSoalPackage(t, database, 20, 1, "Paket B", "ub")
	testutil.SeedExam(t, database, 1, 1, 1, intPtr(10))
	testutil.SeedExam(t, database, 2, 1, 1, intPtr(20))
	testutil.SeedExamSession(t, database, 1, 1, 1, "2026-01-01 08:00:00", "2026-01-01 10:00:00", "T1", "selesai")
	testutil.SeedExamSession(t, database, 2, 1, 2, "2026-01-01 08:00:00", "2026-01-01 10:00:00", "T2", "selesai")
	for _, q := range []string{
		`INSERT INTO hasil_tes (id, tenant_id, peserta_id, mapel_id, exam_session_id, skor, skor_maks, status, validasi)
		 VALUES (1, 1, 1, 1, 1, 10, 10, 'submitted', 'va'), (2, 1, 2, 1, 2, 0, 10, 'submitted', 'vb')`,
		`INSERT INTO hasil_tes_detail (hasil_tes_id, question_id, question_text, question_type, status, awarded_points, max_points, user_answer, correct_answer)
		 VALUES (1, 'q1', 'Soal', 'essayQuestion', 'correct', 10, 10, 'jawab A', ''),
		        (2, 'q1', 'Soal', 'essayQuestion', 'incorrect', 0, 10, 'jawab B', '')`,
	} {
		if _, err := database.Exec(q); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return database
}

func scopeApp(role string, userID int) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", role)
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Get("/export-csv", ExportResultsCSV)
	app.Get("/export-essay/:format", ExportEssayResults)
	app.Get("/essays", GetEssayAnswers)
	app.Get("/analysis", GetEducationalAnalysis)
	app.Get("/item-analysis", GetItemAnalysis)
	return app
}

func getRaw(t *testing.T, app *fiber.App, path string) (*http.Response, string) {
	t.Helper()
	resp := doJSON(t, app, "GET", path, nil)
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// M3: supervisors only see their room's results; admins see the whole tenant.
func TestResultEndpointsScopedToSupervisorRoom(t *testing.T) {
	seedScopeFixture(t)
	sup := scopeApp("supervisor", 1)
	admin := scopeApp("admin", 1)

	for _, path := range []string{"/export-csv", "/export-essay/csv", "/essays"} {
		_, body := getRaw(t, sup, path)
		if !strings.Contains(body, "NIS-A") || strings.Contains(body, "NIS-B") {
			t.Errorf("supervisor %s: want only NIS-A, got %s", path, body)
		}
		_, body = getRaw(t, admin, path)
		if !strings.Contains(body, "NIS-A") || !strings.Contains(body, "NIS-B") {
			t.Errorf("admin %s: want NIS-A and NIS-B, got %s", path, body)
		}
	}
	for _, path := range []string{"/analysis", "/item-analysis"} {
		_, body := getRaw(t, sup, path)
		if strings.Contains(body, "Paket B") || !strings.Contains(body, "Paket A") {
			t.Errorf("supervisor %s: want only Paket A stats, got %s", path, body)
		}
	}
	if resp, _ := getRaw(t, scopeApp("student", 1), "/export-csv"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("student export: status %d, want 403", resp.StatusCode)
	}
}

// M9: the same question_id in two packages yields two separate statistics rows.
func TestItemAnalysisGroupsPerPackage(t *testing.T) {
	seedScopeFixture(t)
	_, body := getRaw(t, scopeApp("admin", 1), "/item-analysis")
	var out struct {
		Data []ItemDifficultyAnalysis `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data) != 2 {
		t.Fatalf("want 2 rows (one per package), got %d: %s", len(out.Data), body)
	}
	got := map[int]ItemDifficultyAnalysis{}
	for _, r := range out.Data {
		got[r.SoalPackageID] = r
	}
	if a := got[10]; a.TotalAttempts != 1 || a.CorrectCount != 1 || a.PackageNama != "Paket A" {
		t.Errorf("package 10 = %+v", a)
	}
	if b := got[20]; b.TotalAttempts != 1 || b.CorrectCount != 0 || b.PackageNama != "Paket B" {
		t.Errorf("package 20 = %+v", b)
	}
	_, body = getRaw(t, scopeApp("admin", 1), "/item-analysis?package_id=20")
	if strings.Contains(body, "Paket A") || !strings.Contains(body, "Paket B") {
		t.Errorf("package_id filter: %s", body)
	}
}

// L4: exports over the cap are cut, flagged by header and filename.
func TestExportTruncationIsFlagged(t *testing.T) {
	seedScopeFixture(t)
	prev := maxExportRows
	maxExportRows = 1
	t.Cleanup(func() { maxExportRows = prev })

	resp, body := getRaw(t, scopeApp("admin", 1), "/export-csv")
	if resp.Header.Get("X-Export-Truncated") != "true" {
		t.Error("missing X-Export-Truncated header")
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "_TERPOTONG") {
		t.Errorf("filename not marked: %q", resp.Header.Get("Content-Disposition"))
	}
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 { // header + 1 data row
		t.Errorf("want 1 data row, got %d lines: %q", len(lines), body)
	}

	resp, _ = getRaw(t, scopeApp("admin", 1), "/export-essay/csv")
	if resp.Header.Get("X-Export-Truncated") != "true" {
		t.Error("essay export: missing X-Export-Truncated header")
	}
}
