package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

// TestCreateTenantValidatesSlug (audit L10): slug must be a DNS label, name non-empty,
// duplicates are 409.
func TestCreateTenantValidatesSlug(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "superadmin")
	defer cleanup()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	app.Post("/api/tenants", CreateTenant)

	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"slug":"../x","name":"X"}`, http.StatusBadRequest},
		{`{"slug":"A B","name":"X"}`, http.StatusBadRequest},
		{`{"slug":"","name":"X"}`, http.StatusBadRequest},
		{`{"slug":"sma-2","name":"  "}`, http.StatusBadRequest},
		{`{"slug":"sma-1","name":"SMA 1"}`, http.StatusOK},
		{`{"slug":"SMA-1","name":"SMA 1 lagi"}`, http.StatusConflict},
	} {
		resp := doJSON(t, app, "POST", "/api/tenants", strings.NewReader(tc.body))
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.body, resp.StatusCode, tc.want)
		}
	}
}

// TestCreateTenantRequiresSuperadmin: a school admin cannot provision tenants (L9).
func TestCreateTenantRequiresSuperadmin(t *testing.T) {
	app, _, _, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/api/tenants", CreateTenant)
	resp := doJSON(t, app, "POST", "/api/tenants", strings.NewReader(`{"slug":"sma-9","name":"X"}`))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("admin create tenant: status = %d, want 403", resp.StatusCode)
	}
}

// TestCreateStudentRequiresPassword (audit L5): no implicit default password.
func TestCreateStudentRequiresPassword(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	app.Post("/api/students", CreateStudent)
	for _, body := range []string{
		`{"no_id":"L5-a","nama_peserta":"X"}`,
		`{"no_id":"L5-b","nama_peserta":"X","password":"12345"}`,
	} {
		resp := doJSON(t, app, "POST", "/api/students", strings.NewReader(body))
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", body, resp.StatusCode)
		}
	}
	var n int
	_ = database.QueryRow(`SELECT COUNT(*) FROM peserta WHERE no_id LIKE 'L5-%'`).Scan(&n)
	if n != 0 {
		t.Errorf("rejected students were stored: %d rows", n)
	}
}
