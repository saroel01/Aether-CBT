package service

import (
	"database/sql"
	"testing"
	"time"

	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// seedSettings inserts (or replaces) the legacy per-tenant settings row used by the old
// global-token login path. tokenExpiry may be nil.
func seedSettings(t *testing.T, db *sql.DB, tenantID int, token string, active bool, tokenExpiry *string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO settings (tenant_id, exam_title, token, is_exam_active, token_expiry)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(tenant_id) DO UPDATE SET token = excluded.token, is_exam_active = excluded.is_exam_active, token_expiry = excluded.token_expiry`,
		tenantID, "Legacy Exam", token, active, tokenExpiry,
	); err != nil {
		t.Fatalf("seed settings tenant %d: %v", tenantID, err)
	}
}

func countExamSessions(t *testing.T, db *sql.DB, tenantID int) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM exam_session WHERE tenant_id = ? AND deleted_at IS NULL`, tenantID).Scan(&n); err != nil {
		t.Fatalf("count exam_session tenant %d: %v", tenantID, err)
	}
	return n
}

func countExams(t *testing.T, db *sql.DB, tenantID int) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM exam WHERE tenant_id = ? AND deleted_at IS NULL`, tenantID).Scan(&n); err != nil {
		t.Fatalf("count exam tenant %d: %v", tenantID, err)
	}
	return n
}

// TestMigrateLegacySessions_CreatesForTenantWithoutSession verifies the core Requirement
// 14.3 behavior: a tenant that still relies on the global settings.token (and has no
// exam_session) gets one legacy exam + one legacy exam_session, whose token equals the
// old settings.token so existing logins resolve to the new session path.
func TestMigrateLegacySessions_CreatesForTenantWithoutSession(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	seedSettings(t, database, 1, "LEGACY-TOKEN", true, nil)

	m := NewLegacyMigrator(repository.NewExamRepository(database), repository.NewExamSessionRepository(database))
	if err := m.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if got := countExams(t, database, 1); got != 1 {
		t.Errorf("exams for tenant 1 = %d, want 1", got)
	}
	if got := countExamSessions(t, database, 1); got != 1 {
		t.Errorf("exam_sessions for tenant 1 = %d, want 1", got)
	}

	// The legacy session must carry the old token so logins keep resolving.
	sess, err := repository.NewExamSessionRepository(database).FindByToken(1, "LEGACY-TOKEN")
	if err != nil || len(sess) == 0 {
		t.Fatalf("legacy session with token LEGACY-TOKEN not found: %v (count=%d)", err, len(sess))
	}
	if sess[0].Status != "aktif" {
		t.Errorf("legacy session status = %q, want aktif", sess[0].Status)
	}
}

// TestMigrateLegacySessions_RerunIsIdempotent verifies Requirement 14.4: running the
// migration again must not duplicate the legacy exam/session.
func TestMigrateLegacySessions_RerunIsIdempotent(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	seedSettings(t, database, 1, "LEGACY", true, nil)

	m := NewLegacyMigrator(repository.NewExamRepository(database), repository.NewExamSessionRepository(database))
	for i := 0; i < 3; i++ {
		if err := m.Migrate(database); err != nil {
			t.Fatalf("Migrate run %d: %v", i, err)
		}
	}

	if got := countExams(t, database, 1); got != 1 {
		t.Errorf("exams after 3 runs = %d, want 1", got)
	}
	if got := countExamSessions(t, database, 1); got != 1 {
		t.Errorf("exam_sessions after 3 runs = %d, want 1", got)
	}
}

// TestMigrateLegacySessions_SkipsTenantWithExistingSession verifies the guard: a tenant
// that already has any exam_session (e.g. an admin already configured the new model) is
// left untouched, even if it has a settings row.
func TestMigrateLegacySessions_SkipsTenantWithExistingSession(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	// Tenant already has a session configured the new way.
	testutil.SeedExamSession(t, database, 1, 1, 1, "2026-01-01 08:00:00", "2026-01-01 10:00:00", "NEW", "terjadwal")
	seedSettings(t, database, 1, "LEGACY", true, nil)

	m := NewLegacyMigrator(repository.NewExamRepository(database), repository.NewExamSessionRepository(database))
	if err := m.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if got := countExams(t, database, 1); got != 1 {
		t.Errorf("exams = %d, want 1 (must not add a legacy exam)", got)
	}
	if got := countExamSessions(t, database, 1); got != 1 {
		t.Errorf("exam_sessions = %d, want 1 (must not add a legacy session)", got)
	}
}

// TestMigrateLegacySessions_SkipsTenantWithoutSettings verifies that a tenant with no
// settings row at all is not processed. NOTE: migration 005 seeds a default settings row
// for tenant 1, so this test uses a non-default tenant (id 2) which has no settings seed.
func TestMigrateLegacySessions_SkipsTenantWithoutSettings(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()

	testutil.SeedTenant(t, database, 2, "second", "Second School")
	// Tenant 2 has no settings row and no exam_session: nothing to migrate.

	m := NewLegacyMigrator(repository.NewExamRepository(database), repository.NewExamSessionRepository(database))
	if err := m.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if got := countExams(t, database, 2); got != 0 {
		t.Errorf("exams = %d, want 0", got)
	}
	if got := countExamSessions(t, database, 2); got != 0 {
		t.Errorf("exam_sessions = %d, want 0", got)
	}
}

// TestMigrateLegacySessions_MultipleTenants verifies that several tenants are migrated
// independently (tenant isolation, Requirement 15).
func TestMigrateLegacySessions_MultipleTenants(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()

	testutil.SeedTenant(t, database, 1, "sekolah-a", "Sekolah A")
	testutil.SeedTenant(t, database, 2, "sekolah-b", "Sekolah B")
	seedSettings(t, database, 1, "TOKEN-A", true, nil)
	seedSettings(t, database, 2, "TOKEN-B", true, nil)

	m := NewLegacyMigrator(repository.NewExamRepository(database), repository.NewExamSessionRepository(database))
	if err := m.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for _, tc := range []struct {
		tenant, wantCount int
		token             string
	}{
		{1, 1, "TOKEN-A"},
		{2, 1, "TOKEN-B"},
	} {
		if got := countExams(t, database, tc.tenant); got != tc.wantCount {
			t.Errorf("exams tenant %d = %d, want %d", tc.tenant, got, tc.wantCount)
		}
		if got := countExamSessions(t, database, tc.tenant); got != tc.wantCount {
			t.Errorf("exam_sessions tenant %d = %d, want %d", tc.tenant, got, tc.wantCount)
		}
		sess, err := repository.NewExamSessionRepository(database).FindByToken(tc.tenant, tc.token)
		if err != nil || len(sess) != 1 {
			t.Errorf("tenant %d: legacy session token %q not found uniquely (err=%v, n=%d)", tc.tenant, tc.token, err, len(sess))
		}
	}
}

// TestMigrateLegacySessions_WindowIsWide verifies the legacy session window is wide open
// around "now" so that an old install can log in immediately after migration (Requirement
// 14.4) without admin intervention.
func TestMigrateLegacySessions_WindowIsWide(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	seedSettings(t, database, 1, "LEGACY", true, nil)

	now := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	m := NewLegacyMigrator(repository.NewExamRepository(database), repository.NewExamSessionRepository(database), WithLegacyClock(func() time.Time { return now }))
	if err := m.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	sess, err := repository.NewExamSessionRepository(database).FindByToken(1, "LEGACY")
	if err != nil || len(sess) != 1 {
		t.Fatalf("legacy session not found: %v (n=%d)", err, len(sess))
	}
	s := sess[0]
	if !s.WaktuMulai.Before(now) || !s.WaktuSelesai.After(now) {
		t.Errorf("legacy window [%s, %s] does not bracket now=%s", s.WaktuMulai, s.WaktuSelesai, now)
	}
}
