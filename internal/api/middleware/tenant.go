package middleware

import (
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
)

// TenantMiddleware extracts tenant from header, query param, form value, or subdomain.
// Supports:
//   - X-Tenant-ID: 2
//   - X-Tenant-Slug: sman1kluet
//   - Subdomain: sman1kluet.aethercbt.id
// In development: falls back to tenant 1 for convenience.
// In production: requires explicit tenant identifier (returns 400 if missing).
func TenantMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Priority 1: explicit ID (Header, Query Parameter, or Form Value)
		idStr := c.Get("X-Tenant-ID")
		if idStr == "" {
			idStr = c.Query("tenant_id")
		}
		if idStr == "" {
			idStr = c.FormValue("tenant_id")
		}
		if idStr != "" {
			if n, err := parseInt(idStr); err == nil && n > 0 {
				c.Locals("tenant_id", n)
				return c.Next()
			}
		}

		// Priority 2: slug (Header, Query Parameter, or Form Value) → lookup
		slug := c.Get("X-Tenant-Slug")
		if slug == "" {
			slug = c.Query("tenant_slug")
		}
		if slug == "" {
			slug = c.FormValue("tenant_slug")
		}
		if slug != "" {
			var id int
			err := db.DB.QueryRow("SELECT id FROM tenants WHERE slug = ? AND deleted_at IS NULL", slug).Scan(&id)
			if err == nil && id > 0 {
				c.Locals("tenant_id", id)
				return c.Next()
			}
		}

		// Priority 3: Subdomain detection from hostname (for Cloud VPS deployment)
		host := c.Hostname()
		parts := strings.Split(host, ".")
		if len(parts) >= 3 {
			// e.g. "sman1kluet.aethercbt.id" -> first part is "sman1kluet"
			subdomain := parts[0]
			if subdomain != "www" && subdomain != "api" {
				var id int
				err := db.DB.QueryRow("SELECT id FROM tenants WHERE slug = ? AND deleted_at IS NULL", subdomain).Scan(&id)
				if err == nil && id > 0 {
					c.Locals("tenant_id", id)
					return c.Next()
				}
			}
		}

		// Default only allowed in development for convenience
		env := os.Getenv("ENV")
		if env == "development" || env == "dev" {
			c.Locals("tenant_id", 1)
			return c.Next()
		}

		// In production/staging: require explicit tenant identification
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Tenant ID is required (X-Tenant-ID or X-Tenant-Slug header)",
		})
	}
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// GetTenantID retrieves tenant_id from context.
// Returns 0 if not set (callers should handle this case).
func GetTenantID(c *fiber.Ctx) int {
	tenantID := c.Locals("tenant_id")
	if tenantID == nil {
		return 0
	}
	if id, ok := tenantID.(int); ok {
		return id
	}
	return 0
}
