package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// TestSecurityHeadersCSPAndNoHSTSOverHTTP (audit L2).
func TestSecurityHeadersCSPAndNoHSTSOverHTTP(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	csp := resp.Header.Get("Content-Security-Policy")
	for _, want := range []string{"frame-ancestors 'self'", "object-src 'none'", "base-uri 'self'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q missing %q", csp, want)
		}
	}
	if h := resp.Header.Get("Strict-Transport-Security"); h != "" {
		t.Errorf("HSTS must not be sent over plain http, got %q", h)
	}
}
