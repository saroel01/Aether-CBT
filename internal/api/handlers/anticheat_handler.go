package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// RecordInfraction increments the student's tab switch count during an exam
func RecordInfraction(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		PesertaID int `json:"peserta_id"`
		MapelID   int `json:"mapel_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if req.PesertaID <= 0 || req.MapelID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid student ID or subject ID")
	}

	// Increment tab_switch_count in cek_login (active sessions)
	_, err := db.DB.Exec(`
		UPDATE cek_login 
		SET tab_switch_count = tab_switch_count + 1, last_activity = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND peserta_id = ? AND mapel_id = ?
	`, tenantID, req.PesertaID, req.MapelID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to log anticheat infraction")
	}

	return utils.SuccessResponse(c, nil, "Infraction recorded successfully")
}

// UpdateStudentProgress updates the count of answered questions for a student session
func UpdateStudentProgress(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		PesertaID      int `json:"peserta_id"`
		MapelID        int `json:"mapel_id"`
		AnsweredCount  int `json:"answered_count"`
		TotalQuestions int `json:"total_questions"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if req.PesertaID <= 0 || req.MapelID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid student ID or subject ID")
	}

	// Update in cek_login (active sessions)
	_, err := db.DB.Exec(`
		UPDATE cek_login 
		SET answered_count = ?, total_questions = ?, last_activity = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND peserta_id = ? AND mapel_id = ?
	`, req.AnsweredCount, req.TotalQuestions, tenantID, req.PesertaID, req.MapelID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update exam progress")
	}

	return utils.SuccessResponse(c, nil, "Progress updated successfully")
}

