package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// localsApp builds an app whose locals mimic an authenticated caller.
func localsApp(role string, userID int) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", role)
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Get("/me", Me)
	app.Put("/me", UpdateMyProfile)
	return app
}

func seedMeFixture(t *testing.T) *sql.DB {
	t.Helper()
	database, cleanup := testutil.NewMigratedDB(t)
	t.Cleanup(cleanup)
	prev := db.DB
	db.DB = database
	t.Cleanup(func() { db.DB = prev })
	testutil.SeedTenant(t, database, 1, "default", "Default")
	testutil.SeedKelas(t, database, 1, 1, "X")
	testutil.SeedRuang(t, database, 1, 1, "Ruang Satu", "ruang1")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "NIS001", "Budi Siswa")
	if _, err := database.Exec(`INSERT INTO users (id, tenant_id, username, password_hash, role, full_name, is_active)
		VALUES (1, 1, 'admin', 'x', 'admin', 'Admin Sekolah', TRUE)`); err != nil {
		t.Fatal(err)
	}
	hash, _ := utils.HashPassword("lama12345")
	if _, err := database.Exec(`UPDATE ruang SET password_hash = ? WHERE id = 1`, hash); err != nil {
		t.Fatal(err)
	}
	return database
}

func meData(t *testing.T, app *fiber.App) map[string]any {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", "/me", nil), -1)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GET /me: %v status=%v", err, resp.StatusCode)
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body.Data
}

func putMe(t *testing.T, app *fiber.App, payload string) int {
	t.Helper()
	req := httptest.NewRequest("PUT", "/me", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

// M4: /me reads the account table that matches the role.
func TestMeIsRoleAware(t *testing.T) {
	seedMeFixture(t)
	if d := meData(t, localsApp("student", 1)); d["username"] != "NIS001" || d["full_name"] != "Budi Siswa" {
		t.Errorf("student /me = %v, want peserta NIS001/Budi Siswa", d)
	}
	if d := meData(t, localsApp("supervisor", 1)); d["username"] != "ruang1" || d["full_name"] != "Ruang Satu" {
		t.Errorf("supervisor /me = %v, want ruang1/Ruang Satu", d)
	}
	if d := meData(t, localsApp("admin", 1)); d["username"] != "admin" || d["full_name"] != "Admin Sekolah" {
		t.Errorf("admin /me = %v", d)
	}
}

// M4 + M5: supervisors change the room password; min length 8; token_version bumps.
func TestSupervisorChangesOwnPassword(t *testing.T) {
	database := seedMeFixture(t)
	app := localsApp("supervisor", 1)

	if got := putMe(t, app, `{"current_password":"lama12345","new_password":"1234567"}`); got != 400 {
		t.Errorf("7-char password: status %d, want 400", got)
	}
	if got := putMe(t, app, `{"current_password":"salah","new_password":"baru123456"}`); got != 401 {
		t.Errorf("wrong current password: status %d, want 401", got)
	}
	if got := putMe(t, app, `{"current_password":"lama12345","new_username":"x"}`); got != 400 {
		t.Errorf("supervisor new_username: status %d, want 400", got)
	}
	if got := putMe(t, app, `{"current_password":"lama12345","new_password":"baru123456"}`); got != 200 {
		t.Fatalf("valid change: status %d, want 200", got)
	}
	var hash string
	var tv int
	if err := database.QueryRow(`SELECT password_hash, token_version FROM ruang WHERE id = 1`).Scan(&hash, &tv); err != nil {
		t.Fatal(err)
	}
	if !utils.CheckPasswordHash("baru123456", hash) || tv != 1 {
		t.Errorf("after change: password updated=%v token_version=%d, want true/1", utils.CheckPasswordHash("baru123456", hash), tv)
	}
	if got := putMe(t, localsApp("student", 1), `{"current_password":"siswa123","new_password":"baru123456"}`); got != 403 {
		t.Errorf("student PUT /me: status %d, want 403", got)
	}
}
