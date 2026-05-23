package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/utils"
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

	// Validate student
	var pesertaID int
	err := db.DB.QueryRow(`
		SELECT id FROM peserta 
		WHERE no_id = ? AND password = ? AND tenant_id = ?
	`, req.NoID, req.Password, tenantID).Scan(&pesertaID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	return utils.SuccessResponse(c, fiber.Map{"peserta_id": pesertaID}, "Login successful")
}
