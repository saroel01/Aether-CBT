package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/saroel01/aether-cbt/internal/models"
)

// sqliteDatetimeLayout is the canonical format in which exam_session timestamps are
// stored. Formatting consistently (rather than letting the driver choose) keeps the
// stored values lexically comparable, so token-overlap window checks work correctly
// even when some rows were seeded directly with the same format.
const sqliteDatetimeLayout = "2006-01-02 15:04:05"

func sqlDatetime(t time.Time) string {
	return t.UTC().Format(sqliteDatetimeLayout)
}

// SessionInput carries the caller-supplied fields for creating/updating an exam session
// (Requirement 4.1). Status defaults to "draft" when empty.
type SessionInput struct {
	ExamID       int
	Nama         *string
	WaktuMulai   time.Time
	WaktuSelesai time.Time
	Token        string
	Status       string
}

// ExamSessionRepository manages scheduled exam waves and their class/room relations.
// All access is tenant-scoped (Requirement 15.2). Token-overlap detection is exposed as
// a query (TokenOverlaps) for the scheduling service to enforce (Requirement 4.4).
type ExamSessionRepository struct {
	db *sql.DB
}

func NewExamSessionRepository(db *sql.DB) *ExamSessionRepository {
	return &ExamSessionRepository{db: db}
}

const examSessionColumns = `id, tenant_id, exam_id, nama, waktu_mulai, waktu_selesai, token, status, created_at, updated_at, deleted_at`

func scanExamSession(s scanner) (*models.ExamSession, error) {
	ss := &models.ExamSession{}
	var nama sql.NullString
	var deletedAt sql.NullTime
	if err := s.Scan(
		&ss.ID, &ss.TenantID, &ss.ExamID, &nama, &ss.WaktuMulai, &ss.WaktuSelesai, &ss.Token, &ss.Status,
		&ss.CreatedAt, &ss.UpdatedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	ss.Nama = nullString(nama)
	ss.DeletedAt = nullTime(deletedAt)
	return ss, nil
}

// validateExam returns ErrInvalidReference if the exam does not belong to the tenant.
func (r *ExamSessionRepository) validateExam(tenantID, examID int) error {
	var count int
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM exam WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		examID, tenantID,
	).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return ErrInvalidReference
	}
	return nil
}

// tenantHasRow reports whether a master-table row belongs to the tenant. table is a
// compile-time-constant table name from this package's own callers ("kelas"/"ruang"),
// never user input, so interpolating it is safe.
func (r *ExamSessionRepository) tenantHasRow(table string, tenantID, id int) (bool, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM `+table+` WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		id, tenantID,
	).Scan(&count)
	return count > 0, err
}

