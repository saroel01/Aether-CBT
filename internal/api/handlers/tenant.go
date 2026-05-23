package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/models"
	"github.com/anomalyco/aether-cbt/internal/utils"
)

// GetAllTenants - Only accessible by admin or superadmin
func GetAllTenants(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "superadmin" && role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Access denied")
	}

	rows, err := db.DB.Query(`
		SELECT id, slug, name, logo, is_active, created_at, updated_at 
		FROM tenants 
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch tenants")
	}
	defer rows.Close()

	var tenants []models.Tenant
	for rows.Next() {
		var t models.Tenant
		rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Logo, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
		tenants = append(tenants, t)
	}

	return utils.SuccessResponse(c, tenants, "Tenants retrieved successfully")
}

// CreateTenant - Only admin or superadmin
func CreateTenant(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "superadmin" && role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Access denied")
	}

	var req struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	_, err := db.DB.Exec(`
		INSERT INTO tenants (slug, name) VALUES (?, ?)
	`, req.Slug, req.Name)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create tenant")
	}

	return utils.SuccessResponse(c, nil, "Tenant created successfully")
}
