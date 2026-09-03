package handlers

// Failing-first tests for codebase-bug-sweep clause 1.2 / 2.2 (bug condition C2).
//
// C2: a peserta write path receives no kelas_id / ruang_id (absent, or supplied as the
// sentinel 0) and stores 0 into peserta.kelas_id / peserta.ruang_id. Those columns are
// declared FOREIGN KEY -> kelas(id) / ruang(id) in migration 009, and no parent row with
// id 0 exists, so every such row is a latent referential-integrity violation. It only
// survives today because the DSN in internal/db/sqlite.go never actually turns
// foreign_keys on (clause 1.1). Gate 1 flips that pragma on for the first time, so these
// rows must be gone BEFORE it lands.
//
// Expected (2.2): absent or 0 normalizes to NULL; a value > 0 that exists in the parent
// table is stored unchanged (preservation clause 3.2).

import (
	"bytes"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// newStudentApp wires a migrated database into the package-global db.DB (the convention
// used by every handler test here) and returns a Fiber app whose locals mimic an
// authenticated admin in tenant 1, with kelas 1 / ruang 1 seeded as valid parents.
func newStudentApp(t *testing.T) (*fiber.App, *sql.DB) {
	t.Helper()
	database, cleanup := testutil.NewMigratedDB(t)
	t.Cleanup(cleanup)
	db.DB = database

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "admin")
		c.Locals("user_id", 1)
		return c.Next()
	})
	app.Post("/api/students", CreateStudent)
	app.Post("/admin/students/import-csv", ImportStudentsCSV)
	return app, database
}

// postJSON issues a JSON POST and returns the status code plus the raw body.
func postJSON(t *testing.T, app *fiber.App, path, body string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body POST %s: %v", path, err)
	}
	return resp.StatusCode, buf.Bytes()
}

// readPesertaFK returns peserta.kelas_id / peserta.ruang_id as nullable integers so a
// stored NULL is distinguishable from a stored 0 — the whole point of clause 2.2.
func readPesertaFK(t *testing.T, database *sql.DB, noID string) (sql.NullInt64, sql.NullInt64) {
	t.Helper()
	var kelasID, ruangID sql.NullInt64
	err := database.QueryRow(
		`SELECT kelas_id, ruang_id FROM peserta WHERE tenant_id = 1 AND no_id = ?`, noID,
	).Scan(&kelasID, &ruangID)
	if err != nil {
		t.Fatalf("read peserta %s: %v", noID, err)
	}
	return kelasID, ruangID
}

func describeFK(v sql.NullInt64) string {
	if !v.Valid {
		return "NULL"
	}
	return fmt.Sprintf("%d", v.Int64)
}