// Create inserts a new exam session (Requirement 4.1), defaulting status to "draft".
func (r *ExamSessionRepository) Create(tenantID int, in SessionInput) (*models.ExamSession, error) {
	if err := r.validateExam(tenantID, in.ExamID); err != nil {
		return nil, err
	}
	status := in.Status
	if status == "" {
		status = models.SessionStatusDraft
	}
	res, err := r.db.Exec(`
		INSERT INTO exam_session (tenant_id, exam_id, nama, waktu_mulai, waktu_selesai, token, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, tenantID, in.ExamID, in.Nama, sqlDatetime(in.WaktuMulai), sqlDatetime(in.WaktuSelesai), in.Token, status)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(tenantID, int(id))
}

// GetByID returns the session for the tenant, or ErrNotFound.
func (r *ExamSessionRepository) GetByID(tenantID, id int) (*models.ExamSession, error) {
	s, err := scanExamSession(r.db.QueryRow(
		`SELECT `+examSessionColumns+` FROM exam_session WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		id, tenantID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s, nil
}

// List returns every session in the tenant, newest first (Requirement 4.6).
func (r *ExamSessionRepository) List(tenantID int) ([]models.ExamSession, error) {
	rows, err := r.db.Query(
		`SELECT `+examSessionColumns+` FROM exam_session WHERE tenant_id = ? AND deleted_at IS NULL ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []models.ExamSession
	for rows.Next() {
		s, err := scanExamSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	return sessions, rows.Err()
}

// Update overwrites the session's fields after re-validating the exam reference. It
// returns ErrNotFound if no such session belongs to the tenant.
func (r *ExamSessionRepository) Update(tenantID, id int, in SessionInput) (*models.ExamSession, error) {
	if _, err := r.GetByID(tenantID, id); err != nil {
		return nil, err
	}
	if err := r.validateExam(tenantID, in.ExamID); err != nil {
		return nil, err
	}
	status := in.Status
	if status == "" {
		status = models.SessionStatusDraft
	}
	res, err := r.db.Exec(`
		UPDATE exam_session SET
			exam_id = ?, nama = ?, waktu_mulai = ?, waktu_selesai = ?, token = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, in.ExamID, in.Nama, sqlDatetime(in.WaktuMulai), sqlDatetime(in.WaktuSelesai), in.Token, status, id, tenantID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(tenantID, id)
}

// Delete soft-deletes the session (it can no longer be found via the tenant-scoped
// lookups, satisfying Requirement 4.7 / 14.4 without dropping rows).
func (r *ExamSessionRepository) Delete(tenantID, id int) error {
	if _, err := r.GetByID(tenantID, id); err != nil {
		return err
	}
	_, err := r.db.Exec(`UPDATE exam_session SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND tenant_id = ?`, id, tenantID)
	return err
}

// AttachClasses links the given classes to the session. Every class must belong to the
// tenant or the whole operation is rejected with ErrInvalidReference (Requirement 4.7).
// Already-linked classes are a no-op (INSERT OR IGNORE).
func (r *ExamSessionRepository) AttachClasses(tenantID, sessionID int, kelasIDs []int) error {
	for _, kelasID := range kelasIDs {
		ok, err := r.tenantHasRow("kelas", tenantID, kelasID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvalidReference
		}
	}
	for _, kelasID := range kelasIDs {
		if _, err := r.db.Exec(`INSERT OR IGNORE INTO exam_session_kelas (session_id, kelas_id) VALUES (?, ?)`, sessionID, kelasID); err != nil {
			return err
		}
	}
	return nil
}

// AttachRooms links the given rooms to the session; every room must belong to the tenant
// (Requirement 4.7). Already-linked rooms are a no-op.
func (r *ExamSessionRepository) AttachRooms(tenantID, sessionID int, ruangIDs []int) error {
	for _, ruangID := range ruangIDs {
		ok, err := r.tenantHasRow("ruang", tenantID, ruangID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvalidReference
		}
	}
	for _, ruangID := range ruangIDs {
		if _, err := r.db.Exec(`INSERT OR IGNORE INTO exam_session_ruang (session_id, ruang_id) VALUES (?, ?)`, sessionID, ruangID); err != nil {
			return err
		}
	}
	return nil
}

// TokenOverlaps reports whether another non-deleted session in the tenant uses the same
// token with an overlapping time window (Requirement 4.4). Pass excludeSessionID to skip
// a session itself (use during updates); 0 excludes nothing.
func (r *ExamSessionRepository) TokenOverlaps(tenantID int, token string, mulai, selesai time.Time, excludeSessionID int) (bool, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM exam_session
		WHERE tenant_id = ? AND token = ? AND deleted_at IS NULL AND id <> ?
		  AND waktu_mulai < ? AND waktu_selesai > ?
	`, tenantID, token, excludeSessionID, sqlDatetime(selesai), sqlDatetime(mulai)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByToken returns all non-deleted sessions in the tenant that share the token. More
// than one may exist when their windows do not overlap (Requirement 4.4 / 4.8); the
// scheduling service selects the effective one. Returns ErrNotFound when none match.
func (r *ExamSessionRepository) FindByToken(tenantID int, token string) ([]models.ExamSession, error) {
	rows, err := r.db.Query(
		`SELECT `+examSessionColumns+` FROM exam_session WHERE tenant_id = ? AND token = ? AND deleted_at IS NULL`,
		tenantID, token,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []models.ExamSession
	for rows.Next() {
		s, err := scanExamSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, ErrNotFound
	}
	return sessions, nil
}
