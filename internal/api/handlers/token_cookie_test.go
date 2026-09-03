package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// TestContentCookieSecureTriState tests Clause 2.14: "auto", "true", "false".
func TestContentCookieSecureTriState(t *testing.T) {
	app := fiber.New()
	app.Get("/set-cookie", func(c *fiber.Ctx) error {
		setContentCookie(c, "test-content-token")
		return c.SendString("ok")
	})

	// Test "true"
	SetContentCookieSecureMode("true")
	req := httptest.NewRequest("GET", "http://example.com/set-cookie", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	cookie := strings.ToLower(resp.Header.Get("Set-Cookie"))
	if !strings.Contains(cookie, "secure") {
		t.Errorf("mode 'true': expected secure in Set-Cookie, got %q", resp.Header.Get("Set-Cookie"))
	}

	// Test "false"
	SetContentCookieSecureMode("false")
	req = httptest.NewRequest("GET", "https://example.com/set-cookie", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	cookie = strings.ToLower(resp.Header.Get("Set-Cookie"))
	if strings.Contains(cookie, "secure") {
		t.Errorf("mode 'false': expected no secure in Set-Cookie, got %q", resp.Header.Get("Set-Cookie"))
	}

	// Test "auto" - plain HTTP without header -> no Secure
	SetContentCookieSecureMode("auto")
	req = httptest.NewRequest("GET", "http://example.com/set-cookie", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	cookie = strings.ToLower(resp.Header.Get("Set-Cookie"))
	if strings.Contains(cookie, "secure") {
		t.Errorf("mode 'auto' (plain http): expected no secure in Set-Cookie, got %q", resp.Header.Get("Set-Cookie"))
	}

	// Test "auto" - with X-Forwarded-Proto: https -> Secure
	req = httptest.NewRequest("GET", "http://example.com/set-cookie", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	cookie = strings.ToLower(resp.Header.Get("Set-Cookie"))
	if !strings.Contains(cookie, "secure") {
		t.Errorf("mode 'auto' (X-Forwarded-Proto: https): expected secure in Set-Cookie, got %q", resp.Header.Get("Set-Cookie"))
	}
}

// TestStudentLoginRejectsEmptyToken tests Clause 2.12.
func TestStudentLoginRejectsEmptyToken(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	testutil.SeedTenant(t, database, 1, "default", "Default School")

	db.DB = database

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/api/student/login", StudentLogin)

	for _, tokenVal := range []string{"", "   ", " \t "} {
		body, err := json.Marshal(map[string]string{
			"no_id":    "2026001",
			"password": "test",
			"token":    tokenVal,
		})
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		req := httptest.NewRequest("POST", "/api/student/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("token %q: status = %d, want 401", tokenVal, resp.StatusCode)
		}
	}
}

// TestWebhookRejectsEmptyAttemptToken tests Clause 2.13.
func TestWebhookRejectsEmptyAttemptToken(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()

	db.DB = database

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/api/ispring/webhook", ISpringWebhook)

	for _, attemptToken := range []string{"", "   "} {
		body := "USER_NAME=2026001&sp=10&tp=10&attempt_token=" + attemptToken
		req := httptest.NewRequest("POST", "/api/ispring/webhook", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("attempt_token %q: status = %d, want 403", attemptToken, resp.StatusCode)
		}
	}
}
