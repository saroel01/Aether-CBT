package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// ListExams returns the tenant's exam definitions (Requirement 2.4).
func ListExams(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	exams, err := repository.NewExamRepository(db.DB).List(tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to list exams")
	}
	return utils.SuccessResponse(c, exams, "Exams retrieved")
}

// CreateExam creates a new exam definition (Requirement 2.1, 2.2, 2.3).
func CreateExam(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	in, ok := readExamInput(c)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}
	exam, err := repository.NewExamRepository(db.DB).Create(tenantID, in)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidReference) {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Referenced mapel or soal package not found in tenant")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create exam")
	}
	return utils.SuccessResponse(c, exam, "Exam created")
}

// UpdateExam updates an exam definition (Requirement 2.2, 2.3).
func UpdateExam(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid exam id")
	}
	in, ok := readExamInput(c)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}
	exam, err := repository.NewExamRepository(db.DB).Update(tenantID, id, in)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Exam not found")
		case errors.Is(err, repository.ErrInvalidReference):
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Referenced mapel or soal package not found in tenant")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update exam")
	}
	return utils.SuccessResponse(c, exam, "Exam updated")
}

// DeleteExam soft-deletes an exam definition, rejecting if a session is scheduled/active
// (Requirement 2.5, 2.6).
func DeleteExam(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid exam id")
	}
	if err := repository.NewExamRepository(db.DB).Delete(tenantID, id); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Exam not found")
		case errors.Is(err, repository.ErrConflict):
			return utils.ErrorResponse(c, fiber.StatusConflict, "Exam has a scheduled or active session")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete exam")
	}
	return utils.SuccessResponse(c, nil, "Exam deleted")
}

// readExamInput parses and validates the exam request body. The second return is false
// when parsing fails.
func readExamInput(c *fiber.Ctx) (repository.ExamInput, bool) {
	var req struct {
		MapelID          int     `json:"mapel_id"`
		Tingkat          *string `json:"tingkat"`
		SoalPackageID    *int    `json:"soal_package_id"`
		DurasiMenit      int     `json:"durasi_menit"`
		KKM              float64 `json:"kkm"`
		ShuffleQuestions bool    `json:"shuffle_questions"`
		ShuffleAnswers   bool    `json:"shuffle_answers"`
		Nama             *string `json:"nama"`
	}
	if err := c.BodyParser(&req); err != nil {
		return repository.ExamInput{}, false
	}
	return repository.ExamInput{
		MapelID:          req.MapelID,
		Tingkat:          req.Tingkat,
		SoalPackageID:    req.SoalPackageID,
		DurasiMenit:      req.DurasiMenit,
		KKM:              req.KKM,
		ShuffleQuestions: req.ShuffleQuestions,
		ShuffleAnswers:   req.ShuffleAnswers,
		Nama:             req.Nama,
	}, true
}
