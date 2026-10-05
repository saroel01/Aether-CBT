package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/utils"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// DefaultBcryptCost specifies the standard bcrypt cost factor (10) for authentication (P1-8).
// Lowering from 14 to 10 prevents CPU exhaustion when hundreds of students log in concurrently in a lab.
const DefaultBcryptCost = utils.BcryptCost

// Login handles user authentication
func Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	tenantID := c.Locals("tenant_id").(int)

	var user models.User
	var passwordHash string
	var tokenVersion int
	err := db.DB.QueryRow(`
		SELECT id, tenant_id, username, password_hash, role, full_name, is_active, last_login, created_at, updated_at, token_version
		FROM users 
		WHERE username = ? AND tenant_id = ? AND is_active = TRUE AND deleted_at IS NULL
	`, req.Username, tenantID).Scan(
		&user.ID, &user.TenantID, &user.Username, &passwordHash,
		&user.Role, &user.FullName, &user.IsActive, &user.LastLogin,
		&user.CreatedAt, &user.UpdatedAt, &tokenVersion,
	)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	if !utils.CheckPasswordHash(req.Password, passwordHash) {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	if user.Role == "supervisor" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Supervisor accounts must log in via /api/auth/supervisor-login with room credentials")
	}

	// Generate JWT token
	token, err := utils.GenerateTokenWithVersion(user.ID, user.TenantID, user.Role, tokenVersion)
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
