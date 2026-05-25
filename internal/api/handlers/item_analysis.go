package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

type ItemDifficultyAnalysis struct {
	QuestionID               string  `json:"question_id"`
	QuestionText             string  `json:"question_text"`
	QuestionType             string  `json:"question_type"`
	CorrectCount             int     `json:"correct_count"`
	TotalAttempts            int     `json:"total_attempts"`
	SuccessRate              float64 `json:"success_rate"`
	DifficultyClassification string  `json:"difficulty_classification"`
}

// GetItemAnalysis calculates question statistics and classifies difficulty based on pedagogical standards
func GetItemAnalysis(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" && role != "supervisor" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	mapelID := c.QueryInt("mapel_id", 0)

	query := `
		SELECT hd.question_id, hd.question_text, hd.question_type,
		       SUM(CASE WHEN hd.status = 'correct' THEN 1 ELSE 0 END) as correct_count,
		       COUNT(hd.id) as total_attempts
		FROM hasil_tes_detail hd
		JOIN hasil_tes h ON hd.hasil_tes_id = h.id
		WHERE h.tenant_id = ?
	`
	var args []interface{}
	args = append(args, tenantID)

	if mapelID > 0 {
		query += " AND h.mapel_id = ?"
		args = append(args, mapelID)
	}

	query += " GROUP BY hd.question_id, hd.question_text, hd.question_type"

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to calculate question analytics")
	}
	defer rows.Close()

	var list []ItemDifficultyAnalysis
	for rows.Next() {
		var a ItemDifficultyAnalysis
		err = rows.Scan(&a.QuestionID, &a.QuestionText, &a.QuestionType, &a.CorrectCount, &a.TotalAttempts)
		if err == nil {
			if a.TotalAttempts > 0 {
				a.SuccessRate = (float64(a.CorrectCount) / float64(a.TotalAttempts)) * 100.0
			} else {
				a.SuccessRate = 0.0
			}

			// Pedagogical difficulty classification rules
			if a.SuccessRate > 85.0 {
				a.DifficultyClassification = "Sangat Mudah"
			} else if a.SuccessRate >= 70.0 {
				a.DifficultyClassification = "Mudah"
			} else if a.SuccessRate >= 50.0 {
				a.DifficultyClassification = "Sedang"
			} else if a.SuccessRate >= 30.0 {
				a.DifficultyClassification = "Sukar"
			} else {
				a.DifficultyClassification = "Sangat Sukar"
			}

			list = append(list, a)
		}
	}

	return utils.SuccessResponse(c, list, "Item difficulty analysis retrieved successfully")
}
