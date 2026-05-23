package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/models"
	"github.com/anomalyco/aether-cbt/internal/utils"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// Login handles user authentication
func Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	tenantID := c.Locals("tenant_id").(int)

	var user models.User
	var passwordHash string
	err := db.DB.QueryRow(`
		SELECT id, tenant_id, username, password_hash, role, full_name, is_active, last_login, created_at, updated_at
		FROM users 
		WHERE username = ? AND tenant_id = ? AND is_active = TRUE AND deleted_at IS NULL
	`, req.Username, tenantID).Scan(
		&user.ID, &user.TenantID, &user.Username, &passwordHash,
		&user.Role, &user.FullName, &user.IsActive, &user.LastLogin,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	if !utils.CheckPasswordHash(req.Password, passwordHash) {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	// Generate JWT token
	token, err := utils.GenerateToken(user.ID, user.TenantID, user.Role)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to generate token")
	}

	// Update last login
	now := time.Now()
	user.LastLogin = &now
	db.DB.Exec("UPDATE users SET last_login = ? WHERE id = ?", now, user.ID)

	return utils.SuccessResponse(c, LoginResponse{
		Token: token,
		User:  &user,
	}, "Login successful")
}
