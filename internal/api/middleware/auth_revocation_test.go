package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// M5: AuthMiddleware rejects tokens of deactivated/deleted accounts and tokens whose
// "tv" claim no longer matches the stored token_version.
func TestAuthMiddlewareRevocation(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	t.Cleanup(cleanup)
	prev := db.DB
	db.DB = database
	t.Cleanup(func() { db.DB = prev })
	utils.SetJWTSecret("auth-revocation-test-secret")

	testutil.SeedTenant(t, database, 1, "default", "Default")
	testutil.SeedKelas(t, database, 1, 1, "X")
	testutil.SeedRuang(t, database, 1, 1, "R1", "r1")
	testutil.SeedRuang(t, database, 2, 1, "R2", "r2")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "S1", "Siswa")
	if _, err := database.Exec(`INSERT INTO users (id, tenant_id, username, password_hash, role, full_name, is_active)
		VALUES (5, 1, 'adm', 'x', 'admin', 'Admin', TRUE)`); err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Get("/p", AuthMiddleware(), func(c *fiber.Ctx) error { return c.SendStatus(200) })
	status := func(tok string) int {
		req := httptest.NewRequest("GET", "/p", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	mustTok := func(id int, role string, tv int) string {
		tok, err := utils.GenerateTokenWithVersion(id, 1, role, tv)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	student := mustTok(1, "student", 0)
	admin := mustTok(5, "admin", 0)
	sup := mustTok(2, "supervisor", 0)
	legacy, _ := utils.GenerateToken(1, 1, "student") // tv 0
	for name, tok := range map[string]string{"student": student, "admin": admin, "supervisor": sup, "legacy": legacy} {
		if got := status(tok); got != 200 {
			t.Fatalf("%s valid token: status %d, want 200", name, got)
		}
	}

	if _, err := database.Exec(`UPDATE peserta SET deleted_at = CURRENT_TIMESTAMP WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if got := status(student); got != 401 {
		t.Errorf("soft-deleted peserta: status %d, want 401", got)
	}

	if _, err := database.Exec(`UPDATE users SET is_active = 0 WHERE id = 5`); err != nil {
		t.Fatal(err)
	}
	if got := status(admin); got != 401 {
		t.Errorf("inactive admin: status %d, want 401", got)
	}
	if _, err := database.Exec(`UPDATE users SET is_active = 1, token_version = token_version + 1 WHERE id = 5`); err != nil {
		t.Fatal(err)
	}
	if got := status(admin); got != 401 {
		t.Errorf("old token after token_version bump: status %d, want 401", got)
	}
	if got := status(mustTok(5, "admin", 1)); got != 200 {
		t.Errorf("new token after bump: status %d, want 200", got)
	}

	if _, err := database.Exec(`UPDATE ruang SET deleted_at = CURRENT_TIMESTAMP WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	if got := status(sup); got != 401 {
		t.Errorf("deleted ruang: status %d, want 401", got)
	}
}
