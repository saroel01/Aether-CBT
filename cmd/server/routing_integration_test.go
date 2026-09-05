package main

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/api/handlers"
	"github.com/saroel01/aether-cbt/internal/api/middleware"
	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/submission"

	_ "modernc.org/sqlite"
)

func setupTestAppForRouting(t *testing.T) (*fiber.App, func()) {
	t.Helper()

	// Locate web/build directory
	webBuildDir := filepath.Join("..", "..", "web", "build")
	if _, err := os.Stat(filepath.Join(webBuildDir, "index.html")); err != nil {
		webBuildDir = "./web/build"
	}
	absWebBuildDir, err := filepath.Abs(webBuildDir)
	if err != nil {
		t.Fatalf("resolve web build dir: %v", err)
	}

	// In-memory test DB
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

	// Seed tenant and full exam chain for content serving test
	_, _ = testDB.Exec("INSERT INTO tenants (id, slug, name) VALUES (1, 'default', 'Sekolah Contoh')")
	_, _ = testDB.Exec("INSERT INTO kelas (id, tenant_id, nama_kelas, tingkat) VALUES (1, 1, 'X-A', 'X')")
	_, _ = testDB.Exec("INSERT INTO ruang (id, tenant_id, nama_ruang, kode_ruang) VALUES (1, 1, 'Lab 1', 'R1')")
	_, _ = testDB.Exec("INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (1, 1, '2026001', 'hash', 'Ahmad', 1, 1)")
	_, _ = testDB.Exec("INSERT INTO mapel (id, tenant_id, nama_mapel, kode_mapel) VALUES (1, 1, 'Matematika', 'MTK')")
	_, _ = testDB.Exec("INSERT INTO soal_package (id, tenant_id, nama, package_uuid, entry_path) VALUES (1, 1, 'Paket 1', 'pkg-uuid-1', 'index.html')")
	_, _ = testDB.Exec("INSERT INTO exam (id, tenant_id, mapel_id, soal_package_id, nama) VALUES (1, 1, 1, 1, 'Ujian Matematika')")

	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	endTime := now.Add(2 * time.Hour).Format("2006-01-02 15:04:05")
	_, _ = testDB.Exec("INSERT INTO exam_session (id, tenant_id, exam_id, nama, waktu_mulai, waktu_selesai, token, status) VALUES (1, 1, 1, 'Sesi 1', ?, ?, 'SES123', 'aktif')", startTime, endTime)
	_, _ = testDB.Exec("INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, session_id, attempt_token, content_token, locked) VALUES (1, 1, 1, 1, 'test-attempt-token', 'test-content-token', 0)")

	// Package file for content serving
	soalDir := t.TempDir()
	handlers.SetSoalStorageDir(soalDir)
	pkgPath := filepath.Join(soalDir, "default", "pkg-uuid-1")
	if err := os.MkdirAll(pkgPath, 0755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgPath, "index.html"), []byte("<html><body>Exam Content Ready</body></html>"), 0644); err != nil {
		t.Fatalf("write pkg index: %v", err)
	}

	// Queue
	queueDir := t.TempDir()
	fsQueue, err := submission.NewFilesystemQueue(queueDir)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	handlers.SetSubmissionQueue(fsQueue)

	app := fiber.New()
	app.Use(middleware.SecurityHeaders())

	// API routes
	api := app.Group("/api")
	api.Use(middleware.TenantMiddleware())

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	api.Get("/qrcode", handlers.GetTokenQRCode)
	api.Post("/ispring/webhook", handlers.ISpringWebhook)
	api.Get("/exam/content/*", handlers.ServeExamContent)

	// Static files & SPA routing
	app.Static("/", absWebBuildDir)
	app.Get("/*", func(c *fiber.Ctx) error {
		path := strings.ToLower(c.Path())
		if path == "/api" || strings.HasPrefix(path, "/api/") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "API route not found",
			})
		}
		return c.SendFile(filepath.Join(absWebBuildDir, "index.html"))
	})

	cleanup := func() {
		handlers.SetSoalStorageDir("data/soal")
		handlers.SetSubmissionQueue(nil)
		testDB.Close()
	}

	return app, cleanup
}