// TestCreateStudentNormalizesSentinelFKToNull covers bug condition C2 on the handler write
// path. "Not supplied" and "supplied as 0" must both land as NULL; a valid > 0 reference
// must be stored unchanged (clause 3.2).
//
// On F (unfixed) the absent and explicit-0 cases store 0 and FAIL.
func TestCreateStudentNormalizesSentinelFKToNull(t *testing.T) {
	cases := []struct {
		name      string
		noID      string
		body      string
		wantKelas sql.NullInt64
		wantRuang sql.NullInt64
	}{
		{
			name:      "both absent normalizes to NULL",
			noID:      "C2-absent",
			body:      `{"no_id":"C2-absent","nama_peserta":"Tanpa Kelas","jenis_kelamin":"L"}`,
			wantKelas: sql.NullInt64{},
			wantRuang: sql.NullInt64{},
		},
		{
			name:      "explicit zero normalizes to NULL",
			noID:      "C2-zero",
			body:      `{"no_id":"C2-zero","nama_peserta":"Nol Sentinel","kelas_id":0,"ruang_id":0,"jenis_kelamin":"L"}`,
			wantKelas: sql.NullInt64{},
			wantRuang: sql.NullInt64{},
		},
		{
			name:      "kelas absent while ruang valid normalizes only kelas",
			noID:      "C2-mixed",
			body:      `{"no_id":"C2-mixed","nama_peserta":"Setengah","ruang_id":1,"jenis_kelamin":"P"}`,
			wantKelas: sql.NullInt64{},
			wantRuang: sql.NullInt64{Int64: 1, Valid: true},
		},
		{
			name:      "valid references are stored as-is",
			noID:      "C2-valid",
			body:      `{"no_id":"C2-valid","nama_peserta":"Lengkap","kelas_id":1,"ruang_id":1,"jenis_kelamin":"L"}`,
			wantKelas: sql.NullInt64{Int64: 1, Valid: true},
			wantRuang: sql.NullInt64{Int64: 1, Valid: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, database := newStudentApp(t)

			status, body := postJSON(t, app, "/api/students", tc.body)
			if status != fiber.StatusOK {
				t.Fatalf("CreateStudent status = %d, want 200 (body: %s)", status, body)
			}

			kelasID, ruangID := readPesertaFK(t, database, tc.noID)
			if kelasID != tc.wantKelas {
				t.Errorf("peserta.kelas_id = %s, want %s (clause 2.2: sentinel 0 must never reach a FOREIGN KEY column)",
					describeFK(kelasID), describeFK(tc.wantKelas))
			}
			if ruangID != tc.wantRuang {
				t.Errorf("peserta.ruang_id = %s, want %s (clause 2.2: sentinel 0 must never reach a FOREIGN KEY column)",
					describeFK(ruangID), describeFK(tc.wantRuang))
			}
		})
	}
}

// TestCreateStudentRowSurvivesForeignKeyCheck is the integrity-level statement of the same
// clause: whatever CreateStudent writes must satisfy PRAGMA foreign_key_check, which is
// exactly the Gate 0 exit criterion. On F the absent-FK row reports two violations.
func TestCreateStudentRowSurvivesForeignKeyCheck(t *testing.T) {
	app, database := newStudentApp(t)

	status, body := postJSON(t, app, "/api/students",
		`{"no_id":"C2-fkcheck","nama_peserta":"Tanpa Kelas","jenis_kelamin":"L"}`)
	if status != fiber.StatusOK {
		t.Fatalf("CreateStudent status = %d, want 200 (body: %s)", status, body)
	}

	violations := foreignKeyViolations(t, database)
	if len(violations) != 0 {
		t.Errorf("PRAGMA foreign_key_check reported %d violation(s) after CreateStudent: %v",
			len(violations), violations)
	}
}

// TestPesertaSentinelZeroIsRejectedWhenForeignKeysEnforced proves the violation is real
// rather than theoretical: on a connection with foreign_keys actually ON, inserting
// kelas_id = 0 must be rejected by SQLite. This is the latent failure Gate 1 will turn
// into a hard production error.
//
// MaxOpenConns(1) pins the pool to a single connection, because PRAGMA foreign_keys is
// per-connection and would otherwise apply to an arbitrary member of the pool.
func TestPesertaSentinelZeroIsRejectedWhenForeignKeysEnforced(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "fk-on.db")
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)

	if err := db.RunMigrations(database, migrationsDirForTest(t)); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := database.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign_keys: %v", err)
	}
	var fkOn int
	if err := database.QueryRow(`PRAGMA foreign_keys`).Scan(&fkOn); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fkOn != 1 {
		t.Fatalf("precondition: foreign_keys = %d, want 1", fkOn)
	}

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")

	_, err = database.Exec(
		`INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
		 VALUES (1, 'sentinel', 'hash', 'Sentinel Nol', 0, 0)`)
	if err == nil {
		t.Fatal("INSERT peserta with kelas_id = 0 succeeded under foreign_keys = ON; " +
			"the sentinel is supposed to violate the FOREIGN KEY to kelas(id)")
	}
	t.Logf("sentinel INSERT correctly rejected: %v", err)

	// And the normalized form must be accepted, otherwise clause 2.2 is unimplementable.
	if _, err := database.Exec(
		`INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
		 VALUES (1, 'normalized', 'hash', 'Tanpa Kelas', NULL, NULL)`); err != nil {
		t.Errorf("INSERT peserta with NULL kelas_id/ruang_id was rejected: %v\n"+
			"clause 2.2 requires NULL to be a storable representation of \"not assigned\"", err)
	}
}

