package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// TestTenantMiddleware_NonAPIRoutesPassThrough verifies that non-API routes
// (frontend browser pages and static assets) are never blocked by TenantMiddleware,
// even when ENV=production and no tenant header is provided.
func TestTenantMiddleware_NonAPIRoutesPassThrough(t *testing.T) {
	t.Setenv("ENV", "production")

	app := fiber.New()
	app.Use(TenantMiddleware())

	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("root landing") })
	app.Get("/admin", func(c *fiber.Ctx) error { return c.SendString("admin page") })
	app.Get("/student/login", func(c *fiber.Ctx) error { return c.SendString("student login") })
	app.Get("/supervisor/login", func(c *fiber.Ctx) error { return c.SendString("supervisor login") })
	app.Get("/_app/immutable/chunk.js", func(c *fiber.Ctx) error { return c.SendString("static js") })
	app.Get("/api-docs", func(c *fiber.Ctx) error { return c.SendString("docs") })
	app.Get("/apidocs/*", func(c *fiber.Ctx) error { return c.SendString("docs asset") })

	paths := []string{
		"/",
		"/admin",
		"/student/login",
		"/supervisor/login",
		"/_app/immutable/chunk.js",
		"/api-docs",
		"/apidocs/index.html",
	}

	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request to %s failed: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("path %s: status = %d, want 200 OK (non-API route should pass through)", path, resp.StatusCode)
		}
	}
}

// TestTenantMiddleware_PublicAPIExemptions verifies that public endpoints
// (/api/health, /api/qrcode, /api/ispring/webhook, /api/exam/content/*)
// are explicitly exempted in TenantMiddleware in production.
func TestTenantMiddleware_PublicAPIExemptions(t *testing.T) {
	t.Setenv("ENV", "production")

	app := fiber.New()
	app.Use(TenantMiddleware())

	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/api/qrcode", func(c *fiber.Ctx) error {
		return c.SendString("qrcode-png")
	})
	app.Post("/api/ispring/webhook", func(c *fiber.Ctx) error {
		return c.SendString("webhook-ok")
	})
	app.Get("/api/exam/content/*", func(c *fiber.Ctx) error {
		return c.SendString("content-ok")
	})
	app.Get("/api/protected-resource", func(c *fiber.Ctx) error {
		return c.SendString("protected-ok")
	})

	tests := []struct {
		method     string
		path       string
		wantStatus int
		desc       string
	}{
		{"GET", "/api/health", http.StatusOK, "health check without tenant"},
		{"GET", "/api/health/", http.StatusOK, "health check trailing slash without tenant"},
		{"GET", "/API/health", http.StatusOK, "uppercase health check without tenant"},
		{"GET", "/api/qrcode?text=TEST", http.StatusOK, "qrcode without tenant"},
		{"GET", "/api/qrcode/?text=TEST", http.StatusOK, "qrcode trailing slash without tenant"},
		{"GET", "/API/qrcode?text=TEST", http.StatusOK, "uppercase qrcode without tenant"},
		{"POST", "/api/ispring/webhook", http.StatusOK, "ispring webhook without tenant"},
		{"POST", "/api/ispring/webhook/", http.StatusOK, "ispring webhook trailing slash without tenant"},
		{"POST", "/API/ispring/webhook", http.StatusOK, "uppercase ispring webhook without tenant"},
		{"GET", "/api/exam/content/index.html", http.StatusOK, "content index without tenant"},
		{"GET", "/API/exam/content/index.html", http.StatusOK, "uppercase content index without tenant"},
		{"GET", "/api/exam/content/assets/image.png", http.StatusOK, "content asset without tenant"},
		{"GET", "/api/protected-resource", http.StatusBadRequest, "unexempted API route requires tenant in production"},
		{"GET", "/API/protected-resource", http.StatusBadRequest, "uppercase unexempted API route requires tenant in production"},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s (%s): %v", tc.method, tc.path, tc.desc, err)
		}
		if resp.StatusCode != tc.wantStatus {
			t.Errorf("%s %s (%s): status = %d, want %d", tc.method, tc.path, tc.desc, resp.StatusCode, tc.wantStatus)
		}
	}
}

// TestTenantMiddleware_EnvConsistency verifies consistent behavior between config.go and TenantMiddleware:
// When ENV is unset, "development", or "dev", TenantMiddleware falls back to tenant 1.
// When ENV is "production", explicit tenant identifier is required.
func TestTenantMiddleware_EnvConsistency(t *testing.T) {
	cases := []struct {
		envValue   string
		wantStatus int
		wantTenant int
		desc       string
	}{
		{"", http.StatusOK, 1, "empty/unset ENV falls back to tenant 1"},
		{"development", http.StatusOK, 1, "development ENV falls back to tenant 1"},
		{"dev", http.StatusOK, 1, "dev ENV falls back to tenant 1"},
		{"production", http.StatusBadRequest, 0, "production ENV rejects missing tenant"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Setenv("ENV", tc.envValue)

			app := fiber.New()
			app.Use(TenantMiddleware())

			var observedTenant int
			app.Get("/api/test-tenant", func(c *fiber.Ctx) error {
				observedTenant = GetTenantID(c)
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/api/test-tenant", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK && observedTenant != tc.wantTenant {
				t.Errorf("tenant = %d, want %d", observedTenant, tc.wantTenant)
			}
		})
	}
}

// TestTenantMiddleware_ExplicitTenantInProduction verifies that passing X-Tenant-ID
// works properly in production mode.
func TestTenantMiddleware_ExplicitTenantInProduction(t *testing.T) {
	t.Setenv("ENV", "production")

	app := fiber.New()
	app.Use(TenantMiddleware())

	var capturedTenant int
	app.Get("/api/data", func(c *fiber.Ctx) error {
		capturedTenant = GetTenantID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Tenant-ID", "2")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if capturedTenant != 2 {
		t.Errorf("capturedTenant = %d, want 2", capturedTenant)
	}
}
