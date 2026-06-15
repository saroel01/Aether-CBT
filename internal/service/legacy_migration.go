package service

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/repository"
)

// legacyMigration constants. The legacy session is created from the old global
// settings.token / is_exam_active configuration so that an old install keeps working
// after the upgrade without admin intervention (Requirement 14.3, design AD-1). The mapel
// and exam are deterministic placeholders so the migration is fully idempotent.
const (
	legacyMapelName = "Warisan (Migrasi)"
	legacyMapelKode = "WARISAN"
	legacyExamName  = "Ujian Warisan"
	legacyExamDurasi = 90

	// The legacy session window is wide-open around "now" so an old install can log in
	// immediately after upgrade (Requirement 14.4): a decade before and after the
	// migration instant. Wide, but finite, so datetime storage stays well-formed.
	legacyWindowPad = 10 * 365 * 24 * time.Hour
)

// LegacyMigrator creates a legacy exam + exam_session for each tenant that still relies
// on the old global settings.token (Requirement 14.3). It is idempotent: tenants that
// already have any exam_session are skipped, and the placeholder mapel/exam are reused on
// rerun (Requirement 14.4). Run it once at startup, after RunMigrations.
type LegacyMigrator struct {
	exams    *repository.ExamRepository
	sessions *repository.ExamSessionRepository
	now      func() time.Time
}

// LegacyMigratorOption configures a LegacyMigrator.
type LegacyMigratorOption func(*LegacyMigrator)

// WithLegacyClock overrides the migrator's clock (defaults to time.Now). Use in tests to
// make the legacy session window deterministic.
func WithLegacyClock(now func() time.Time) LegacyMigratorOption {
	return func(m *LegacyMigrator) { m.now = now }
}

// NewLegacyMigrator builds a migrator over the given repositories.
func NewLegacyMigrator(exams *repository.ExamRepository, sessions *repository.ExamSessionRepository, opts ...LegacyMigratorOption) *LegacyMigrator {
	m := &LegacyMigrator{
		exams:    exams,
		sessions: sessions,
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// legacyTenant is a tenant that has a settings row but no exam_session yet.
type legacyTenant struct {
	id          int
	token       string
	examActive  bool
}

// Migrate scans for tenants still on the legacy global-token model and creates one legacy
// exam + exam_session for each (Requirement 14.3). It is safe to call on every startup:
// tenants already on the session model are skipped, and placeholder entities are reused
// (Requirement 14.4).
func (m *LegacyMigrator) Migrate(database *sql.DB) error {
	tenants, err := m.legacyTenants(database)
	if err != nil {
		return fmt.Errorf("legacy migration: scan tenants: %w", err)
	}
	for _, t := range tenants {
		if err := m.migrateTenant(database, t); err != nil {
			return fmt.Errorf("legacy migration: tenant %d: %w", t.id, err)
		}
	}
	return nil
}

// legacyTenants returns the tenants that have a settings row AND zero exam_sessions. The
// guard is "any session" (not just a legacy one) so an admin who already configured the
// new model is never clobbered.
func (m *LegacyMigrator) legacyTenants(database *sql.DB) ([]legacyTenant, error) {
	rows, err := database.Query(`
		SELECT s.tenant_id, s.token, COALESCE(s.is_exam_active, 0)
		FROM settings s
		WHERE NOT EXISTS (
			SELECT 1 FROM exam_session es
			WHERE es.tenant_id = s.tenant_id AND es.deleted_at IS NULL
		)
		AND EXISTS (
			SELECT 1 FROM tenants t
			WHERE t.id = s.tenant_id AND t.deleted_at IS NULL
		)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []legacyTenant
	for rows.Next() {
		var lt legacyTenant
		if err := rows.Scan(&lt.id, &lt.token, &lt.examActive); err != nil {
			return nil, err
		}
		out = append(out, lt)
	}
	return out, rows.Err()
}

// migrateTenant creates the placeholder mapel (if absent), the legacy exam (if absent),
// and the legacy session for one tenant.
func (m *LegacyMigrator) migrateTenant(database *sql.DB, t legacyTenant) error {
	mapelID, err := m.ensurePlaceholderMapel(database, t.id)
	if err != nil {
		return err
	}
	examID, err := m.ensureLegacyExam(t.id, mapelID)
	if err != nil {
		return err
	}
	now := m.now()
	status := models.SessionStatusTerjadwal
	if t.examActive {
		status = models.SessionStatusAktif
	}
	_, err = m.sessions.Create(t.id, repository.SessionInput{
		ExamID:       examID,
		WaktuMulai:   now.Add(-legacyWindowPad),
		WaktuSelesai: now.Add(legacyWindowPad),
		Token:        t.token,
		Status:       status,
	})
	return err
}

// ensurePlaceholderMapel returns the id of the deterministic placeholder mapel for the
// tenant, creating it if needed. Idempotent via INSERT OR IGNORE + SELECT (mapel has a
// UNIQUE(tenant_id, nama_mapel) constraint).
func (m *LegacyMigrator) ensurePlaceholderMapel(database *sql.DB, tenantID int) (int, error) {
	if _, err := database.Exec(
		`INSERT OR IGNORE INTO mapel (tenant_id, nama_mapel, kode_mapel) VALUES (?, ?, ?)`,
		tenantID, legacyMapelName, legacyMapelKode,
	); err != nil {
		return 0, err
	}
	var id int
	if err := database.QueryRow(
		`SELECT id FROM mapel WHERE tenant_id = ? AND nama_mapel = ? AND deleted_at IS NULL`,
		tenantID, legacyMapelName,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// ensureLegacyExam returns the id of the legacy exam definition for the tenant, creating
// it if no exam with the legacy name exists yet. Reusing the ExamRepository.Create would
// also work, but scanning first avoids inserting a duplicate on every call.
func (m *LegacyMigrator) ensureLegacyExam(tenantID, mapelID int) (int, error) {
	exams, err := m.exams.List(tenantID)
	if err != nil {
		return 0, err
	}
	for i := range exams {
		if exams[i].Nama != nil && *exams[i].Nama == legacyExamName {
			return exams[i].ID, nil
		}
	}
	nama := legacyExamName
	created, err := m.exams.Create(tenantID, repository.ExamInput{
		MapelID:     mapelID,
		Nama:        &nama,
		DurasiMenit: legacyExamDurasi,
	})
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}