func TestEndToEndRoutingAndExemptions(t *testing.T) {
	// Enforce production mode to ensure tenant enforcement is active where required
	t.Setenv("ENV", "production")

	app, cleanup := setupTestAppForRouting(t)
	defer cleanup()

	// 1. GET /admin without custom headers -> 200 OK text/html (SvelteKit frontend, NOT 400 Bad Request)
	t.Run("GET /admin serves SvelteKit HTML", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET /admin failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "<!doctype html>") && !strings.Contains(string(body), "<html") {
			t.Errorf("body does not contain HTML tags: %s", string(body[:min(100, len(body))]))
		}
	})

	// 2. GET / returns 200 OK with SvelteKit landing page
	t.Run("GET / serves SvelteKit landing page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET / failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), `"status":"running"`) {
			t.Errorf("GET / returned legacy JSON instead of HTML frontend!")
		}
	})

	// 3. GET /student/login and GET /supervisor/login return 200 OK with SvelteKit HTML
	t.Run("GET /student/login and /supervisor/login serve SvelteKit HTML", func(t *testing.T) {
		routes := []string{"/student/login", "/supervisor/login"}
		for _, r := range routes {
			req := httptest.NewRequest("GET", r, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("GET %s failed: %v", r, err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s status = %d, want 200", r, resp.StatusCode)
			}
			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(ct, "text/html") {
				t.Errorf("%s Content-Type = %q, want text/html", r, ct)
			}
		}
	})

	// 4. Static frontend asset loading
	t.Run("GET static frontend assets", func(t *testing.T) {
		webBuildDir := filepath.Join("..", "..", "web", "build")
		if _, err := os.Stat(filepath.Join(webBuildDir, "index.html")); err != nil {
			webBuildDir = "./web/build"
		}
		assetPath := "/_app/immutable/entry/start.Gg_XYRc0.js"
		if matches, err := filepath.Glob(filepath.Join(webBuildDir, "_app", "immutable", "entry", "start.*.js")); err == nil && len(matches) > 0 {
			assetPath = "/_app/immutable/entry/" + filepath.Base(matches[0])
		}
		req := httptest.NewRequest("GET", assetPath, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET static asset failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("static asset status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "javascript") {
			t.Errorf("Content-Type = %q, want javascript", ct)
		}
	})

	// 5. GET /api/health returns 200 OK with {"status":"ok"} without tenant header
	t.Run("GET /api/health without tenant header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/health", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET /api/health failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"status":"ok"`) {
			t.Errorf("payload = %s, want status ok", string(body))
		}
	})

	// 6. GET /api/qrcode?text=TEST returns 200 OK with Content-Type: image/png without tenant header
	t.Run("GET /api/qrcode without tenant header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/qrcode?text=TEST", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET /api/qrcode failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "image/png") {
			t.Errorf("Content-Type = %q, want image/png", ct)
		}
	})

	// 7. POST /api/ispring/webhook with valid form body processes without tenant header
	t.Run("POST /api/ispring/webhook without tenant header", func(t *testing.T) {
		form := url.Values{}
		form.Add("sid", "2026001")
		form.Add("sp", "80")
		form.Add("tp", "100")
		form.Add("attempt_token", "test-attempt-token")

		req := httptest.NewRequest("POST", "/api/ispring/webhook", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("POST /api/ispring/webhook failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 200, body: %s", resp.StatusCode, string(body))
		}
	})

	// 8. GET /api/exam/content/* is accessible without tenant header (returns 401 when cookie missing, not 400 tenant required)
	t.Run("GET /api/exam/content/* without tenant header missing cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET /api/exam/content/* failed: %v", err)
		}
		// Expect 401 Unauthorized (due to missing exam cookie), NOT 400 Bad Request (tenant required)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401 (missing cookie, but tenant requirement exempted)", resp.StatusCode)
		}
	})

	// 9. GET /api/exam/content/* with valid content-session cookie serves package with injected shim
	t.Run("GET /api/exam/content/* with valid session cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
		req.Header.Set("Cookie", "aether_exam=test-content-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET /api/exam/content/* with cookie failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 200, body: %s", resp.StatusCode, string(body))
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Exam Content Ready") {
			t.Errorf("expected package index content, got: %s", string(body))
		}
	})

	// 10. Non-API route prefixed with /api (e.g. /api-docs) is NOT blocked as backend API route
	t.Run("GET /api-docs serves SvelteKit HTML", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api-docs", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET /api-docs failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
	})

	// 11. Unmatched API route returns 404 JSON when tenant is provided, and 400 when missing in production
	t.Run("GET /API/nonexistent returns 404 JSON with tenant header and 400 without", func(t *testing.T) {
		// Without tenant header in production: rejected by TenantMiddleware
		reqNoTenant := httptest.NewRequest("GET", "/API/nonexistent", nil)
		respNoTenant, err := app.Test(reqNoTenant)
		if err != nil {
			t.Fatalf("GET /API/nonexistent without tenant failed: %v", err)
		}
		if respNoTenant.StatusCode != http.StatusBadRequest {
			t.Errorf("status without tenant = %d, want 400", respNoTenant.StatusCode)
		}

		// With tenant header in production: reaches 404 handler, returning JSON instead of HTML
		reqWithTenant := httptest.NewRequest("GET", "/API/nonexistent", nil)
		reqWithTenant.Header.Set("X-Tenant-ID", "1")
		respWithTenant, err := app.Test(reqWithTenant)
		if err != nil {
			t.Fatalf("GET /API/nonexistent with tenant failed: %v", err)
		}
		if respWithTenant.StatusCode != http.StatusNotFound {
			t.Errorf("status with tenant = %d, want 404", respWithTenant.StatusCode)
		}
		body, _ := io.ReadAll(respWithTenant.Body)
		if !strings.Contains(string(body), `"error":"API route not found"`) {
			t.Errorf("expected API route not found JSON, got: %s", string(body))
		}
	})

	// 12. Uppercase /API/health without tenant header
	t.Run("GET /API/health without tenant header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/API/health", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET /API/health failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"status":"ok"`) {
			t.Errorf("payload = %s, want status ok", string(body))
		}
	})
}
