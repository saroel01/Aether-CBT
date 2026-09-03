package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

// newMappingTestApp wires the curriculum-mapping routes on a migrated DB with the caller
// authenticated as an admin of tenant 1.
func newMappingTestApp(t *testing.T) (*fiber.App, *sql.DB, func()) {
	t.Helper()
	app, adminOnly, database, cleanup := newAdminTestApp(t, "admin")
	app.Post("/api/mapping/link", adminOnly, LinkClassSubject)
	app.Post("/api/mapping/unlink", adminOnly, UnlinkClassSubject)
	return app, database, cleanup
}

// seedTwoTenantCurriculum gives tenant 1 and tenant 2 a kelas + mapel each, and links
// tenant 2's pair in kelas_mapel. Tenant 1's admin has no business touching that row.
func seedTwoTenantCurriculum(t *testing.T, database *sql.DB) {
	t.Helper()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedTenant(t, database, 2, "other", "Other School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedKelas(t, database, 2, 2, "XII IPS 1")
	testutil.SeedMapel(t, database, 2, 2, "Biologi", "BIO")
	if _, err := database.Exec(
		`INSERT INTO kelas_mapel (kelas_id, mapel_id, tenant_id, is_active) VALUES (?, ?, ?, TRUE)`,
		2, 2, 2,
	); err != nil {
		t.Fatalf("seed tenant 2 mapping: %v", err)
	}
}

func countMapping(t *testing.T, database *sql.DB, kelasID, mapelID int) int {
	t.Helper()
	var n int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM kelas_mapel WHERE kelas_id = ? AND mapel_id = ?`, kelasID, mapelID,
	).Scan(&n); err != nil {
		t.Fatalf("count kelas_mapel(%d,%d): %v", kelasID, mapelID, err)
	}
	return n
}

// TestUnlinkClassSubject_CrossTenantRejected covers codebase-bug-sweep clause 1.6/2.6 (C6):
// UnlinkClassSubject's DELETE carried no tenant_id predicate, so an admin in tenant 1 who
// knows (or guesses) tenant 2's kelas/mapel ids could delete tenant 2's curriculum mapping —
// and got a 200 success telling them it worked.
//
// The expected status is 404, matching the tenancy decision taken for session attach: the
// caller must not learn that a mapping exists in another tenant.
//
// On F the row is deleted and the response is 200.
//
// Validates: Requirements 2.6, 3.7
func TestUnlinkClassSubject_CrossTenantRejected(t *testing.T) {
	app, database, cleanup := newMappingTestApp(t)
	defer cleanup()
	seedTwoTenantCurriculum(t, database)

	resp := doJSON(t, app, "POST", "/api/mapping/unlink", strings.NewReader(`{"kelas_id":2,"mapel_id":2}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-tenant unlink: status = %d, want 404", resp.StatusCode)
	}
	if n := countMapping(t, database, 2, 2); n != 1 {
		t.Errorf("tenant 2's mapping rows = %d, want 1 (must survive a cross-tenant unlink)", n)
	}
}

// TestUnlinkClassSubject_InvalidIDs pins the input-validation half of clause 2.6. A missing
// id currently parses as 0, the DELETE matches nothing, and the handler still answers 200
// "successfully unlinked" — a success response for an operation that did nothing.
//
// Validates: Requirements 2.6
func TestUnlinkClassSubject_InvalidIDs(t *testing.T) {
	app, database, cleanup := newMappingTestApp(t)
	defer cleanup()
	seedTwoTenantCurriculum(t, database)

	cases := []struct {
		name string
		body string
	}{
		{"non-integer kelas_id", `{"kelas_id":"abc","mapel_id":1}`},
		{"non-integer mapel_id", `{"kelas_id":1,"mapel_id":"abc"}`},
		{"missing kelas_id", `{"mapel_id":1}`},
		{"missing mapel_id", `{"kelas_id":1}`},
		{"both missing", `{}`},
		{"zero ids", `{"kelas_id":0,"mapel_id":0}`},
		{"negative ids", `{"kelas_id":-1,"mapel_id":-2}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, app, "POST", "/api/mapping/unlink", strings.NewReader(tc.body))
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("unlink %s: status = %d, want 400", tc.body, resp.StatusCode)
			}
		})
	}
}

// TestUnlinkClassSubject_OwnTenantStillSucceeds is the preservation half of clause 3.7:
// link then unlink inside the caller's own tenant keeps working with the same response.
//
// Validates: Requirements 3.7
func TestUnlinkClassSubject_OwnTenantStillSucceeds(t *testing.T) {
	app, database, cleanup := newMappingTestApp(t)
	defer cleanup()
	seedTwoTenantCurriculum(t, database)

	resp := doJSON(t, app, "POST", "/api/mapping/link", strings.NewReader(`{"kelas_id":1,"mapel_id":1}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("own-tenant link: status = %d, want 200", resp.StatusCode)
	}
	if body := decodeJSON(t, resp); body["message"] != "Subject successfully mapped to class" {
		t.Errorf("link message = %v", body["message"])
	}
	if n := countMapping(t, database, 1, 1); n != 1 {
		t.Fatalf("own-tenant mapping rows after link = %d, want 1", n)
	}

	resp = doJSON(t, app, "POST", "/api/mapping/unlink", strings.NewReader(`{"kelas_id":1,"mapel_id":1}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("own-tenant unlink: status = %d, want 200", resp.StatusCode)
	}
	if body := decodeJSON(t, resp); body["message"] != "Subject successfully unlinked from class" {
		t.Errorf("unlink message = %v", body["message"])
	}
	if n := countMapping(t, database, 1, 1); n != 0 {
		t.Errorf("own-tenant mapping rows after unlink = %d, want 0", n)
	}
	// Tenant 2's mapping is untouched throughout.
	if n := countMapping(t, database, 2, 2); n != 1 {
		t.Errorf("tenant 2's mapping rows = %d, want 1", n)
	}
}
