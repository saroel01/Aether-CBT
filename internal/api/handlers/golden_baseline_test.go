package handlers

// Golden baseline capture for preservation clause 3.13 (codebase-bug-sweep, tasks 1.1 / 1.2).
//
// WHY THIS FILE EXISTS
// -------------------
// Clause 3.13 requires that, for every row that scans cleanly (the ¬C domain), each list and
// export endpoint returns an IDENTICAL payload, row order, and format before and after the
// bug sweep. F (unfixed) and F' (fixed) cannot be executed side by side in one process, so the
// only usable evidence is a recorded artefact. The files under testdata/golden/ were recorded
// from the UNFIXED code and are the reference F. Task 6.29 re-runs these same tests against F'
// and asserts the bytes still match.
//
// DO NOT regenerate these files as part of a fix. Regeneration (`go test ./internal/api/handlers
// -run TestGolden -update`) destroys the baseline and with it the only proof of 3.13.
//
// The dataset is deliberately inside ¬C: no NULL in any scanned column, no scan error, every FK
// referentially valid (so the same seed still works once clause 2.1 turns foreign_keys on), and
// every timestamp pinned to a literal so nothing depends on wall-clock time.

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// updateGolden regenerates the committed baseline. Guarded behind a flag so a normal
// `go test ./...` can never silently overwrite the F reference.
var updateGolden = flag.Bool("update", false,
	"regenerate the codebase-bug-sweep golden baseline under testdata/golden (destroys the pre-fix reference)")

const goldenDir = "testdata/golden"

// assertGolden compares got against testdata/golden/<name>, or writes it when -update is set.
// Comparison is byte-exact: clause 3.13 covers payload, ordering, AND format.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join(goldenDir, name)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("golden updated: %s (%d bytes)", path, len(got))
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run: go test ./internal/api/handlers -run TestGolden -update)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden mismatch for %s: clause 3.13 requires an identical payload, ordering, and format.\n--- want (%d bytes) ---\n%s\n--- got (%d bytes) ---\n%s",
			path, len(want), truncateForLog(want), len(got), truncateForLog(got))
	}
}

// assertGoldenJSON records a derived structure (XLSX cells, PDF page text) as indented JSON so
// the committed baseline stays reviewable in a diff.
func assertGoldenJSON(t *testing.T, name string, v interface{}) {
	t.Helper()
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden %s: %v", name, err)
	}
	assertGolden(t, name, append(encoded, '\n'))
}

func truncateForLog(b []byte) string {
	const max = 4000
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "\n...[truncated]"
}

// newGoldenApp returns a Fiber app whose locals mimic an authenticated caller in tenant 1,
// backed by a freshly migrated database exposed as the package-global db.DB (the convention
// every handler test in this package already uses).
func newGoldenApp(t *testing.T, role string) (*fiber.App, *sql.DB) {
	t.Helper()
	database, cleanup := testutil.NewMigratedDB(t)
	t.Cleanup(cleanup)
	db.DB = database

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", role)
		c.Locals("user_id", 1)
		return c.Next()
	})
	seedGoldenBaseline(t, database)
	return app, database
}

// getBody issues a GET and returns the status plus the raw response bytes, unmodified.
func getBody(t *testing.T, app *fiber.App, path string) (int, []byte) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil), -1)
	if err != nil {
		t.Fatalf("app.Test GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body GET %s: %v", path, err)
	}
	return resp.StatusCode, buf.Bytes()
}

// getOK issues a GET, requires 200, and returns the raw response bytes.
func getOK(t *testing.T, app *fiber.App, path string) []byte {
	t.Helper()
	status, body := getBody(t, app, path)
	if status != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200 (body: %s)", path, status, truncateForLog(body))
	}
	return body
}

// Fixed timestamps. Every column the handlers under test scan is pinned to a literal so the
// recorded baseline never depends on when the suite runs.
const (
	goldenCreatedAt = "2026-01-05 07:00:00"
	goldenUpdatedAt = "2026-01-05 07:00:00"
)

