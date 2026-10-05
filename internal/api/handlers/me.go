package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// minProfilePasswordLen is the minimum length for a self-service password change (M5).
const minProfilePasswordLen = 8

// Me returns current logged-in account info (protected). The account table depends on
// the role: users (admin/superadmin), ruang (supervisor) or peserta (student) — M4.
func Me(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	var q string
	switch role {
	case "supervisor":
		q = `SELECT username, nama_ruang FROM ruang WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`
	case "student":
		q = `SELECT no_id, nama_peserta FROM peserta WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`
	default:
		q = `SELECT username, full_name FROM users WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`
	}

	var username, fullName string
	if err := db.DB.QueryRow(q, userID, tenantID).Scan(&username, &fullName); err != nil {
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

// UpdateMyProfile lets an admin change their own username and/or password, and a
// supervisor change the room password. Requires the current password. Every password
// change bumps token_version so previously issued JWTs stop working (M5).
func UpdateMyProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	tenantID := c.Locals("tenant_id").(int)
	role, _ := c.Locals("role").(string)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewUsername     string `json:"new_username"`
		NewPassword     string `json:"new_password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	table := "users"
	switch role {
	case "admin", "superadmin":
	case "supervisor":
		table = "ruang"
		if strings.TrimSpace(req.NewUsername) != "" {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Username ruang dikelola admin")
		}
	default:
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Forbidden")
	}

	if req.CurrentPassword == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Current password is required")
	}

	// Fetch current password hash
	var storedHash string
	err := db.DB.QueryRow(
		"SELECT password_hash FROM "+table+" WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL",
		userID, tenantID).Scan(&storedHash)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "User not found")
	}

	// Verify current password
	if !utils.CheckPasswordHash(req.CurrentPassword, storedHash) {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Current password is incorrect")
	}

	updates := []string{}
	args := []interface{}{}

	// Update username if provided (admin only; rejected above for supervisors)
	if strings.TrimSpace(req.NewUsername) != "" {
		updates = append(updates, "username = ?")
		args = append(args, strings.TrimSpace(req.NewUsername))
	}

	// Update password if provided
	if req.NewPassword != "" {
		if len(req.NewPassword) < minProfilePasswordLen {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "New password must be at least 8 characters")
		}
		newHash, err := utils.HashPassword(req.NewPassword)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to hash password")
		}
		updates = append(updates, "password_hash = ?", "token_version = token_version + 1")
		args = append(args, newHash)
	}

	if len(updates) == 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "No changes provided")
	}

	// Add user ID and tenant for WHERE clause
	args = append(args, userID, tenantID)

	query := "UPDATE " + table + " SET " + strings.Join(updates, ", ") + " WHERE id = ? AND tenant_id = ?"
	_, err = db.DB.Exec(query, args...)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update profile")
	}

	return utils.SuccessResponse(c, nil, "Profile updated successfully")
}
