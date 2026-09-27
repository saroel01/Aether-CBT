package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/saroel01/aether-cbt/internal/api/handlers"
	"github.com/saroel01/aether-cbt/internal/api/middleware"
	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/submission"

	_ "modernc.org/sqlite"
)

func setupResilienceTestApp(t *testing.T) (*fiber.App, func()) {
	t.Helper()

	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.DB = testDB

	migrationsDir := filepath.Join("..", "..", "internal", "db", "migrations")
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = "internal/db/migrations"
	}
	if err := db.RunMigrations(testDB, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	queueDir := t.TempDir()
	fsQueue, err := submission.NewFilesystemQueue(queueDir)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	handlers.SetSubmissionQueue(fsQueue)

	app := fiber.New(fiber.Config{
		BodyLimit: 100 * 1024 * 1024,
	})

	// 1. Recover middleware (P1-13)
	app.Use(recover.New())

	// 2. BodyLimit middleware (P1-9)
	defaultBodyLimit := 2 * 1024 * 1024 // 2 MB
	maxUploadLimit := 100 * 1024 * 1024 // 100 MB
	app.Use(func(c *fiber.Ctx) error {
		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}

		p := strings.ToLower(c.Path())

		// Upload routes are permitted up to maxUploadLimit (100 MB)
		if strings.HasPrefix(p, "/api/admin/soal-packages/upload") || strings.HasPrefix(p, "/api/soal-packages/upload") {
			if c.Request().Header.ContentLength() > maxUploadLimit || len(c.Body()) > maxUploadLimit {
				return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
					"status":  "error",
					"message": fmt.Sprintf("Ukuran file paket soal melebihi batas maksimum (%d MB)", maxUploadLimit/(1024*1024)),
				})
			}
			return c.Next()
		}

		if strings.HasPrefix(p, "/api/ispring/webhook") {
			webhookLimit := 1 * 1024 * 1024 // 1 MB
			if c.Request().Header.ContentLength() > webhookLimit || len(c.Body()) > webhookLimit {
				return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
					"status":  "error",
					"message": "Webhook payload exceeds maximum allowed size (1 MB)",
				})
			}
			return c.Next()
		}

		cl := c.Request().Header.ContentLength()
		if cl > defaultBodyLimit {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"status":  "error",
				"message": "Payload too large. Maximum body size is 2 MB.",
			})
		}
		if len(c.Body()) > defaultBodyLimit {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"status":  "error",
				"message": "Payload too large. Maximum body size is 2 MB.",
			})
		}
		return c.Next()
	})

	api := app.Group("/api")
	api.Use(middleware.TenantMiddleware())

	// Login rate limiter (P1-8)
	loginLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			tenant := ""
			if t := c.Locals("tenant_id"); t != nil {
				tenant = fmt.Sprint(t)
			}
			return c.IP() + "|" + tenant
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"status":  "error",
				"message": "Too many login attempts. Please try again later.",
			})
		},
	})

	auth := api.Group("/auth")
	auth.Post("/login", loginLimiter, handlers.Login)
	auth.Post("/student-login", loginLimiter, handlers.StudentLogin)
	auth.Post("/supervisor-login", loginLimiter, handlers.SupervisorLogin)

	api.Post("/ispring/webhook", handlers.ISpringWebhook)

	// Mock upload route to test 100 MB allowance
	api.Post("/admin/soal-packages/upload", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Test route that panics to verify recover middleware
	api.Get("/test-panic", func(c *fiber.Ctx) error {
		panic("simulated panic in handler")
	})

	cleanup := func() {
		handlers.SetSubmissionQueue(nil)
		testDB.Close()
	}

	return app, cleanup
}

func TestServerResilience_PanicRecovery(t *testing.T) {
	app, cleanup := setupResilienceTestApp(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/test-panic", nil)
	req.Header.Set("X-Tenant-ID", "1")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 InternalServerError from recovered panic, got %d", resp.StatusCode)
	}
}

func TestServerResilience_LoginRateLimiter(t *testing.T) {
	endpoints := []string{"/api/auth/login", "/api/auth/student-login", "/api/auth/supervisor-login"}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			app, cleanup := setupResilienceTestApp(t)
			defer cleanup()

			// First 10 attempts should proceed to handler (even if 400/401, NOT 429)
			for i := 1; i <= 10; i++ {
				req := httptest.NewRequest("POST", ep, bytes.NewBufferString(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Tenant-ID", "1")
				resp, err := app.Test(req)
				if err != nil {
					t.Fatalf("attempt %d failed: %v", i, err)
				}
				if resp.StatusCode == http.StatusTooManyRequests {
					t.Fatalf("attempt %d prematurely rate-limited (got 429)", i)
				}
			}

			// 11th attempt must be rejected with 429 Too Many Requests (P1-8 regression)
			req11 := httptest.NewRequest("POST", ep, bytes.NewBufferString(`{}`))
			req11.Header.Set("Content-Type", "application/json")
			req11.Header.Set("X-Tenant-ID", "1")
			resp11, err := app.Test(req11)
			if err != nil {
				t.Fatalf("attempt 11 failed: %v", err)
			}
			if resp11.StatusCode != http.StatusTooManyRequests {
				t.Errorf("attempt 11 got status %d, want 429 TooManyRequests", resp11.StatusCode)
			}
		})
	}
}

func TestServerResilience_BodyLimit(t *testing.T) {
	app, cleanup := setupResilienceTestApp(t)
	defer cleanup()

	t.Run("General route rejects payload exceeding 2 MB", func(t *testing.T) {
		largeBody := make([]byte, 3*1024*1024) // 3 MB
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(largeBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", "1")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413 RequestEntityTooLarge", resp.StatusCode)
		}
	})

	t.Run("Webhook route rejects payload exceeding 1 MB", func(t *testing.T) {
		largeBody := make([]byte, 2*1024*1024) // 2 MB
		req := httptest.NewRequest("POST", "/api/ispring/webhook", bytes.NewReader(largeBody))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Tenant-ID", "1")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413 RequestEntityTooLarge", resp.StatusCode)
		}
	})

	t.Run("Upload route permits payload exceeding 2 MB", func(t *testing.T) {
		largeBody := make([]byte, 3*1024*1024) // 3 MB
		req := httptest.NewRequest("POST", "/api/admin/soal-packages/upload", bytes.NewReader(largeBody))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-Tenant-ID", "1")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode == http.StatusRequestEntityTooLarge {
			t.Errorf("upload route was unexpectedly rejected with 413")
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("upload status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("Upload route permits payload with mixed case path", func(t *testing.T) {
		largeBody := make([]byte, 3*1024*1024) // 3 MB
		req := httptest.NewRequest("POST", "/API/Admin/Soal-Packages/Upload", bytes.NewReader(largeBody))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-Tenant-ID", "1")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode == http.StatusRequestEntityTooLarge {
			t.Errorf("mixed-case upload route was unexpectedly rejected with 413")
		}
	})

	t.Run("Upload route rejects payload exceeding 100 MB", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/soal-packages/upload", bytes.NewReader([]byte{}))
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Length", fmt.Sprintf("%d", 101*1024*1024)) // 101 MB header
		req.Header.Set("X-Tenant-ID", "1")
		resp, err := app.Test(req)
		if err != nil {
			if strings.Contains(err.Error(), "body size exceeds the given limit") {
				// Rejected at transport level by Fiber's BodyLimit (100 MB)
				return
			}
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("expected 413 for payload > 100 MB, got status %d", resp.StatusCode)
		}
	})
}
