package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

// Audit H1: dead-lettered submissions are listed per tenant; a supervisor (user_id = ruang.id)
// only sees peserta of their own room.
func TestGetSubmissionFailures_TenantAndRoomScope(t *testing.T) {
	for _, tc := range []struct {
		role string
		want []string
	}{
		{"admin", []string{"R1-001", "R2-001"}},
		{"supervisor", []string{"R1-001"}},
	} {
		t.Run(tc.role, func(t *testing.T) {
			app, _, database, cleanup := newAdminTestApp(t, tc.role)
			defer cleanup()
			app.Get("/api/supervisor/submission-failures", GetSubmissionFailures)

			testutil.SeedTenant(t, database, 1, "default", "Default School")
			testutil.SeedTenant(t, database, 2, "other", "Other School")
			testutil.SeedKelas(t, database, 1, 1, "X-1")
			testutil.SeedRuang(t, database, 1, 1, "Ruang 1", "ruang_1")
			testutil.SeedRuang(t, database, 2, 1, "Ruang 2", "ruang_2")
			testutil.SeedPeserta(t, database, 1, 1, 1, 1, "R1-001", "Siswa R1")
			testutil.SeedPeserta(t, database, 2, 1, 1, 2, "R2-001", "Siswa R2")
			if _, err := database.Exec(`
				INSERT INTO submission_failure (tenant_id, no_id, validasi, attempt_token, error_message)
				VALUES (1, 'R1-001', 'v1', 't1', 'grace period exceeded'),
				       (1, 'R2-001', 'v2', 't2', 'invalid detail XML'),
				       (2, 'R1-001', 'v3', 't3', 'other tenant')
			`); err != nil {
				t.Fatalf("seed submission_failure: %v", err)
			}

			resp := doJSON(t, app, "GET", "/api/supervisor/submission-failures", nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			var body struct {
				Data []SubmissionFailure `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			var got []string
			for _, f := range body.Data {
				if f.ErrorMessage == "other tenant" {
					t.Fatalf("row from another tenant leaked: %+v", f)
				}
				got = append(got, f.NoID)
			}
			sort.Strings(got)
			if len(got) != len(tc.want) {
				t.Fatalf("no_ids = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("no_ids = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
