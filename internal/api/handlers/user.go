package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetUsers returns all users in current tenant
func GetUsers(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	rows, err := db.DB.Query(`
		SELECT id, tenant_id, username, role, full_name, is_active, created_at 
		FROM users 
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch users")
	}
	defer rows.Close()

	type UserResponse struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		Role      string `json:"role"`
		FullName  string `json:"full_name"`
		IsActive  bool   `json:"is_active"`
		CreatedAt string `json:"created_at"`
	}

	var users []UserResponse
	for rows.Next() {
		var u UserResponse
		rows.Scan(&u.ID, &u.Username, &u.Role, &u.FullName, &u.IsActive, &u.CreatedAt)
		users = append(users, u)
	}

	return utils.SuccessResponse(c, users, "Users retrieved")
}

// CreateUser creates a new user in current tenant
func CreateUser(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		FullName string `json:"full_name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	// Role allowlist (review H15, Task 24): an admin may only mint admin/supervisor accounts,
	// never superadmin (which is reserved for out-of-band provisioning).
	allowedRoles := map[string]bool{"admin": true, "supervisor": true}
	if !allowedRoles[req.Role] {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "role must be 'admin' or 'supervisor'")
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "username and password are required")
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to hash password")
	}

	_, err = db.DB.Exec(`
		INSERT INTO users (tenant_id, username, password_hash, role, full_name)
		VALUES (?, ?, ?, ?, ?)
	`, tenantID, req.Username, hash, req.Role, req.FullName)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create user")
	}

	return utils.SuccessResponse(c, nil, "User created successfully")
}
