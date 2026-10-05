package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// TestTenantMiddleware_DefaultTenantID: in production a request with no tenant header to a
// bare IP host is rejected unless DEFAULT_TENANT_ID is set, in which case that tenant is used.
func TestTenantMiddleware_DefaultTenantID(t *testing.T) {
	t.Setenv("ENV", "production")
	app := fiber.New()
	app.Use(TenantMiddleware())
	app.Get("/api/tenant", func(c *fiber.Ctx) error { return c.JSON(GetTenantID(c)) })

	do := func() (*http.Response, error) {
		req := httptest.NewRequest("GET", "/api/tenant", nil)
		req.Host = "192.168.1.10:3000"
		return app.Test(req)
	}

	t.Setenv("DEFAULT_TENANT_ID", "")
	resp, err := do()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unset default: status = %d, want 400", resp.StatusCode)
	}

	t.Setenv("DEFAULT_TENANT_ID", "3")
	resp, err = do()
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 8)
	n, _ := resp.Body.Read(buf)
	if resp.StatusCode != http.StatusOK || string(buf[:n]) != "3" {
		t.Fatalf("default 3: status = %d body = %q, want 200 \"3\"", resp.StatusCode, buf[:n])
	}
}
