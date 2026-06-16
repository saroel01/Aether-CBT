package handlers

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/api/middleware"
	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// newAdminTestApp builds a Fiber app guarded by the real RequireRoles middleware, with
// tenant_id/role/user_id injected by a test middleware (no JWT setup). The migrated DB is
// exposed as the package-global db.DB so handlers see it. Returns the adminOnly middleware
// so each test wires the routes under test with the correct role guard.
func newAdminTestApp(t *testing.T, role string) (*fiber.App, fiber.Handler, *sql.DB, func()) {
	t.Helper()
	database, cleanup := testutil.NewMigratedDB(t)
	db.DB = database

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", role)
		c.Locals("user_id", 1)
		return c.Next()
	})
	return app, middleware.RequireRoles("admin", "superadmin"), database, func() { _ = database.Close(); cleanup() }
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body io.Reader) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test %s %s: %v", method, path, err)
	}
	return resp
}

func TestSetClassTingkat_AdminUpdates(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Put("/api/classes/:id/tingkat", adminOnly, SetClassTingkat)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")

	resp := doJSON(t, app, "PUT", "/api/classes/1/tingkat", strings.NewReader(`{"tingkat":"XII"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var tingkat sql.NullString
	if err := database.QueryRow(`SELECT tingkat FROM kelas WHERE id = 1`).Scan(&tingkat); err != nil {
		t.Fatalf("query tingkat: %v", err)
	}
	if !tingkat.Valid || tingkat.String != "XII" {
		t.Errorf("tingkat = %v, want XII", tingkat)
	}
}

func TestSetClassTingkat_NonAdminForbidden(t *testing.T) {
	app, adminOnly, _, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Put("/api/classes/:id/tingkat", adminOnly, SetClassTingkat)

	resp := doJSON(t, app, "PUT", "/api/classes/1/tingkat", strings.NewReader(`{"tingkat":"XII"}`))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for non-admin", resp.StatusCode)
	}
}

func TestSetClassTingkat_NotFound(t *testing.T) {
	app, adminOnly, _, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Put("/api/classes/:id/tingkat", adminOnly, SetClassTingkat)

	resp := doJSON(t, app, "PUT", "/api/classes/999/tingkat", strings.NewReader(`{"tingkat":"X"}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// TestGetClasses_IncludesTingkat verifies Requirement 1.4: the class list includes the
// tingkat attribute on every row (so the admin UI can display and edit it).
func TestGetClasses_IncludesTingkat(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Get("/api/classes", GetClasses)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	if _, err := database.Exec(`UPDATE kelas SET tingkat = 'XII' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	resp := doJSON(t, app, "GET", "/api/classes", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeJSON(t, resp)
	rows, ok := body["data"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("data = %v, want 1 row", body["data"])
	}
	first := rows[0].(map[string]interface{})
	if first["tingkat"] != "XII" {
		t.Errorf("tingkat = %v, want XII", first["tingkat"])
	}
}

// TestGetSettings_SeedsRandomToken verifies the auto-seed path generates a random
// per-tenant token instead of the known constant "ujian2026" (review H22, Task 18). Uses
// tenant 2 because migration 005 already seeds tenant 1's settings with 'ujian2026'.
func TestGetSettings_SeedsRandomToken(t *testing.T) {
	_, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	testutil.SeedTenant(t, database, 2, "other", "Other School")
	// Tenant 2 has NO settings row -> getSettingsForTenant must auto-seed a random token.

	s, err := getSettingsForTenant(2)
	if err != nil {
		t.Fatalf("getSettingsForTenant(2): %v", err)
	}
	if s.Token == "ujian2026" {
		t.Fatal("seeded token must not be the known constant ujian2026")
	}
	if len(s.Token) < 16 {
		t.Fatalf("seeded token too short: %q (want >= 16 chars)", s.Token)
	}

	// A second fetch must return the SAME persisted token (not a new random one).
	s2, _ := getSettingsForTenant(2)
	if s2.Token != s.Token {
		t.Fatalf("second fetch token = %q, want stable %q", s2.Token, s.Token)
	}
}

// TestCreateUser_RejectsSuperadminRole verifies CreateUser rejects a "superadmin" role
// (and any role outside the admin/supervisor allowlist) so an admin cannot mint a superadmin
// (review H15, Task 24).
func TestCreateUser_RejectsSuperadminRole(t *testing.T) {
	app, _, _, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/users", CreateUser)

	resp := doJSON(t, app, "POST", "/api/users", strings.NewReader(`{"username":"x","password":"y","role":"superadmin"}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("superadmin creation: status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateUser_AcceptsSupervisorRole verifies the allowlist still permits a supervisor.
func TestCreateUser_AcceptsSupervisorRole(t *testing.T) {
	app, _, _, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/users", CreateUser)

	resp := doJSON(t, app, "POST", "/api/users", strings.NewReader(`{"username":"sup2","password":"y","role":"supervisor"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("supervisor creation: status = %d, want 200", resp.StatusCode)
	}
}

// TestCreateStudent_RejectsDuplicateNoID verifies CreateStudent rejects a duplicate no_id
// within the same tenant with 409 (review H14, Task 23).
func TestCreateStudent_RejectsDuplicateNoID(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/students", CreateStudent)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa") // existing

	resp := doJSON(t, app, "POST", "/api/students", strings.NewReader(`{"no_id":"2026001","password":"pw","nama_peserta":"Dup","kelas_id":1,"ruang_id":1}`))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate no_id: status = %d, want 409", resp.StatusCode)
	}
}

// TestCreateStudent_RejectsCrossTenantRef verifies CreateStudent rejects a kelas_id from
// another tenant with 400 (review H14, Task 23).
func TestCreateStudent_RejectsCrossTenantRef(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/students", CreateStudent)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedTenant(t, database, 2, "other", "Other School")
	testutil.SeedKelas(t, database, 2, 2, "Other Tenant Class") // tenant 2
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")

	// Caller is tenant 1; kelas_id 2 belongs to tenant 2.
	resp := doJSON(t, app, "POST", "/api/students", strings.NewReader(`{"no_id":"2026099","password":"pw","nama_peserta":"X","kelas_id":2,"ruang_id":1}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cross-tenant kelas ref: status = %d, want 400", resp.StatusCode)
	}
}

// TestLinkClassSubject_RejectsCrossTenant verifies LinkClassSubject rejects a link between
// a kelas in the caller's tenant and a mapel from a DIFFERENT tenant (review H16, Task 17).
func TestLinkClassSubject_RejectsCrossTenant(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/curriculum/link", LinkClassSubject)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedTenant(t, database, 2, "other", "Other School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1") // tenant 1
	testutil.SeedMapel(t, database, 5, 2, "Biologi", "BIO") // tenant 2 — different tenant

	// Caller is tenant 1 (test middleware); linking kelas 1 (tenant 1) to mapel 5 (tenant 2).
	resp := doJSON(t, app, "POST", "/api/admin/curriculum/link", strings.NewReader(`{"kelas_id":1,"mapel_id":5}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cross-tenant link: status = %d, want 400", resp.StatusCode)
	}
}

// TestLinkClassSubject_AcceptsSameTenant verifies a same-tenant link still succeeds (regression
// guard for the tenant validation added in Task 17).
func TestLinkClassSubject_AcceptsSameTenant(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/curriculum/link", LinkClassSubject)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedMapel(t, database, 5, 1, "Kimia", "KIM") // same tenant 1

	resp := doJSON(t, app, "POST", "/api/admin/curriculum/link", strings.NewReader(`{"kelas_id":1,"mapel_id":5}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("same-tenant link: status = %d, want 200", resp.StatusCode)
	}
}
