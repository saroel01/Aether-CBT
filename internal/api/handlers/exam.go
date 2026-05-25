package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// StudentLogin - Login khusus untuk peserta ujian
func StudentLogin(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		NoID     string `json:"no_id"`
		Password string `json:"password"`
		Token    string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	// Validate token from settings
	var globalToken string
	db.DB.QueryRow("SELECT token FROM settings WHERE tenant_id = ?", tenantID).Scan(&globalToken)
	if req.Token != globalToken {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid exam token")
	}

	var pesertaID int
	var storedPassword string
	err := db.DB.QueryRow(`
		SELECT id, password FROM peserta 
		WHERE no_id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, req.NoID, tenantID).Scan(&pesertaID, &storedPassword)

	if err != nil || !utils.CheckPasswordOrPlaintext(req.Password, storedPassword) {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	jwtToken, err := utils.GenerateToken(pesertaID, tenantID, "student")
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to generate student session token")
	}

	return utils.SuccessResponse(c, fiber.Map{
		"peserta_id": pesertaID,
		"token":      jwtToken,
		"user": fiber.Map{
			"id":        pesertaID,
			"username":  req.NoID,
			"role":      "student",
			"tenant_id": tenantID,
		},
	}, "Login successful")
}
