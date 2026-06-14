package repository

import (
	"database/sql"
	"errors"

	"github.com/saroel01/aether-cbt/internal/models"
)

// SoalPackageInput carries the caller-supplied metadata for a new soal package. Nullable
// fields use pointers so the caller can express "unknown" (e.g. iSpring version not
// detected, Requirements 3.6, 3.6a).
type SoalPackageInput struct {
	Nama           string
	PackageUUID    string
	EntryPath      string // defaults to "index.html" when empty
	IspringVersion *string
	TotalSize      int64
	Checksum       *string
	UploadedBy     *int
}

// SoalPackageRepository manages metadata for uploaded iSpring packages. The package
// files live on disk (data/soal/{tenant_slug}/{uuid}) and are handled by the storage
// layer; this repository owns only the DB metadata. All access is tenant-scoped
// (Requirement 15.2).
type SoalPackageRepository struct {
	db *sql.DB
}

func NewSoalPackageRepository(db *sql.DB) *SoalPackageRepository {
	return &SoalPackageRepository{db: db}
}

const soalPackageColumns = `id, tenant_id, nama, package_uuid, entry_path, ispring_version, total_size, checksum, uploaded_by, created_at, updated_at, deleted_at`

func scanSoalPackage(s scanner) (*models.SoalPackage, error) {
	p := &models.SoalPackage{}
	var ispringVersion, checksum sql.NullString
	var uploadedBy sql.NullInt64
	var deletedAt sql.NullTime
	if err := s.Scan(
		&p.ID, &p.TenantID, &p.Nama, &p.PackageUUID, &p.EntryPath,
		&ispringVersion, &p.TotalSize, &checksum, &uploadedBy,
		&p.CreatedAt, &p.UpdatedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	p.IspringVersion = nullString(ispringVersion)
	p.Checksum = nullString(checksum)
	p.UploadedBy = nullInt(uploadedBy)
	p.DeletedAt = nullTime(deletedAt)
	return p, nil
}

// Create inserts a new package metadata row and returns the created record (Requirement 3.6).
func (r *SoalPackageRepository) Create(tenantID int, in SoalPackageInput) (*models.SoalPackage, error) {
	entryPath := in.EntryPath
	if entryPath == "" {
		entryPath = "index.html"
	}
	res, err := r.db.Exec(`
		INSERT INTO soal_package (tenant_id, nama, package_uuid, entry_path, ispring_version, total_size, checksum, uploaded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, tenantID, in.Nama, in.PackageUUID, entryPath, in.IspringVersion, in.TotalSize, in.Checksum, in.UploadedBy)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(tenantID, int(id))
}

// GetByID returns the package for the tenant, or ErrNotFound (Requirement 3.9, 15.2).
func (r *SoalPackageRepository) GetByID(tenantID, id int) (*models.SoalPackage, error) {
	p, err := scanSoalPackage(r.db.QueryRow(
		`SELECT `+soalPackageColumns+` FROM soal_package WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		id, tenantID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// List returns every package in the tenant (Requirement 3.9), newest first.
func (r *SoalPackageRepository) List(tenantID int) ([]models.SoalPackage, error) {
	rows, err := r.db.Query(
		`SELECT `+soalPackageColumns+` FROM soal_package WHERE tenant_id = ? AND deleted_at IS NULL ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []models.SoalPackage
	for rows.Next() {
		p, err := scanSoalPackage(rows)
		if err != nil {
			return nil, err
		}
		packages = append(packages, *p)
	}
	return packages, rows.Err()
}

// Delete removes the package metadata row. It returns ErrConflict if any non-deleted
// exam still links this package (Requirement 3.10), and ErrNotFound if no such package
// belongs to the tenant. The package files on disk are removed separately by the storage
// layer once the metadata row is gone.
func (r *SoalPackageRepository) Delete(tenantID, id int) error {
	if _, err := r.GetByID(tenantID, id); err != nil {
		return err // ErrNotFound
	}
	var linked int
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM exam WHERE soal_package_id = ? AND tenant_id = ? AND deleted_at IS NULL`,
		id, tenantID,
	).Scan(&linked); err != nil {
		return err
	}
	if linked > 0 {
		return ErrConflict
	}
	_, err := r.db.Exec(`DELETE FROM soal_package WHERE id = ? AND tenant_id = ?`, id, tenantID)
	return err
}