// seedGoldenBaseline installs the deterministic dataset shared by tasks 1.1 and 1.2.
//
// Tenant 1 is the subject under test; tenant 2 exists purely to prove the recorded payloads
// stay tenant-scoped. Identifier orders are deliberately mismatched (row id order != no_id
// order != nama order != nama_kelas order) so a golden file actually pins the ORDER BY rather
// than accidentally agreeing with insertion order.
func seedGoldenBaseline(t *testing.T, database *sql.DB) {
	t.Helper()

	testutil.SeedTenant(t, database, 1, "default", "SMA Negeri Aether")
	testutil.SeedTenant(t, database, 2, "other", "SMA Tetangga")
	// Migration 001 pre-seeds tenant 1 as "Default School" and SeedTenant is INSERT OR IGNORE,
	// so force the name: ExportEssayResults prints it into the PDF header.
	mustExec(t, database, `UPDATE tenants SET name = 'SMA Negeri Aether' WHERE id = 1`)

	// kelas: "XI IPS 2" sorts BEFORE "XII IPA 1" (byte 2 is ' ' vs 'I'), i.e. the opposite of
	// id order, which is what the ORDER BY k.nama_kelas in both exports must reproduce.
	testutil.SeedKelas(t, database, 10, 1, "XI IPS 2")
	testutil.SeedKelas(t, database, 11, 1, "XII IPA 1")
	testutil.SeedKelas(t, database, 20, 2, "ZZ Kelas Tenant Lain")

	testutil.SeedRuang(t, database, 10, 1, "Ruang Bawah", "ruang_bawah")
	testutil.SeedRuang(t, database, 11, 1, "Ruang Atas", "ruang_atas")
	testutil.SeedRuang(t, database, 20, 2, "Ruang Tenant Lain", "ruang_lain")

	testutil.SeedMapel(t, database, 10, 1, "Matematika", "MAT")
	testutil.SeedMapel(t, database, 11, 1, "Bahasa Indonesia", "BIN")
	testutil.SeedMapel(t, database, 12, 1, "Kimia", "KIM")
	testutil.SeedMapel(t, database, 20, 2, "Mapel Tenant Lain", "ZZZ")

	// kelas_mapel: kelas 11 -> {Matematika, Bahasa Indonesia} active, Kimia INACTIVE;
	// kelas 10 -> {Kimia} active. The inactive row exercises the is_active filter.
	mustExec(t, database, `INSERT INTO kelas_mapel (id, kelas_id, mapel_id, tenant_id, is_active) VALUES
		(1, 11, 10, 1, TRUE),
		(2, 11, 11, 1, TRUE),
		(3, 11, 12, 1, FALSE),
		(4, 10, 12, 1, TRUE),
		(5, 20, 20, 2, TRUE)`)

	// peserta: id order, no_id order, and nama order all differ from one another.
	testutil.SeedPeserta(t, database, 100, 1, 11, 10, "2026003", "Ahmad Zaki")
	testutil.SeedPeserta(t, database, 101, 1, 11, 10, "2026001", "Citra Dewi")
	testutil.SeedPeserta(t, database, 102, 1, 10, 11, "2026002", "Budi Santoso")
	testutil.SeedPeserta(t, database, 200, 2, 20, 20, "9999001", "Siswa Tenant Lain")
	mustExec(t, database, `UPDATE peserta SET jenis_kelamin = 'L' WHERE id IN (100, 102, 200)`)
	mustExec(t, database, `UPDATE peserta SET jenis_kelamin = 'P' WHERE id = 101`)

	// hasil_tes: skor and skor_maks are always NOT NULL, so every row sits in ¬C for clause
	// 1.16 (the NULL-skor row that vanishes from the CSV is a C-domain case recorded elsewhere).
	mustExec(t, database, `INSERT INTO hasil_tes
		(id, tenant_id, peserta_id, mapel_id, skor, skor_maks, kkm, durasi_kerja, waktu_mulai, waktu_selesai, status, validasi, created_at, updated_at) VALUES
		(1000, 1, 100, 10, 85.5,  100.0, 75.0, 3600, '2026-03-01 07:00:00', '2026-03-01 08:00:00', 'submitted', '1_2026003_10', '2026-03-01 08:00:00', '2026-03-01 08:00:00'),
		(1001, 1, 101, 10, 70.0,  100.0, 75.0, 3300, '2026-03-01 07:00:00', '2026-03-01 07:55:00', 'submitted', '1_2026001_10', '2026-03-01 08:05:00', '2026-03-01 08:05:00'),
		(1002, 1, 102, 12, 92.25, 100.0, 75.0, 3000, '2026-03-01 07:00:00', '2026-03-01 07:50:00', 'submitted', '1_2026002_12', '2026-03-01 08:10:00', '2026-03-01 08:10:00'),
		(2000, 2, 200, 20, 10.0,  100.0, 75.0, 600,  '2026-03-01 07:00:00', '2026-03-01 07:10:00', 'submitted', '2_9999001_20', '2026-03-01 08:15:00', '2026-03-01 08:15:00')`)

	// hasil_tes_detail: ASCII only. gofpdf renders with the core Arial/Helvetica font, so a
	// non-Latin-1 rune would be silently mangled and make the PDF baseline hard to read.
	// Q01 spans three attempts with three different statuses, Q02 two, Q03 one -> the GROUP BY
	// in GetEducationalAnalysis / GetItemAnalysis produces three distinct difficulty buckets.
	mustExec(t, database, `INSERT INTO hasil_tes_detail
		(id, hasil_tes_id, question_id, question_text, question_type, status, awarded_points, max_points, user_answer, correct_answer, attempts_used, created_at) VALUES
		(5000, 1000, 'Q01', 'Jelaskan hukum Ohm.',            'essayQuestion',          'correct',   10.0, 10.0, 'V = I x R',                          'V = I x R',                          1, '2026-03-01 08:00:00'),
		(5001, 1000, 'Q02', 'Ibu kota Indonesia?',            'multipleChoiceQuestion', 'correct',    5.0,  5.0, 'Jakarta',                            'Jakarta',                            1, '2026-03-01 08:00:00'),
		(5002, 1001, 'Q01', 'Jelaskan hukum Ohm.',            'essayQuestion',          'partial',    5.0, 10.0, 'Tegangan sebanding dengan arus',      'V = I x R',                          1, '2026-03-01 08:05:00'),
		(5003, 1001, 'Q02', 'Ibu kota Indonesia?',            'multipleChoiceQuestion', 'incorrect',  0.0,  5.0, 'Bandung',                            'Jakarta',                            1, '2026-03-01 08:05:00'),
		(5004, 1002, 'Q03', 'Sebutkan tiga sifat asam.',      'essayQuestion',          'correct',    8.0,  8.0, 'Rasa asam, pH di bawah 7, korosif',  'Rasa asam, pH di bawah 7, korosif',  1, '2026-03-01 08:10:00'),
		(5005, 1002, 'Q01', 'Jelaskan hukum Ohm.',            'essayQuestion',          'incorrect',  0.0, 10.0, 'Belum sempat menjawab.',             'V = I x R',                          1, '2026-03-01 08:10:00'),
		(6000, 2000, 'Q90', 'Soal tenant lain.',              'essayQuestion',          'correct',    4.0,  4.0, 'Jawaban tenant lain',                'Jawaban tenant lain',                1, '2026-03-01 08:15:00')`)

	// peserta.created_at and ruang.created_at default to CURRENT_TIMESTAMP and are BOTH scanned
	// into the GetStudents / GetRooms payloads. Pin them, otherwise the golden captures the
	// moment the recording ran and can never be reproduced.
	mustExec(t, database, `UPDATE peserta SET created_at = ?, updated_at = ?`, goldenCreatedAt, goldenUpdatedAt)
	mustExec(t, database, `UPDATE ruang   SET created_at = ?, updated_at = ?`, goldenCreatedAt, goldenUpdatedAt)
	mustExec(t, database, `UPDATE kelas   SET created_at = ?, updated_at = ?`, goldenCreatedAt, goldenUpdatedAt)
	mustExec(t, database, `UPDATE mapel   SET created_at = ?, updated_at = ?`, goldenCreatedAt, goldenUpdatedAt)
}