// TestImportStudentsCSVNormalizesSentinelFKToNull covers the CSV write path. A blank or 0
// kelas_id / ruang_id cell must land as NULL, never as the sentinel 0; a valid reference is
// stored unchanged (clause 3.2).
func TestImportStudentsCSVNormalizesSentinelFKToNull(t *testing.T) {
	cases := []struct {
		name      string
		row       string
		noID      string
		wantKelas sql.NullInt64
		wantRuang sql.NullInt64
	}{
		{
			name:      "blank cells normalize to NULL",
			row:       "CSV-blank,Siswa Kosong,,,L\n",
			noID:      "CSV-blank",
			wantKelas: sql.NullInt64{},
			wantRuang: sql.NullInt64{},
		},
		{
			name:      "zero cells normalize to NULL",
			row:       "CSV-zero,Siswa Nol,0,0,L\n",
			noID:      "CSV-zero",
			wantKelas: sql.NullInt64{},
			wantRuang: sql.NullInt64{},
		},
		{
			name:      "valid references are stored as-is",
			row:       "CSV-valid,Siswa Lengkap,1,1,P\n",
			noID:      "CSV-valid",
			wantKelas: sql.NullInt64{Int64: 1, Valid: true},
			wantRuang: sql.NullInt64{Int64: 1, Valid: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, database := newStudentApp(t)

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "students.csv")
			if err != nil {
				t.Fatalf("create csv part: %v", err)
			}
			if _, err := part.Write([]byte("no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\n" + tc.row)); err != nil {
				t.Fatalf("write csv part: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("close multipart writer: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/admin/students/import-csv", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("import request failed: %v", err)
			}
			defer resp.Body.Close()
			var respBody bytes.Buffer
			_, _ = respBody.ReadFrom(resp.Body)
			if resp.StatusCode != fiber.StatusOK {
				t.Fatalf("import status = %d, want 200 (body: %s)", resp.StatusCode, respBody.String())
			}

			kelasID, ruangID := readPesertaFK(t, database, tc.noID)
			if kelasID != tc.wantKelas {
				t.Errorf("peserta.kelas_id = %s, want %s", describeFK(kelasID), describeFK(tc.wantKelas))
			}
			if ruangID != tc.wantRuang {
				t.Errorf("peserta.ruang_id = %s, want %s", describeFK(ruangID), describeFK(tc.wantRuang))
			}
		})
	}
}

// foreignKeyViolations returns one description per PRAGMA foreign_key_check row.
func foreignKeyViolations(t *testing.T, database *sql.DB) []string {
	t.Helper()
	rows, err := database.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var table, parent sql.NullString
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			t.Fatalf("scan foreign_key_check: %v", err)
		}
		out = append(out, fmt.Sprintf("%s rowid=%d -> %s (fk #%d)",
			table.String, rowid.Int64, parent.String, fkid.Int64))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign_key_check: %v", err)
	}
	return out
}

// migrationsDirForTest resolves internal/db/migrations from this package's directory.
func migrationsDirForTest(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatalf("resolve migrations dir: %v", err)
	}
	return abs
}

