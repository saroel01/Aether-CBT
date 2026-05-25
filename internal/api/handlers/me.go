package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
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

// UpdateMyProfile allows the logged-in admin to update their own username and/or password.
// Requires current password for security verification.
func UpdateMyProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewUsername     string `json:"new_username"`
		NewPassword     string `json:"new_password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	if req.CurrentPassword == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Current password is required")
	}

	// Fetch current password hash
	var storedHash string
	err := db.DB.QueryRow(`
		SELECT password_hash FROM users 
		WHERE id = ? AND tenant_id = ?
	`, userID, tenantID).Scan(&storedHash)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "User not found")
	}

	// Verify current password
	if !utils.CheckPasswordHash(req.CurrentPassword, storedHash) {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Current password is incorrect")
	}

	updates := []string{}
	args := []interface{}{}

	// Update username if provided
	if strings.TrimSpace(req.NewUsername) != "" {
		updates = append(updates, "username = ?")
		args = append(args, strings.TrimSpace(req.NewUsername))
	}

	// Update password if provided
	if req.NewPassword != "" {
		if len(req.NewPassword) < 6 {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "New password must be at least 6 characters")
		}
		newHash, _ := utils.HashPassword(req.NewPassword)
		updates = append(updates, "password_hash = ?")
		args = append(args, newHash)
	}

	if len(updates) == 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "No changes provided")
	}

	// Add user ID and tenant for WHERE clause
	args = append(args, userID, tenantID)

	query := "UPDATE users SET " + strings.Join(updates, ", ") + " WHERE id = ? AND tenant_id = ?"
	_, err = db.DB.Exec(query, args...)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update profile")
	}

	return utils.SuccessResponse(c, nil, "Profile updated successfully")
}
