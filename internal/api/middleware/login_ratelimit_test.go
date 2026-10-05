package middleware

import (
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// keyFor runs LoginRateLimitKey inside a request with the given tenant and body.
func keyFor(t *testing.T, tenant int, body string) string {
	t.Helper()
	app := fiber.New()
	var key string
	app.Post("/login", func(c *fiber.Ctx) error {
		c.Locals("tenant_id", tenant)
		key = LoginRateLimitKey(c)
		return nil
	})
	req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if _, err := app.Test(req); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return key
}

func TestLoginRateLimitKey_PerAccountAndTenant(t *testing.T) {
	a := keyFor(t, 1, `{"username":"Admin ","password":"x"}`)
	b := keyFor(t, 1, `{"username":"guru","password":"x"}`)
	s := keyFor(t, 1, `{"no_id":"S001","password":"x"}`)
	other := keyFor(t, 2, `{"username":"admin","password":"x"}`)

	if a == b || a == s || b == s {
		t.Errorf("different accounts on same IP share a key: %q %q %q", a, b, s)
	}
	if a == other {
		t.Errorf("different tenants share a key: %q", a)
	}
	if !strings.HasPrefix(a, "1|admin|") {
		t.Errorf("key = %q, want prefix 1|admin| (trimmed, lowercased)", a)
	}
	if got := keyFor(t, 1, `not-json`); !strings.HasPrefix(got, "1||") {
		t.Errorf("unparsable body key = %q, want empty account", got)
	}
}

// The IP limiter must bound attempts from one IP across all accounts (password spraying).
func TestLoginIPRateLimit_BoundsSprayingAcrossAccounts(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("tenant_id", 1); return c.Next() })
	app.Post("/login",
		limiter.New(limiter.Config{Max: 3, Expiration: time.Minute, KeyGenerator: LoginIPRateLimitKey}),
		limiter.New(limiter.Config{Max: 3, Expiration: time.Minute, KeyGenerator: LoginRateLimitKey}),
		func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusUnauthorized) })
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf(`{"username":"user%d","password":"x"}`, i)
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		want := fiber.StatusUnauthorized
		if i >= 3 {
			want = fiber.StatusTooManyRequests
		}
		if resp.StatusCode != want {
			t.Fatalf("attempt %d: status = %d, want %d", i, resp.StatusCode, want)
		}
	}
}

func ipSeen(t *testing.T, cfg fiber.Config) string {
	t.Helper()
	app := fiber.New(cfg)
	app.Get("/ip", func(c *fiber.Ctx) error { return c.SendString(c.IP()) })
	req := httptest.NewRequest("GET", "/ip", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestTrustedProxyConfig_ClientIPFromXFF(t *testing.T) {
	trusted := ipSeen(t, fiber.Config{
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"0.0.0.0/0"},
		EnableIPValidation:      true,
	})
	if trusted != "203.0.113.9" {
		t.Errorf("trusted proxy: c.IP() = %q, want 203.0.113.9", trusted)
	}
	if plain := ipSeen(t, fiber.Config{}); plain == "203.0.113.9" {
		t.Errorf("without proxy config X-Forwarded-For must be ignored, got %q", plain)
	}
}