// TestCreateStudentPreservesValidReferencesAndResponses is the preservation half of clause 2.2
// (clause 3.2): for input OUTSIDE the bug condition — kelas_id / ruang_id supplied as a
// positive id that exists in the caller's tenant — nothing may change. The stored relation is
// byte-for-byte the id that was sent, and every response body (success and the two validation
// failures) is exactly what the pre-fix code returned.
func TestCreateStudentPreservesValidReferencesAndResponses(t *testing.T) {
	app, database := newStudentApp(t)

	// Success: unchanged body, unchanged stored relation.
	status, body := postJSON(t, app, "/api/students",
		`{"no_id":"P32-valid","nama_peserta":"Siswa Sah","kelas_id":1,"ruang_id":1,"jenis_kelamin":"L"}`)
	if status != fiber.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", status, body)
	}
	const wantSuccess = `{"success":true,"message":"Student created successfully"}`
	if string(body) != wantSuccess {
		t.Errorf("success body = %s, want %s (clause 3.2: the response must be identical)", body, wantSuccess)
	}
	kelasID, ruangID := readPesertaFK(t, database, "P32-valid")
	if kelasID != (sql.NullInt64{Int64: 1, Valid: true}) || ruangID != (sql.NullInt64{Int64: 1, Valid: true}) {
		t.Errorf("stored references = %v / %v, want 1 / 1", kelasID, ruangID)
	}

	// A reference that exists but belongs to another tenant is still rejected, with the same
	// message and status as before.
	testutil.SeedTenant(t, database, 2, "other", "SMA Tetangga")
	testutil.SeedKelas(t, database, 77, 2, "Kelas Tenant Lain")
	testutil.SeedRuang(t, database, 78, 2, "Ruang Tenant Lain", "ruang_lain")

	for _, tc := range []struct {
		name     string
		body     string
		wantBody string
	}{
		{
			name:     "cross-tenant kelas",
			body:     `{"no_id":"P32-kelas","nama_peserta":"X","kelas_id":77,"ruang_id":1,"jenis_kelamin":"L"}`,
			wantBody: `{"success":false,"error":"class not found in tenant"}`,
		},
		{
			name:     "cross-tenant ruang",
			body:     `{"no_id":"P32-ruang","nama_peserta":"X","kelas_id":1,"ruang_id":78,"jenis_kelamin":"L"}`,
			wantBody: `{"success":false,"error":"room not found in tenant"}`,
		},
		{
			name:     "missing no_id",
			body:     `{"nama_peserta":"X","kelas_id":1,"ruang_id":1,"jenis_kelamin":"L"}`,
			wantBody: `{"success":false,"error":"no_id is required"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, body := postJSON(t, app, "/api/students", tc.body)
			if status != fiber.StatusBadRequest {
				t.Errorf("status = %d, want 400", status)
			}
			if string(body) != tc.wantBody {
				t.Errorf("body = %s, want %s", body, tc.wantBody)
			}
		})
	}

	// Duplicate no_id keeps its 409 and message.
	status, body = postJSON(t, app, "/api/students",
		`{"no_id":"P32-valid","nama_peserta":"Duplikat","kelas_id":1,"ruang_id":1,"jenis_kelamin":"L"}`)
	if status != fiber.StatusConflict {
		t.Errorf("duplicate status = %d, want 409", status)
	}
	const wantConflict = `{"success":false,"error":"no_id already exists in this tenant"}`
	if string(body) != wantConflict {
		t.Errorf("duplicate body = %s, want %s", body, wantConflict)
	}
}

// TestImportStudentsCSVPreservesValidReferencesAndResponses is the clause 3.2 counterpart for
// the CSV path: a fully populated row stores the same relation and returns the same body, and
// the existing per-row validation failures keep their exact status and message.
func TestImportStudentsCSVPreservesValidReferencesAndResponses(t *testing.T) {
	postCSV := func(t *testing.T, app *fiber.App, rows string) (int, []byte) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "students.csv")
		if err != nil {
			t.Fatalf("create csv part: %v", err)
		}
		if _, err := part.Write([]byte("no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\n" + rows)); err != nil {
			t.Fatalf("write csv part: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close multipart writer: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/students/import-csv", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("import request failed: %v", err)
		}
		defer resp.Body.Close()
		var out bytes.Buffer
		if _, err := out.ReadFrom(resp.Body); err != nil {
			t.Fatalf("read import body: %v", err)
		}
		return resp.StatusCode, out.Bytes()
	}

	t.Run("valid rows are unchanged", func(t *testing.T) {
		app, database := newStudentApp(t)
		status, body := postCSV(t, app, "CSV-P1,Siswa Satu,1,1,L\nCSV-P2,Siswa Dua,1,1,P\n")
		if status != fiber.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", status, body)
		}
		const wantBody = `{"success":true,"message":"Import succeeded","data":{"imported":2}}`
		if string(body) != wantBody {
			t.Errorf("body = %s, want %s (clause 3.2)", body, wantBody)
		}
		for _, noID := range []string{"CSV-P1", "CSV-P2"} {
			kelasID, ruangID := readPesertaFK(t, database, noID)
			if kelasID != (sql.NullInt64{Int64: 1, Valid: true}) || ruangID != (sql.NullInt64{Int64: 1, Valid: true}) {
				t.Errorf("%s stored references = %v / %v, want 1 / 1", noID, kelasID, ruangID)
			}
		}
	})

	t.Run("existing validation failures keep their message", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			rows     string
			wantBody string
		}{
			{
				name:     "non numeric kelas_id",
				rows:     "CSV-bad,Siswa,abc,1,L\n",
				wantBody: `{"success":false,"error":"Row 2: missing/invalid no_id, nama, kelas_id, or ruang_id"}`,
			},
			{
				name:     "negative kelas_id",
				rows:     "CSV-neg,Siswa,-3,1,L\n",
				wantBody: `{"success":false,"error":"Row 2: missing/invalid no_id, nama, kelas_id, or ruang_id"}`,
			},
			{
				name:     "blank nama",
				rows:     "CSV-noname,,1,1,L\n",
				wantBody: `{"success":false,"error":"Row 2: missing/invalid no_id, nama, kelas_id, or ruang_id"}`,
			},
			{
				name:     "unknown kelas_id",
				rows:     "CSV-unknown,Siswa,999,1,L\n",
				wantBody: `{"success":false,"error":"Row 2: kelas_id 999 not found in tenant"}`,
			},
			{
				name:     "unknown ruang_id",
				rows:     "CSV-unknownruang,Siswa,1,999,L\n",
				wantBody: `{"success":false,"error":"Row 2: ruang_id 999 not found in tenant"}`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				app, _ := newStudentApp(t)
				status, body := postCSV(t, app, tc.rows)
				if status != fiber.StatusBadRequest {
					t.Errorf("status = %d, want 400", status)
				}
				if string(body) != tc.wantBody {
					t.Errorf("body = %s, want %s", body, tc.wantBody)
				}
			})
		}
	})
}

// TestSeedNormalizationHelperMapsSentinelToNull covers the normalization rule shared by the
// seed CLI (cmd/seed, package main, so the rule itself is the unit under test) and every other
// peserta write path: any non-positive id becomes NULL, any positive id passes through.
func TestSeedNormalizationHelperMapsSentinelToNull(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want sql.NullInt64
	}{
		{0, sql.NullInt64{}},
		{-1, sql.NullInt64{}},
		{1, sql.NullInt64{Int64: 1, Valid: true}},
		{4242, sql.NullInt64{Int64: 4242, Valid: true}},
	} {
		if got := db.NullableFK(tc.in); got != tc.want {
			t.Errorf("NullableFK(%d) = %v, want %v", tc.in, got, tc.want)
		}
	}

	one := int64(1)
	zero := int64(0)
	for _, tc := range []struct {
		name string
		in   *int64
		want sql.NullInt64
	}{
		{"absent", nil, sql.NullInt64{}},
		{"explicit zero", &zero, sql.NullInt64{}},
		{"positive", &one, sql.NullInt64{Int64: 1, Valid: true}},
	} {
		if got := db.NullableFKPtr(tc.in); got != tc.want {
			t.Errorf("NullableFKPtr(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
