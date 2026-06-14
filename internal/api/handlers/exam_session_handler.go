package handlers

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/service"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// newSchedulingService wires the scheduling service over fresh repositories bound to the
// global DB (cheap to construct per request).
func newSchedulingService() *service.SchedulingService {
	return service.NewSchedulingService(
		repository.NewExamSessionRepository(db.DB),
		repository.NewExamRepository(db.DB),
	)
}

// ListExamSessions returns the tenant's sessions (Requirement 4.6).
func ListExamSessions(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	sessions, err := repository.NewExamSessionRepository(db.DB).List(tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to list exam sessions")
	}
	return utils.SuccessResponse(c, sessions, "Exam sessions retrieved")
}

// CreateExamSession creates a session after the scheduling service validates the window and
// token overlap (Requirement 4.1, 4.2, 4.4).
func CreateExamSession(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	in, ok := readSessionInput(c)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body (waktu_mulai/waktu_selesai must be RFC3339)")
	}
	if err := newSchedulingService().ValidateCreate(tenantID, in); err != nil {
		return mapScheduleError(c, err)
	}
	sess, err := repository.NewExamSessionRepository(db.DB).Create(tenantID, in)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidReference) {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Referenced exam not found in tenant")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create exam session")
	}
	return utils.SuccessResponse(c, sess, "Exam session created")
}

// UpdateExamSession updates a session after re-validating window/token overlap and the
// package-required rule for status transitions (Requirement 4.2, 4.3, 4.4).
func UpdateExamSession(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid session id")
	}
	in, ok := readSessionInput(c)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body (waktu_mulai/waktu_selesai must be RFC3339)")
	}
	if err := newSchedulingService().ValidateUpdate(tenantID, id, in); err != nil {
		return mapScheduleError(c, err)
	}
	sess, err := repository.NewExamSessionRepository(db.DB).Update(tenantID, id, in)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Exam session not found")
		case errors.Is(err, repository.ErrInvalidReference):
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Referenced exam not found in tenant")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update exam session")
	}
	return utils.SuccessResponse(c, sess, "Exam session updated")
}

// DeleteExamSession soft-deletes a session (Requirement 4.7).
func DeleteExamSession(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid session id")
	}
	if err := repository.NewExamSessionRepository(db.DB).Delete(tenantID, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Exam session not found")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete exam session")
	}
	return utils.SuccessResponse(c, nil, "Exam session deleted")
}

// LinkSessionClasses attaches classes to a session; cross-tenant ids are rejected
// (Requirement 4.7, 5.1).
func LinkSessionClasses(c *fiber.Ctx) error {
	return attachSession(c, "classes")
}

// LinkSessionRooms attaches rooms to a session (Requirement 4.7, 5.2).
func LinkSessionRooms(c *fiber.Ctx) error {
	return attachSession(c, "rooms")
}

func attachSession(c *fiber.Ctx, kind string) error {
	tenantID := c.Locals("tenant_id").(int)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid session id")
	}
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}
	repo := repository.NewExamSessionRepository(db.DB)
	var aErr error
	if kind == "classes" {
		aErr = repo.AttachClasses(tenantID, id, req.IDs)
	} else {
		aErr = repo.AttachRooms(tenantID, id, req.IDs)
	}
	if aErr != nil {
		if errors.Is(aErr, repository.ErrInvalidReference) {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "One or more "+kind+" do not belong to the tenant")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to link "+kind)
	}
	return utils.SuccessResponse(c, nil, kind+" linked")
}

// readSessionInput parses the session body. Timestamps must be RFC3339.
func readSessionInput(c *fiber.Ctx) (repository.SessionInput, bool) {
	var req struct {
		ExamID       int     `json:"exam_id"`
		Nama         *string `json:"nama"`
		WaktuMulai   string  `json:"waktu_mulai"`
		WaktuSelesai string  `json:"waktu_selesai"`
		Token        string  `json:"token"`
		Status       string  `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return repository.SessionInput{}, false
	}
	mulai, err := time.Parse(time.RFC3339, req.WaktuMulai)
	if err != nil {
		return repository.SessionInput{}, false
	}
	selesai, err := time.Parse(time.RFC3339, req.WaktuSelesai)
	if err != nil {
		return repository.SessionInput{}, false
	}
	return repository.SessionInput{
		ExamID:       req.ExamID,
		Nama:         req.Nama,
		WaktuMulai:   mulai,
		WaktuSelesai: selesai,
		Token:        req.Token,
		Status:       req.Status,
	}, true
}

// mapScheduleError maps scheduling-service errors to HTTP responses.
func mapScheduleError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidWindow):
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Session window invalid: end must be after start")
	case errors.Is(err, service.ErrTokenConflict):
		return utils.ErrorResponse(c, fiber.StatusConflict, "Session token overlaps another session's time window")
	case errors.Is(err, service.ErrPackageRequired):
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Link a soal package to the exam before scheduling or activating")
	}
	return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Session validation failed")
}
