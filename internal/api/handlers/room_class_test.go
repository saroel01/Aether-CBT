package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/api/middleware"
	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
	"github.com/saroel01/aether-cbt/internal/utils"
)

func newAdminOnlyApp(t *testing.T, role string) (*fiber.App, func()) {
	t.Helper()
	database, cleanup := testutil.NewMigratedDB(t)
	db.DB = database

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", role)
		return c.Next()
	})

	adminOnly := middleware.RequireRoles("admin", "superadmin")
	app.Post("/api/admin/rooms", adminOnly, CreateRoom)
	app.Delete("/api/admin/rooms/:id", adminOnly, DeleteRoom)
	app.Delete("/api/admin/classes/:id", adminOnly, DeleteClass)

	return app, cleanup
}

func TestCreateRoom_ValidationAndConflict(t *testing.T) {
	app, cleanup := newAdminOnlyApp(t, "admin")
	defer cleanup()
	testutil.SeedTenant(t, db.DB, 1, "default", "Default School")

	// Missing fields -> 400
	for _, payload := range []string{
		`{"nama_ruang":"","username":"r1","password":"p1"}`,
		`{"nama_ruang":"R1","username":"","password":"p1"}`,
		`{"nama_ruang":"R1","username":"r1","password":""}`,
	} {
		req := httptest.NewRequest("POST", "/api/admin/rooms", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("payload %s: got %d, want 400", payload, resp.StatusCode)
		}
	}

	// Success -> 200 and password is hashed
	payload := `{"nama_ruang":"Lab 1","username":"lab1","password":"secretpassword"}`
	req := httptest.NewRequest("POST", "/api/admin/rooms", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}

	var hash string
	if err := db.DB.QueryRow(`SELECT password_hash FROM ruang WHERE tenant_id = 1 AND username = 'lab1'`).Scan(&hash); err != nil {
		t.Fatalf("query password_hash: %v", err)
	}
	if hash == "secretpassword" {
		t.Error("password was stored in plain text!")
	}
	if !utils.CheckPasswordHash("secretpassword", hash) {
		t.Error("stored hash cannot be verified with original password")
	}

	// Duplicate username -> 409 Conflict
	req = httptest.NewRequest("POST", "/api/admin/rooms", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate username: got status %d, want 409", resp.StatusCode)
	}
}

func TestDeleteRoom_ValidationAndNotFound(t *testing.T) {
	app, cleanup := newAdminOnlyApp(t, "superadmin") // superadmin allowed
	defer cleanup()
	testutil.SeedTenant(t, db.DB, 1, "default", "Default School")
	testutil.SeedRuang(t, db.DB, 10, 1, "Ruang A", "ruang_a")

	// Invalid ID string -> 400
	req := httptest.NewRequest("DELETE", "/api/admin/rooms/abc", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("abc ID: got %d, want 400", resp.StatusCode)
	}

	// Non-existent ID -> 404
	req = httptest.NewRequest("DELETE", "/api/admin/rooms/999", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("999 ID: got %d, want 404", resp.StatusCode)
	}

	// Successful delete -> 200
	req = httptest.NewRequest("DELETE", "/api/admin/rooms/10", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete 10: got %d, want 200", resp.StatusCode)
	}

	// Repeat delete -> 404 (already deleted)
	req = httptest.NewRequest("DELETE", "/api/admin/rooms/10", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("repeat delete 10: got %d, want 404", resp.StatusCode)
	}
}

func TestDeleteClass_ValidationAndNotFound(t *testing.T) {
	app, cleanup := newAdminOnlyApp(t, "superadmin") // superadmin allowed
	defer cleanup()
	testutil.SeedTenant(t, db.DB, 1, "default", "Default School")
	testutil.SeedKelas(t, db.DB, 20, 1, "Kelas XII")

	// Invalid ID string -> 400
	req := httptest.NewRequest("DELETE", "/api/admin/classes/xyz", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("xyz ID: got %d, want 400", resp.StatusCode)
	}

	// Non-existent ID -> 404
	req = httptest.NewRequest("DELETE", "/api/admin/classes/999", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("999 ID: got %d, want 404", resp.StatusCode)
	}

	// Successful delete -> 200
	req = httptest.NewRequest("DELETE", "/api/admin/classes/20", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete 20: got %d, want 200", resp.StatusCode)
	}

	// Repeat delete -> 404
	req = httptest.NewRequest("DELETE", "/api/admin/classes/20", nil)
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("repeat delete 20: got %d, want 404", resp.StatusCode)
	}
}
