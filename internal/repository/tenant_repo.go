package repository

import (
	"database/sql"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/models"
)

type TenantRepository struct{}

func NewTenantRepository() *TenantRepository {
	return &TenantRepository{}
}

func (r *TenantRepository) GetByID(id int) (*models.Tenant, error) {
	tenant := &models.Tenant{}
	err := db.DB.QueryRow(`
		SELECT id, slug, name, logo, is_active, created_at, updated_at 
		FROM tenants 
		WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Logo, &tenant.IsActive, &tenant.CreatedAt, &tenant.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return tenant, nil
}

func (r *TenantRepository) GetBySlug(slug string) (*models.Tenant, error) {
	tenant := &models.Tenant{}
	err := db.DB.QueryRow(`
		SELECT id, slug, name, logo, is_active, created_at, updated_at 
		FROM tenants 
		WHERE slug = ? AND deleted_at IS NULL
	`, slug).Scan(&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Logo, &tenant.IsActive, &tenant.CreatedAt, &tenant.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return tenant, nil
}
