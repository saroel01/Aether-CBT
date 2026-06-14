package repository

import (
	"database/sql"
	"errors"

	"github.com/saroel01/aether-cbt/internal/models"
)

// defaultExamDurationMinutes mirrors the exam.durasi_menit schema default and is applied
// when a caller omits (zero) the duration.
const defaultExamDurationMinutes = 90

// ExamInput carries the caller-supplied fields for creating/updating an exam definition
// (Requirement 2.1). SoalPackageID and Tingkat/Nama are nullable pointers.
type ExamInput struct {
	MapelID          int
	Tingkat          *string
	SoalPackageID    *int
	DurasiMenit      int
	KKM              float64
	ShuffleQuestions bool
	ShuffleAnswers   bool
	Nama             *string
}

// ExamRepository manages reusable exam definitions. All access is tenant-scoped
// (Requirement 15.2); mapel and soal-package references are validated against the tenant
// on write (Requirements 2.2, 2.3).
type ExamRepository struct {
	db *sql.DB
}

func NewExamRepository(db *sql.DB) *ExamRepository {
	return &ExamRepository{db: db}
}

const examColumns = `id, tenant_id, mapel_id, tingkat, soal_package_id, durasi_menit, kkm, shuffle_questions, shuffle_answers, nama, created_at, updated_at, deleted_at`

func scanExam(s scanner) (*models.Exam, error) {
	e := &models.Exam{}
	var tingkat, nama sql.NullString
	var soalPackageID sql.NullInt64
	var deletedAt sql.NullTime
	if err := s.Scan(
		&e.ID, &e.TenantID, &e.MapelID, &tingkat, &soalPackageID, &e.DurasiMenit, &e.KKM,
		&e.ShuffleQuestions, &e.ShuffleAnswers, &nama, &e.CreatedAt, &e.UpdatedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	e.Tingkat = nullString(tingkat)
	e.SoalPackageID = nullInt(soalPackageID)
	e.Nama = nullString(nama)
	e.DeletedAt = nullTime(deletedAt)
	return e, nil
}

func (r *ExamRepository) mapelExists(tenantID, mapelID int) (bool, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM mapel WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		mapelID, tenantID,
	).Scan(&count)
	return count > 0, err
}

func (r *ExamRepository) packageExists(tenantID, packageID int) (bool, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM soal_package WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		packageID, tenantID,
	).Scan(&count)
	return count > 0, err
}

// validateReferences checks that the mapel (required) and the optional soal package both
// belong to the tenant, returning ErrInvalidReference otherwise (Requirements 2.2, 2.3).
func (r *ExamRepository) validateReferences(tenantID int, in ExamInput) error {
	ok, err := r.mapelExists(tenantID, in.MapelID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidReference
	}
	if in.SoalPackageID != nil {
		ok, err := r.packageExists(tenantID, *in.SoalPackageID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvalidReference
		}
	}
	return nil
}

// Create inserts a new exam definition after validating its references (Requirement 2.1).
func (r *ExamRepository) Create(tenantID int, in ExamInput) (*models.Exam, error) {
	if err := r.validateReferences(tenantID, in); err != nil {
		return nil, err
	}
	durasi := in.DurasiMenit
	if durasi <= 0 {
		durasi = defaultExamDurationMinutes
	}
	res, err := r.db.Exec(`
		INSERT INTO exam (tenant_id, mapel_id, tingkat, soal_package_id, durasi_menit, kkm, shuffle_questions, shuffle_answers, nama)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tenantID, in.MapelID, in.Tingkat, in.SoalPackageID, durasi, in.KKM, in.ShuffleQuestions, in.ShuffleAnswers, in.Nama)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(tenantID, int(id))
}

// GetByID returns the exam for the tenant, or ErrNotFound (Requirement 2.4, 15.2).
func (r *ExamRepository) GetByID(tenantID, id int) (*models.Exam, error) {
	e, err := scanExam(r.db.QueryRow(
		`SELECT `+examColumns+` FROM exam WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		id, tenantID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return e, nil
}

// List returns every exam definition in the tenant, newest first (Requirement 2.4).
func (r *ExamRepository) List(tenantID int) ([]models.Exam, error) {
	rows, err := r.db.Query(
		`SELECT `+examColumns+` FROM exam WHERE tenant_id = ? AND deleted_at IS NULL ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exams []models.Exam
	for rows.Next() {
		e, err := scanExam(rows)
		if err != nil {
			return nil, err
		}
		exams = append(exams, *e)
	}
	return exams, rows.Err()
}

// Update overwrites the exam definition's fields after validating references. It returns
// ErrNotFound if no such exam belongs to the tenant (Requirement 2.2, 2.3).
func (r *ExamRepository) Update(tenantID, id int, in ExamInput) (*models.Exam, error) {
	if _, err := r.GetByID(tenantID, id); err != nil {
		return nil, err
	}
	if err := r.validateReferences(tenantID, in); err != nil {
		return nil, err
	}
	durasi := in.DurasiMenit
	if durasi <= 0 {
		durasi = defaultExamDurationMinutes
	}
	res, err := r.db.Exec(`
		UPDATE exam SET
			mapel_id = ?, tingkat = ?, soal_package_id = ?, durasi_menit = ?, kkm = ?,
			shuffle_questions = ?, shuffle_answers = ?, nama = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, in.MapelID, in.Tingkat, in.SoalPackageID, durasi, in.KKM, in.ShuffleQuestions, in.ShuffleAnswers, in.Nama, id, tenantID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(tenantID, id)
}

// Delete soft-deletes the exam definition. It returns ErrConflict if any scheduled or
// active session still references it (Requirement 2.5) and ErrNotFound if no such exam
// belongs to the tenant. Soft delete keeps the row for audit and result history
// (Requirement 2.6).
func (r *ExamRepository) Delete(tenantID, id int) error {
	if _, err := r.GetByID(tenantID, id); err != nil {
		return err
	}
	var active int
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM exam_session
		 WHERE exam_id = ? AND tenant_id = ? AND status IN ('terjadwal', 'aktif') AND deleted_at IS NULL`,
		id, tenantID,
	).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return ErrConflict
	}
	_, err := r.db.Exec(`UPDATE exam SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND tenant_id = ?`, id, tenantID)
	return err
}