func mustExec(t *testing.T, database *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatalf("seed exec failed: %v\nquery: %s", err, query)
	}
}

// TestGoldenBaselineDatasetIsInNonBugDomain guards the premise of both golden tasks: the seed
// must contain no NULL in any column the handlers under test scan. If a future edit introduces
// one, the recorded payloads would no longer be evidence for clause 3.13 (they would sit in the
// C domain of clause 1.16 instead), so fail loudly rather than record a misleading baseline.
func TestGoldenBaselineDatasetIsInNonBugDomain(t *testing.T) {
	_, database := newGoldenApp(t, "admin")

	checks := []struct {
		name  string
		query string
	}{
		{"hasil_tes.skor / skor_maks / status / created_at", `SELECT COUNT(*) FROM hasil_tes WHERE skor IS NULL OR skor_maks IS NULL OR status IS NULL OR created_at IS NULL`},
		{"hasil_tes_detail scanned columns", `SELECT COUNT(*) FROM hasil_tes_detail WHERE question_id IS NULL OR question_text IS NULL OR question_type IS NULL OR status IS NULL OR awarded_points IS NULL OR max_points IS NULL OR user_answer IS NULL OR correct_answer IS NULL`},
		{"peserta scanned columns", `SELECT COUNT(*) FROM peserta WHERE no_id IS NULL OR nama_peserta IS NULL OR kelas_id IS NULL OR ruang_id IS NULL OR created_at IS NULL`},
		{"ruang scanned columns", `SELECT COUNT(*) FROM ruang WHERE nama_ruang IS NULL OR username IS NULL OR created_at IS NULL`},
		{"mapel scanned columns", `SELECT COUNT(*) FROM mapel WHERE nama_mapel IS NULL OR kode_mapel IS NULL`},
		{"kelas scanned columns", `SELECT COUNT(*) FROM kelas WHERE nama_kelas IS NULL`},
	}
	for _, ch := range checks {
		var n int
		if err := database.QueryRow(ch.query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", ch.name, err)
		}
		if n != 0 {
			t.Errorf("%s: %d row(s) contain NULL; the golden baseline dataset must stay inside the non-bug domain", ch.name, n)
		}
	}

	// Referential integrity: the same seed has to survive clause 2.1 switching foreign_keys on.
	rows, err := database.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	defer rows.Close()
	var violations []string
	for rows.Next() {
		var table, parent sql.NullString
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			t.Fatalf("scan foreign_key_check: %v", err)
		}
		violations = append(violations, fmt.Sprintf("%s rowid=%d -> %s", table.String, rowid.Int64, parent.String))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign_key_check: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("golden seed violates %d foreign key(s): %v", len(violations), violations)
	}
}
