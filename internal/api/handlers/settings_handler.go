package handlers

import (
	"database/sql"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

type SettingsResponse struct {
	ExamTitle    string `json:"exam_title"`
	ProctorName  string `json:"proctor_name"`
	FooterText   string `json:"footer_text"`
	Token        string `json:"token"`
	IsExamActive bool   `json:"is_exam_active"`
}

// GetSettings retrieves the configurations for the active tenant
func GetSettings(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	s, err := getSettingsForTenant(tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to retrieve settings")
	}

	return utils.SuccessResponse(c, s, "Settings retrieved")
}

// GetSupervisorSettings returns read-only exam settings needed by room supervisors.
func GetSupervisorSettings(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "supervisor" && role != "admin" && role != "superadmin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only exam staff can view supervisor settings")
	}

	s, err := getSettingsForTenant(tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to retrieve settings")
	}

	return utils.SuccessResponse(c, s, "Supervisor settings retrieved")
}

func getSettingsForTenant(tenantID int) (SettingsResponse, error) {
	var s SettingsResponse
	err := db.DB.QueryRow(`
		SELECT exam_title, COALESCE(proctor_name, ''), COALESCE(footer_text, ''), token, is_exam_active 
		FROM settings 
		WHERE tenant_id = ?
	`, tenantID).Scan(&s.ExamTitle, &s.ProctorName, &s.FooterText, &s.Token, &s.IsExamActive)

	if err != nil {
		if err == sql.ErrNoRows {
			// Seed settings automatically if missing. The default token is a crypto-random
			// value per tenant (NOT a known constant) so it cannot be guessed (review H22,
			// Task 18). GenerateSecureToken(16) yields 32 hex chars.
			randomToken, genErr := utils.GenerateSecureToken(16)
			if genErr != nil {
				return SettingsResponse{}, fmt.Errorf("generate default exam token: %w", genErr)
			}
			_, _ = db.DB.Exec(`
				INSERT INTO settings (tenant_id, exam_title, token, is_exam_active)
				VALUES (?, 'Ujian Akhir Semester 2025/2026', ?, TRUE)
			`, tenantID, randomToken)

			s.ExamTitle = "Ujian Akhir Semester 2025/2026"
			s.Token = randomToken
			s.IsExamActive = true
		} else {
			return SettingsResponse{}, err
		}
	}

	return s, nil
}

// UpdateSettings updates active tenant configurations
func UpdateSettings(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators can modify settings")
	}

	var req SettingsResponse
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if req.ExamTitle == "" || req.Token == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Exam Title and Token are required")
	}

	_, err := db.DB.Exec(`
		INSERT INTO settings (tenant_id, exam_title, proctor_name, footer_text, token, is_exam_active)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id) DO UPDATE SET
			exam_title = excluded.exam_title,
			proctor_name = excluded.proctor_name,
			footer_text = excluded.footer_text,
			token = excluded.token,
			is_exam_active = excluded.is_exam_active
	`, tenantID, req.ExamTitle, req.ProctorName, req.FooterText, req.Token, req.IsExamActive)

	if err != nil {
		// Try fallback update if no unique constraint on tenant_id in schema (schema might not have unique index, so we do direct UPDATE or INSERT)
		_, err = db.DB.Exec(`
			UPDATE settings 
			SET exam_title = ?, proctor_name = ?, footer_text = ?, token = ?, is_exam_active = ?
			WHERE tenant_id = ?
		`, req.ExamTitle, req.ProctorName, req.FooterText, req.Token, req.IsExamActive, tenantID)

		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to save settings")
		}
	}

	return utils.SuccessResponse(c, nil, "Settings updated successfully")
}
