package handlers

import (
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// tenantSlugPattern is a DNS-label-safe slug (L10): it is used as a subdomain and in
// X-Tenant-Slug, so anything else could never resolve and may inject into hostnames.
var tenantSlugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// GetAllTenants - superadmin only (route guarded by superadminOnly; provisioning is out-of-band).
func GetAllTenants(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "superadmin" {
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
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Logo, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read tenant")
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to iterate tenants")
	}

	return utils.SuccessResponse(c, tenants, "Tenants retrieved successfully")
}

// CreateTenant - superadmin only.
func CreateTenant(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "superadmin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Access denied")
	}

	var req struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	name := strings.TrimSpace(req.Name)
	if !tenantSlugPattern.MatchString(slug) {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "slug harus huruf kecil/angka/tanda hubung (maks 63 karakter)")
	}
	if name == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "name is required")
	}

	_, err := db.DB.Exec(`
		INSERT INTO tenants (slug, name) VALUES (?, ?)
	`, slug, name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return utils.ErrorResponse(c, fiber.StatusConflict, "slug already exists")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create tenant")
	}

	return utils.SuccessResponse(c, nil, "Tenant created successfully")
}
