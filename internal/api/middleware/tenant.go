package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
)

// TenantMiddleware extracts tenant from header.
// Supports:
//   - X-Tenant-ID: 2
//   - X-Tenant-Slug: sman1kluet
// Falls back to tenant 1 (default) for development.
func TenantMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Priority 1: explicit ID header
		if idStr := c.Get("X-Tenant-ID"); idStr != "" {
			if n, err := parseInt(idStr); err == nil && n > 0 {
				c.Locals("tenant_id", n)
				return c.Next()
			}
		}

		// Priority 2: slug header → lookup
		if slug := c.Get("X-Tenant-Slug"); slug != "" {
			var id int
			err := db.DB.QueryRow("SELECT id FROM tenants WHERE slug = ? AND deleted_at IS NULL", slug).Scan(&id)
			if err == nil && id > 0 {
				c.Locals("tenant_id", id)
				return c.Next()
			}
		}

		// Default for development / single tenant
		c.Locals("tenant_id", 1)
		return c.Next()
	}
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// GetTenantID retrieves tenant_id from context
func GetTenantID(c *fiber.Ctx) int {
	tenantID := c.Locals("tenant_id")
	if tenantID == nil {
		return 1
	}
	return tenantID.(int)
}
