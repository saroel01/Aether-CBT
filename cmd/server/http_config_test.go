package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newBodyLimitTestApp(uploadMax int64) *fiber.App {
	app := fiber.New(newFiberConfig())
	app.Use(bodyLimitGuard(uploadMax))
	ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	app.Post("/api/auth/login", ok)
	app.Post("/api/ispring/webhook", ok)
	app.Post("/api/admin/soal-packages/upload", func(c *fiber.Ctx) error {
		fh, err := c.FormFile("file")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return c.SendString(strconv.FormatInt(fh.Size, 10))
	})
	return app
}

func multipartBody(t *testing.T, size int) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "pkg.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("a"), size)); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

// TestBodyLimitGuard covers audit M2: buffered requests are capped small while package
// uploads stream past the global limit up to SoalUploadMaxBytes.
func TestBodyLimitGuard(t *testing.T) {
	app := newBodyLimitTestApp(10 << 20)

	post := func(path, ctype string, body []byte) *http.Response {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", ctype)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		return resp
	}

	if r := post("/api/auth/login", "application/json", bytes.Repeat([]byte("x"), 5<<20)); r.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("5 MB login: status %d, want 413", r.StatusCode)
	}
	if r := post("/api/auth/login", "application/json", bytes.Repeat([]byte("x"), 3<<20)); r.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("3 MB JSON: status %d, want 413", r.StatusCode)
	}
	if r := post("/api/auth/login", "application/json", []byte(`{"username":"a"}`)); r.StatusCode != http.StatusOK {
		t.Errorf("small JSON: status %d, want 200", r.StatusCode)
	}
	if r := post("/api/ispring/webhook", "application/x-www-form-urlencoded", bytes.Repeat([]byte("x"), (1<<20)+10)); r.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("webhook > 1 MB: status %d, want 413", r.StatusCode)
	}

	const fileSize = 6 << 20
	body, ctype := multipartBody(t, fileSize)
	r := post("/api/admin/soal-packages/upload", ctype, body.Bytes())
	got, _ := io.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK || string(got) != strconv.Itoa(fileSize) {
		t.Errorf("6 MB upload: status %d body %q, want 200 %d", r.StatusCode, got, fileSize)
	}

	big, bigType := multipartBody(t, 11<<20)
	if r := post("/api/admin/soal-packages/upload", bigType, big.Bytes()); r.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("upload > max: status %d, want 413", r.StatusCode)
	}
}

func TestNewFiberConfigTimeouts(t *testing.T) {
	cfg := newFiberConfig()
	if cfg.BodyLimit != globalBodyLimit || !cfg.StreamRequestBody || cfg.WriteTimeout == 0 || cfg.ReadTimeout == 0 {
		t.Errorf("unexpected config: body=%d stream=%v read=%v write=%v", cfg.BodyLimit, cfg.StreamRequestBody, cfg.ReadTimeout, cfg.WriteTimeout)
	}
}

func TestStaticCacheControl(t *testing.T) {
	app := fiber.New()
	app.Use(staticCacheControl())
	app.Get("/*", func(c *fiber.Ctx) error { return c.SendString("x") })
	for path, want := range map[string]string{
		"/_app/immutable/entry/app.js": "public, max-age=31536000, immutable",
		"/admin":                       "no-cache",
		"/api/health":                  "",
	} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Header.Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control %q, want %q", path, got, want)
		}
	}
}

// A missing hashed chunk falls through to the SPA fallback; index.html must not be cached
// as immutable.
func TestStaticCacheControlSPAFallback(t *testing.T) {
	app := fiber.New()
	app.Use(staticCacheControl())
	app.Get("/*", func(c *fiber.Ctx) error {
		c.Type("html")
		return c.SendString("<!doctype html>")
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/_app/immutable/chunks/missing.js", nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("SPA fallback Cache-Control %q, want no-cache", got)
	}
}
