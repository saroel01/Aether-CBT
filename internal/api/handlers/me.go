package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/utils"
)

// Me returns current logged-in user info (protected)
func Me(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	var username, fullName string
	err := db.DB.QueryRow(`
		SELECT username, full_name 
		FROM users 
		WHERE id = ? AND tenant_id = ?
	`, userID, tenantID).Scan(&username, &fullName)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "User not found")
	}

	return utils.SuccessResponse(c, fiber.Map{
		"id":        userID,
		"tenant_id": tenantID,
		"username":  username,
		"full_name": fullName,
		"role":      role,
	}, "OK")
}
