package middleware

import (
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
)

// isAPIRoute returns true if the request path targets the API namespace ("/api" or "/api/...").
// Case-insensitive comparison aligns with Fiber's default routing behavior.
func isAPIRoute(path string) bool {
	p := strings.ToLower(path)
	return p == "/api" || strings.HasPrefix(p, "/api/")
}

// TenantMiddleware extracts tenant from header, query param, form value, or subdomain.
// Supports:
//   - X-Tenant-ID: 2
//   - X-Tenant-Slug: sman1kluet
//   - Subdomain: sman1kluet.aethercbt.id
// In development: falls back to tenant 1 for convenience.
// In production: requires explicit tenant identifier (returns 400 if missing).
func TenantMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawPath := c.Path()

		// Defensive guard: if path does not target "/api" namespace, pass through immediately.
		// Non-API routes (frontend SPA, static assets, /api-docs, etc.) should never be blocked.
		if !isAPIRoute(rawPath) {
			return c.Next()
		}

		p := strings.ToLower(rawPath)

		// Public API exemptions:
		// 1. GET /api/health (health check service)
		// 2. GET /api/qrcode (public QR code image generator)
		// 3. POST /api/ispring/webhook (iSpring result webhook, authenticates tenant from attempt_token)
		// 4. GET /api/exam/content/* (iSpring quiz assets, authorized by content-session cookie)
		if p == "/api/health" || p == "/api/health/" ||
			p == "/api/qrcode" || p == "/api/qrcode/" ||
			p == "/api/ispring/webhook" || p == "/api/ispring/webhook/" ||
			p == "/api/exam/content" || strings.HasPrefix(p, "/api/exam/content/") {
			return c.Next()
		}

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
		if slug != "" && db.DB != nil {
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
		if len(parts) >= 3 && db.DB != nil {
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
		// Consistent with config.go: fallback to tenant 1 if ENV is unset or "development"/"dev"
		env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
		if env == "" || env == "development" || env == "dev" {
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
	// strconv.Atoi rejects trailing garbage (e.g. "1abc"), unlike the previous fmt.Sscanf
	// which silently parsed a leading int — strict parsing closes a small injection surface
	// (review security finding #7, Task 39).
	return strconv.Atoi(strings.TrimSpace(s))
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
