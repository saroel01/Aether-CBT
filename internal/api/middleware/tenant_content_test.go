package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// TestTenantMiddleware_ExemptsContentPath: content serving is authorized by the content-
// session cookie (AD-2), and the iSpring player loads sub-assets via plain HTML tags that
// carry no tenant header. So TenantMiddleware must let /api/exam/content/* through in
// production without a tenant identifier, while still rejecting other unidentified paths.
func TestTenantMiddleware_ExemptsContentPath(t *testing.T) {
	t.Setenv("ENV", "production") // no development fallback
	app := fiber.New()
	app.Use(TenantMiddleware())
	app.Get("/api/exam/content/*", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/api/other", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp, err := app.Test(httptest.NewRequest("GET", "/api/exam/content/index.html", nil))
	if err != nil {
		t.Fatalf("content request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("content path: status = %d, want 200 (exempt from tenant requirement)", resp.StatusCode)
	}

	resp2, err := app.Test(httptest.NewRequest("GET", "/api/other", nil))
	if err != nil {
		t.Fatalf("other request: %v", err)
	}
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("non-content path: status = %d, want 400 (tenant required in production)", resp2.StatusCode)
	}
}
