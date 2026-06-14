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
