package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// StudentLogin authenticates a participant against an effective exam session when the token
// resolves to one (distinguishing "not started yet" from "ended"), and falls back to the
// legacy global settings.token during the transition (Requirements 6.1-6.6).
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

	resolved := resolveSessionForToken(tenantID, req.Token)
	if resolved.session == nil {
		if !resolved.legacy {
			// A session matched but is not enterable right now (Requirement 6.3).
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, resolved.notEnterable)
		}
		// Legacy fallback: validate against the global settings token (Requirement 6.6).
		var globalToken string
		db.DB.QueryRow("SELECT token FROM settings WHERE tenant_id = ?", tenantID).Scan(&globalToken)
		if req.Token == "" || req.Token != globalToken {
			return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid exam token")
		}
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

	resp := fiber.Map{
		"peserta_id": pesertaID,
		"token":      jwtToken,
		"user": fiber.Map{
			"id":        pesertaID,
			"username":  req.NoID,
			"role":      "student",
			"tenant_id": tenantID,
		},
	}
	if resolved.session != nil {
		resp["session_id"] = resolved.session.ID
		resp["exam_id"] = resolved.session.ExamID
	}
	return utils.SuccessResponse(c, resp, "Login successful")
}
