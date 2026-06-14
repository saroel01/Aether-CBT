package repository

import "database/sql"

// GradeRepository manages the grade level (tingkat) attribute on classes. All access is
// tenant-scoped: a class can only be modified or observed within its owning tenant
// (Requirement 1.5).
type GradeRepository struct {
	db *sql.DB
}

func NewGradeRepository(db *sql.DB) *GradeRepository {
	return &GradeRepository{db: db}
}

// SetTingkat assigns a grade level (e.g. "X"/"XI"/"XII") to a class. It returns
// ErrNotFound when no class with the given id belongs to the tenant (Requirement 1.1).
func (r *GradeRepository) SetTingkat(tenantID, kelasID int, tingkat string) error {
	res, err := r.db.Exec(`
		UPDATE kelas
		SET tingkat = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, tingkat, kelasID, tenantID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTingkat returns the distinct, non-empty grade levels assigned to the tenant's
// classes, ordered for stable display (Requirement 1.4). Empty/null values (classes
// whose grade is not yet set) are excluded (Requirement 1.3).
func (r *GradeRepository) ListTingkat(tenantID int) ([]string, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT tingkat FROM kelas
		WHERE tenant_id = ? AND tingkat IS NOT NULL AND tingkat <> '' AND deleted_at IS NULL
		ORDER BY tingkat
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var levels []string
	for rows.Next() {
		var level string
		if err := rows.Scan(&level); err != nil {
			return nil, err
		}
		levels = append(levels, level)
	}
	return levels, rows.Err()
}
