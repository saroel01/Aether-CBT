package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// antiCheatLockThreshold is the infraction count at which a session is server-locked
// (Requirement 10.6). Wired from config at startup via SetAntiCheatLockThreshold.
var antiCheatLockThreshold = 3

// SetAntiCheatLockThreshold configures the lock threshold from the loaded configuration.
func SetAntiCheatLockThreshold(n int) {
	if n > 0 {
		antiCheatLockThreshold = n
	}
}

// RecordInfraction records a tab-switch / focus-loss infraction. On the session-based path it
// increments the server-side counter and server-locks the session once the configured
// threshold is reached (Requirements 10.1, 10.2, 10.6); the legacy mapel-based path is kept
// during the transition (Requirement 6.6). The response reports the new count and locked
// state so the client can react immediately.
func RecordInfraction(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	var req struct {
		PesertaID int `json:"peserta_id"`
		SessionID int `json:"session_id"`
		MapelID   int `json:"mapel_id"` // legacy path
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if req.PesertaID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid student ID")
	}
	if role == "student" {
		if userID, _ := c.Locals("user_id").(int); userID != req.PesertaID {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Students can only record their own infractions")
		}
	}

	if req.SessionID > 0 {
		cekRepo := repository.NewCekLoginRepository(db.DB)
		count, err := cekRepo.IncrementInfraction(tenantID, req.PesertaID, req.SessionID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return utils.ErrorResponse(c, fiber.StatusNotFound, "No active session found")
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to record infraction")
		}
		locked := false
		if count >= antiCheatLockThreshold {
			if err := cekRepo.Lock(tenantID, req.PesertaID, req.SessionID); err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to lock session")
			}
			locked = true
		}
		return utils.SuccessResponse(c, fiber.Map{
			"infraction_count": count,
			"locked":           locked,
		}, "Infraction recorded")
	}

	// Legacy mapel-based path (transition, Requirement 6.6).
	if req.MapelID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "session_id or mapel_id is required")
	}
	// Lock guard on the legacy path mirrors the session-based path (review H1, Task 13).
	var legacyLocked bool
	_ = db.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(locked, 0) FROM cek_login WHERE tenant_id = ? AND peserta_id = ? AND mapel_id = ?`,
		tenantID, req.PesertaID, req.MapelID,
	).Scan(&legacyLocked)
	if legacyLocked {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Session is locked; infraction not recorded")
	}
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

// UpdateStudentProgress records answered/total counts. The client debounces the calls; the
// server performs a single light UPDATE keyed by session_id (idempotent) when available,
// rejecting a server-locked session (Requirement 10.3, Property 11), and falls back to the
// legacy mapel-based path otherwise (Requirement 13.2, AD-6, 6.6).
func UpdateStudentProgress(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		PesertaID      int `json:"peserta_id"`
		SessionID      int `json:"session_id"`
		MapelID        int `json:"mapel_id"` // legacy path
		AnsweredCount  int `json:"answered_count"`
		TotalQuestions int `json:"total_questions"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if req.PesertaID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid student ID")
	}
	// Ownership: a student may only update their OWN progress (review H2, Task 12). Admin/
	// supervisor roles (proctor resets) are unaffected; this mirrors RecordInfraction's check.
	if role := c.Locals("role"); role == "student" {
		if userID, _ := c.Locals("user_id").(int); userID != req.PesertaID {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Students can only update their own progress")
		}
	}

	if req.SessionID > 0 {
		cekRepo := repository.NewCekLoginRepository(db.DB)
		// A server-locked session is refused: the student may not continue (Property 11).
		if locked, _ := cekRepo.IsLocked(tenantID, req.PesertaID, req.SessionID); locked {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Session is locked; progress not recorded")
		}
		err := cekRepo.UpdateProgress(tenantID, req.PesertaID, req.SessionID, req.AnsweredCount, req.TotalQuestions)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return utils.ErrorResponse(c, fiber.StatusNotFound, "No active session found")
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update exam progress")
		}
		return utils.SuccessResponse(c, nil, "Progress updated successfully")
	}

	// Legacy mapel-based path (transition).
	if req.MapelID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "session_id or mapel_id is required")
	}
	// Lock guard on the legacy path mirrors the session-based path (review H1, Task 13).
	var legacyLocked bool
	_ = db.DB.QueryRowContext(c.Context(),
		`SELECT COALESCE(locked, 0) FROM cek_login WHERE tenant_id = ? AND peserta_id = ? AND mapel_id = ?`,
		tenantID, req.PesertaID, req.MapelID,
	).Scan(&legacyLocked)
	if legacyLocked {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Session is locked; progress not recorded")
	}
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
