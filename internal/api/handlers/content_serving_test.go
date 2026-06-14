package handlers

import (
	"database/sql"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/soalpkg"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// enterableWindow returns a real-clock window that spans now (Requirement: content is
// enterable only while the server clock is inside [mulai, selesai]).
func enterableWindow() (time.Time, time.Time) {
	now := time.Now()
	return now.Add(-1 * time.Hour), now.Add(2 * time.Hour)
}

// contentSeed describes the tenant-scoped graph a content-serving test needs.
type contentSeed struct {
	tenantID, kelasID, ruangID, pesertaID, mapelID, packageID, examID int
	slug, noID, nama, packageUUID, attemptToken, contentToken, token, status string
	mulai, selesai                                                        time.Time
}

// seedContentGraph inserts the full graph and an active cek_login bound to contentToken,
// returning the created session id. It bypasses service-level validation so tests can
// construct otherwise-rejected states (e.g. an active session on a package-less exam).
func seedContentGraph(t *testing.T, db *sql.DB, s contentSeed) int {
	t.Helper()
	testutil.SeedTenant(t, db, s.tenantID, s.slug, s.slug+" School")
	testutil.SeedKelas(t, db, s.kelasID, s.tenantID, "Kelas "+s.slug)
	testutil.SeedRuang(t, db, s.ruangID, s.tenantID, "Ruang "+s.slug, "ruang_"+s.slug)
	testutil.SeedPeserta(t, db, s.pesertaID, s.tenantID, s.kelasID, s.ruangID, s.noID, s.nama)
	testutil.SeedMapel(t, db, s.mapelID, s.tenantID, "Mapel "+s.slug, strings.ToUpper(s.slug))
	testutil.SeedSoalPackage(t, db, s.packageID, s.tenantID, "Pkg "+s.slug, s.packageUUID)
	testutil.SeedExam(t, db, s.examID, s.tenantID, s.mapelID, &s.packageID)
	sess, err := repository.NewExamSessionRepository(db).Create(s.tenantID, repository.SessionInput{
		ExamID: s.examID, WaktuMulai: s.mulai, WaktuSelesai: s.selesai,
		Token: s.token, Status: s.status,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	cek := repository.NewCekLoginRepository(db)
	if err := cek.Start(s.tenantID, s.pesertaID, sess.ID, s.attemptToken); err != nil {
		t.Fatalf("start cek_login: %v", err)
	}
	if err := cek.SetContentToken(s.tenantID, s.pesertaID, sess.ID, s.contentToken); err != nil {
		t.Fatalf("set content token: %v", err)
	}
	return sess.ID
}

// writePackage writes an index.html (with the given visible marker) and a data/player.js
// asset under {storageDir}/{slug}/{uuid}/ so the handler has real files to serve.
func writePackage(t *testing.T, storageDir, slug, uuid, marker string) {
	t.Helper()
	pkgDir := filepath.Join(storageDir, slug, uuid)
	if err := os.MkdirAll(filepath.Join(pkgDir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	indexHTML := "<html><head><title>x</title></head><body>" + marker + "</body></html>"
	if err := os.WriteFile(filepath.Join(pkgDir, "index.html"), []byte(indexHTML), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "data", "player.js"), []byte("// player code "+marker), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
}

// newContentTestAppReal wires the content route over a migrated DB and a temp storage dir,
// restoring the global storage dir on cleanup. The tenant is derived from the cookie token,
// so the test middleware's tenant_id is irrelevant (and no role guard is applied).
func newContentTestAppReal(t *testing.T) (*fiber.App, *sql.DB, string, func()) {
	t.Helper()
	app, _, database, cleanup := newAdminTestApp(t, "student")
	dir := t.TempDir()
	SetSoalStorageDir(dir)
	app.Get("/api/exam/content/*", ServeExamContent)
	return app, database, dir, func() { SetSoalStorageDir("data/soal"); cleanup() }
}

func defaultTenant1Seed(token string) contentSeed {
	mulai, selesai := enterableWindow()
	return contentSeed{
		tenantID: 1, kelasID: 1, ruangID: 1, pesertaID: 1, mapelID: 1, packageID: 10, examID: 1,
		slug: "default", noID: "2026001", nama: "Siswa", packageUUID: "uuid-10",
		attemptToken: "att-1", contentToken: token, token: "TOK", status: models.SessionStatusAktif,
		mulai: mulai, selesai: selesai,
	}
}

func TestServeExamContent_OwnerGetsIndexWithShim(t *testing.T) {
	app, database, dir, cleanup := newContentTestAppReal(t)
	defer cleanup()
	sid := seedContentGraph(t, database, defaultTenant1Seed("ct-1"))
	_ = sid
	writePackage(t, dir, "default", "uuid-10", "QUIZMARKER")

	req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
	req.Header.Set("Cookie", "aether_exam=ct-1")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	if !strings.Contains(bs, "window.__AETHER__") {
		t.Errorf("shim not injected; body head: %q", snippet(bs))
	}
	if !strings.Contains(bs, `"attemptToken":"att-1"`) {
		t.Errorf("attempt token not embedded in shim context")
	}
	if !strings.Contains(bs, "QUIZMARKER") {
		t.Errorf("served body does not contain the package marker; got: %q", snippet(bs))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestServeExamContent_AssetServedWithoutShim(t *testing.T) {
	app, database, dir, cleanup := newContentTestAppReal(t)
	defer cleanup()
	seedContentGraph(t, database, defaultTenant1Seed("ct-1"))
	writePackage(t, dir, "default", "uuid-10", "QUIZMARKER")

	req := httptest.NewRequest("GET", "/api/exam/content/data/player.js", nil)
	req.Header.Set("Cookie", "aether_exam=ct-1")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	if strings.Contains(bs, "window.__AETHER__") {
		t.Errorf("shim must NOT be injected on non-entry assets")
	}
	if !strings.Contains(bs, "player code") {
		t.Errorf("asset body mismatch; got: %q", snippet(bs))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q, want javascript", ct)
	}
}

func TestServeExamContent_MissingCookie(t *testing.T) {
	app, database, _, cleanup := newContentTestAppReal(t)
	defer cleanup()
	seedContentGraph(t, database, defaultTenant1Seed("ct-1"))

	req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (missing cookie)", resp.StatusCode)
	}
}

func TestServeExamContent_BogusToken(t *testing.T) {
	app, database, _, cleanup := newContentTestAppReal(t)
	defer cleanup()
	seedContentGraph(t, database, defaultTenant1Seed("ct-1"))

	req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
	req.Header.Set("Cookie", "aether_exam=not-a-real-token")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (bogus token)", resp.StatusCode)
	}
}

func TestServeExamContent_Locked(t *testing.T) {
	app, database, dir, cleanup := newContentTestAppReal(t)
	defer cleanup()
	sid := seedContentGraph(t, database, defaultTenant1Seed("ct-lock"))
	writePackage(t, dir, "default", "uuid-10", "QUIZMARKER")
	if err := repository.NewCekLoginRepository(database).Lock(1, 1, sid); err != nil {
		t.Fatalf("lock: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
	req.Header.Set("Cookie", "aether_exam=ct-lock")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (locked)", resp.StatusCode)
	}
}

func TestServeExamContent_WindowEnded(t *testing.T) {
	app, database, dir, cleanup := newContentTestAppReal(t)
	defer cleanup()
	now := time.Now()
	s := defaultTenant1Seed("ct-ended")
	s.mulai = now.Add(-2 * time.Hour)
	s.selesai = now.Add(-1 * time.Hour) // ended
	seedContentGraph(t, database, s)
	writePackage(t, dir, "default", "uuid-10", "QUIZMARKER")

	req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
	req.Header.Set("Cookie", "aether_exam=ct-ended")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (window ended)", resp.StatusCode)
	}
}

func TestServeExamContent_NoPackage(t *testing.T) {
	app, database, _, cleanup := newContentTestAppReal(t)
	defer cleanup()
	// Exam 1 with NO linked package, but an active session (bypasses service validation).
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	mulai, selesai := enterableWindow()
	sess, _ := repository.NewExamSessionRepository(database).Create(1, repository.SessionInput{
		ExamID: 1, WaktuMulai: mulai, WaktuSelesai: selesai, Token: "TOK", Status: models.SessionStatusAktif,
	})
	cek := repository.NewCekLoginRepository(database)
	_ = cek.Start(1, 1, sess.ID, "att-1")
	_ = cek.SetContentToken(1, 1, sess.ID, "ct-nopkg")

	req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
	req.Header.Set("Cookie", "aether_exam=ct-nopkg")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (no package)", resp.StatusCode)
	}
}

func TestServeExamContent_MissingAsset(t *testing.T) {
	app, database, dir, cleanup := newContentTestAppReal(t)
	defer cleanup()
	seedContentGraph(t, database, defaultTenant1Seed("ct-1"))
	writePackage(t, dir, "default", "uuid-10", "QUIZMARKER")

	req := httptest.NewRequest("GET", "/api/exam/content/data/does-not-exist.js", nil)
	req.Header.Set("Cookie", "aether_exam=ct-1")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (missing asset)", resp.StatusCode)
	}
}

func TestServeExamContent_TraversalDoesNotLeak(t *testing.T) {
	app, database, dir, cleanup := newContentTestAppReal(t)
	defer cleanup()
	seedContentGraph(t, database, defaultTenant1Seed("ct-1"))
	writePackage(t, dir, "default", "uuid-10", "QUIZMARKER")
	// Plant a sensitive file outside the package dir to prove it is NOT served.
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("TOPSECRET"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	// A traversal attempt must not leak the out-of-package file. Fasthttp normalizes ".."
	// (and "%2e%2e") in the URL before routing, so such requests are rejected at the
	// framework layer (404); the handler's ResolvePath is defense-in-depth (-> 400). The
	// spec accepts either status (HANDOFF 8.3); the hard property is "no leakage".
	for _, p := range []string{"/api/exam/content/%2e%2e/secret.txt", "/api/exam/content/../secret.txt"} {
		req := httptest.NewRequest("GET", p, nil)
		req.Header.Set("Cookie", "aether_exam=ct-1")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test %s: %v", p, err)
		}
		if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 400 or 404", p, resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "TOPSECRET") {
			t.Errorf("%s: traversal leaked a file outside the package dir", p)
		}
	}
}

// TestMapServeError_TraversalMapsTo400 covers the handler's defense-in-depth traversal
// mapping directly: if ResolvePath ever returns ErrPathTraversal inside the handler, it must
// surface as 400 (the URL layer normally rejects ".." first, so this is belt-and-suspenders).
func TestMapServeError_TraversalMapsTo400(t *testing.T) {
	app := fiber.New()
	app.Get("/t", func(c *fiber.Ctx) error { return mapServeError(c, soalpkg.ErrPathTraversal) })
	req := httptest.NewRequest("GET", "/t", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (ErrPathTraversal)", resp.StatusCode)
	}
}

// TestMapServeError_NotExistMapsTo404 covers the missing-file mapping.
func TestMapServeError_NotExistMapsTo404(t *testing.T) {
	app := fiber.New()
	app.Get("/t", func(c *fiber.Ctx) error {
		return mapServeError(c, &fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist})
	})
	req := httptest.NewRequest("GET", "/t", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (not exist)", resp.StatusCode)
	}
}

func TestServeExamContent_CrossTenantIsolation(t *testing.T) {
	app, database, dir, cleanup := newContentTestAppReal(t)
	defer cleanup()
	// Tenant 1 (default) and tenant 2 (tenant2), each with their own package + content token.
	seedContentGraph(t, database, defaultTenant1Seed("ct-1"))
	t2Mulai, t2Selesai := enterableWindow()
	seedContentGraph(t, database, contentSeed{
		tenantID: 2, kelasID: 2, ruangID: 2, pesertaID: 2, mapelID: 2, packageID: 20, examID: 2,
		slug: "tenant2", noID: "2026002", nama: "Siswa2", packageUUID: "uuid-20",
		attemptToken: "att-2", contentToken: "ct-2", token: "TOK2", status: models.SessionStatusAktif,
		mulai: t2Mulai, selesai: t2Selesai,
	})
	writePackage(t, dir, "default", "uuid-10", "TENANT1-MARKER")
	writePackage(t, dir, "tenant2", "uuid-20", "TENANT2-MARKER")

	// Tenant 1's token must serve tenant 1's index, never tenant 2's.
	req := httptest.NewRequest("GET", "/api/exam/content/index.html", nil)
	req.Header.Set("Cookie", "aether_exam=ct-1")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	if !strings.Contains(bs, "TENANT1-MARKER") {
		t.Errorf("expected tenant-1 content; got: %q", snippet(bs))
	}
	if strings.Contains(bs, "TENANT2-MARKER") {
		t.Errorf("tenant-1 token leaked tenant-2 content (isolation broken)")
	}
}

func snippet(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
