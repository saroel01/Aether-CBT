package handlers

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

func buildZipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %q: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func newMultipartUpload(t *testing.T, app *fiber.App, path, filename string, content []byte) *http.Response {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	req := httptest.NewRequest("POST", path, body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp
}

func TestUploadSoalPackage_HappyPath(t *testing.T) {
	SetSoalStorageDir(t.TempDir())
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/soal-packages/upload", adminOnly, UploadSoalPackage)
	testutil.SeedTenant(t, database, 1, "default", "Default School")

	zipBytes := buildZipBytes(t, map[string]string{"index.html": "<html></html>", "data/player.js": "x"})
	resp := newMultipartUpload(t, app, "/api/admin/soal-packages/upload", "kimia.zip", zipBytes)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d, want 200", resp.StatusCode)
	}

	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM soal_package WHERE tenant_id = 1`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 package row, got %d", n)
	}
}

func TestUploadSoalPackage_NonAdminForbidden(t *testing.T) {
	SetSoalStorageDir(t.TempDir())
	app, adminOnly, _, cleanup := newAdminTestApp(t, "student")
	defer cleanup()
	app.Post("/api/admin/soal-packages/upload", adminOnly, UploadSoalPackage)

	zipBytes := buildZipBytes(t, map[string]string{"index.html": "x"})
	resp := newMultipartUpload(t, app, "/api/admin/soal-packages/upload", "x.zip", zipBytes)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestUploadSoalPackage_RejectsNonZipExtension(t *testing.T) {
	SetSoalStorageDir(t.TempDir())
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/admin/soal-packages/upload", adminOnly, UploadSoalPackage)
	testutil.SeedTenant(t, database, 1, "default", "Default School")

	resp := newMultipartUpload(t, app, "/api/admin/soal-packages/upload", "notazip.txt", []byte("hello"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestListSoalPackages(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Get("/api/admin/soal-packages", adminOnly, ListSoalPackages)
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedSoalPackage(t, database, 1, 1, "P1", "u1")

	resp := doJSON(t, app, "GET", "/api/admin/soal-packages", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDeleteSoalPackage_LinkedConflict(t *testing.T) {
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Delete("/api/admin/soal-packages/:id", adminOnly, DeleteSoalPackage)
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedSoalPackage(t, database, 10, 1, "P", "u10")
	testutil.SeedExam(t, database, 1, 1, 1, intPtr(10))

	resp := doJSON(t, app, "DELETE", "/api/admin/soal-packages/10", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (linked)", resp.StatusCode)
	}
}
