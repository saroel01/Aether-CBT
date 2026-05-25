package handlers

import (
	"database/sql"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

type EssayAnswerResponse struct {
	DetailID      int     `json:"detail_id"`
	HasilTesID    int     `json:"hasil_tes_id"`
	QuestionID    string  `json:"question_id"`
	QuestionText  string  `json:"question_text"`
	AwardedPoints float64 `json:"awarded_points"`
	MaxPoints     float64 `json:"max_points"`
	UserAnswer    string  `json:"user_answer"`
	CorrectAnswer string  `json:"correct_answer"`
	PesertaID     int     `json:"peserta_id"`
	NoID          string  `json:"no_id"`
	NamaPeserta   string  `json:"nama_peserta"`
	NamaKelas     string  `json:"nama_kelas"`
	NamaMapel     string  `json:"nama_mapel"`
}

// GetEssayAnswers returns all essay questions answered by students, with dynamic class & subject filters
func GetEssayAnswers(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" && role != "supervisor" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	kelasID := c.QueryInt("kelas_id", 0)
	mapelID := c.QueryInt("mapel_id", 0)

	query := `
		SELECT htd.id, htd.hasil_tes_id, htd.question_id, htd.question_text, htd.awarded_points, htd.max_points, htd.user_answer, htd.correct_answer,
		       p.id AS peserta_id, p.no_id, p.nama_peserta, COALESCE(k.nama_kelas, '—') AS nama_kelas, COALESCE(m.nama_mapel, '—') AS nama_mapel
		FROM hasil_tes_detail htd
		JOIN hasil_tes ht ON htd.hasil_tes_id = ht.id
		JOIN peserta p ON ht.peserta_id = p.id
		LEFT JOIN kelas k ON p.kelas_id = k.id
		LEFT JOIN mapel m ON ht.mapel_id = m.id
		WHERE ht.tenant_id = ? AND htd.question_type = 'essayQuestion'
	`
	var args []interface{}
	args = append(args, tenantID)

	if kelasID > 0 {
		query += " AND p.kelas_id = ?"
		args = append(args, kelasID)
	}
	if mapelID > 0 {
		query += " AND ht.mapel_id = ?"
		args = append(args, mapelID)
	}

	query += " ORDER BY p.nama_peserta ASC, htd.id ASC"

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to retrieve essay answers")
	}
	defer rows.Close()

	var list []EssayAnswerResponse
	for rows.Next() {
		var r EssayAnswerResponse
		err = rows.Scan(
			&r.DetailID, &r.HasilTesID, &r.QuestionID, &r.QuestionText, &r.AwardedPoints, &r.MaxPoints, &r.UserAnswer, &r.CorrectAnswer,
			&r.PesertaID, &r.NoID, &r.NamaPeserta, &r.NamaKelas, &r.NamaMapel,
		)
		if err == nil {
			list = append(list, r)
		}
	}

	return utils.SuccessResponse(c, list, "Essay answers retrieved successfully")
}

type GradeEssayRequest struct {
	DetailID      int     `json:"detail_id"`
	AwardedPoints float64 `json:"awarded_points"`
}

// GradeEssayAnswer updates an essay grade and recalculates the final parent score automatically
func GradeEssayAnswer(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators are authorized to grade essays")
	}

	var req GradeEssayRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if req.DetailID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Detail ID is required")
	}

	if req.AwardedPoints < 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Awarded points cannot be negative")
	}

	// Fetch max points and parent hasil_tes_id
	var maxPoints float64
	var hasilTesID int
	err := db.DB.QueryRow(`
		SELECT htd.max_points, htd.hasil_tes_id 
		FROM hasil_tes_detail htd
		JOIN hasil_tes ht ON htd.hasil_tes_id = ht.id
		WHERE htd.id = ? AND ht.tenant_id = ?
	`, req.DetailID, tenantID).Scan(&maxPoints, &hasilTesID)

	if err != nil {
		if err == sql.ErrNoRows {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Essay answer record not found")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to verify essay answer")
	}

	if req.AwardedPoints > maxPoints {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Awarded points cannot exceed maximum points (%.2f)", maxPoints))
	}

	// Determine pedagogical status
	status := "partial"
	if req.AwardedPoints == maxPoints {
		status = "correct"
	} else if req.AwardedPoints == 0 {
		status = "incorrect"
	}

	// Begin transaction to ensure consistency
	tx, err := db.DB.Begin()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to start database transaction")
	}
	defer tx.Rollback()

	// Update the detail record
	_, err = tx.Exec(`
		UPDATE hasil_tes_detail 
		SET awarded_points = ?, status = ?
		WHERE id = ?
	`, req.AwardedPoints, status, req.DetailID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update essay grade")
	}

	// Recalculate total score
	var totalScore float64
	err = tx.QueryRow(`
		SELECT COALESCE(SUM(awarded_points), 0) 
		FROM hasil_tes_detail 
		WHERE hasil_tes_id = ?
	`, hasilTesID).Scan(&totalScore)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to recalculate total exam score")
	}

	// Update parent exam result
	_, err = tx.Exec(`
		UPDATE hasil_tes 
		SET skor = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, totalScore, hasilTesID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update final exam result")
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to commit database transaction")
	}

	return utils.SuccessResponse(c, fiber.Map{
		"hasil_tes_id":   hasilTesID,
		"new_total_skor": totalScore,
		"status":         status,
	}, "Essay answer graded and total score recalculated successfully")
}
