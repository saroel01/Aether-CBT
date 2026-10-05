package handlers

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetActiveExamInfo returns configuration title and active status
func GetActiveExamInfo(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var title, proctor, footer string
	var isActive bool
	err := db.DB.QueryRow(`
		SELECT exam_title, proctor_name, footer_text, is_exam_active
		FROM settings
		WHERE tenant_id = ?
	`, tenantID).Scan(&title, &proctor, &footer, &isActive)

	if err != nil {
		// Return friendly defaults if no custom settings exist yet
		return utils.SuccessResponse(c, fiber.Map{
			"exam_title":     "Ujian Akhir Semester 2025/2026",
			"proctor_name":   "Proktor Utama",
			"footer_text":    "Aether CBT - Modern Testing System",
			"is_exam_active": true,
		}, "Default settings retrieved")
	}

	return utils.SuccessResponse(c, fiber.Map{
		"exam_title":     title,
		"proctor_name":   proctor,
		"footer_text":    footer,
		"is_exam_active": isActive,
	}, "Active exam settings retrieved")
}

// GetAvailableMapels returns subjects active for the current tenant, filtered by student's class if peserta_id is provided
func GetAvailableMapels(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	pesertaID := c.QueryInt("peserta_id", 0)
	if role, _ := c.Locals("role").(string); role == "student" {
		// L6: a student only ever sees their own class mapping; the query is ignored.
		pesertaID, _ = c.Locals("user_id").(int)
	}

	var rows *sql.Rows
	var err error

	if pesertaID > 0 {
		// Resolve student's class ID. kelas_id became nullable with clause 2.2, so COALESCE
		// keeps an unassigned student on exactly the pre-fix path: the class-scoped query
		// below matches no kelas_mapel row and the student sees no subjects, rather than
		// falling through to the "all subjects in tenant" branch (clause 2.17, 3.14).
		var kelasID int
		err = db.DB.QueryRow(`
			SELECT COALESCE(kelas_id, 0)
			FROM peserta
			WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
		`, pesertaID, tenantID).Scan(&kelasID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return utils.ErrorResponse(c, fiber.StatusNotFound, "Student not found")
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to resolve student class")
		}

		// Query only subjects mapped to this class. Do not fall through on 0 rows or query error.
		rows, err = db.DB.Query(`
			SELECT m.id, m.nama_mapel, m.kode_mapel
			FROM mapel m
			JOIN kelas_mapel km ON m.id = km.mapel_id
			WHERE km.kelas_id = ? AND km.is_active = TRUE AND m.tenant_id = ? AND m.deleted_at IS NULL
		`, kelasID, tenantID)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch mapped subjects")
		}
	} else {
		// No peserta_id specified: return all tenant subjects
		rows, err = db.DB.Query(`
			SELECT id, nama_mapel, kode_mapel
			FROM mapel
			WHERE tenant_id = ? AND deleted_at IS NULL
		`, tenantID)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch mapels")
		}
	}
	defer rows.Close()

	type MapelItem struct {
		ID        int    `json:"id"`
		NamaMapel string `json:"nama_mapel"`
		KodeMapel string `json:"kode_mapel"`
	}

	var list []MapelItem
	for rows.Next() {
		var m MapelItem
		if err := rows.Scan(&m.ID, &m.NamaMapel, &m.KodeMapel); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read subject")
		}
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to iterate subjects")
	}

	return utils.SuccessResponse(c, list, "Subjects list retrieved successfully")
}

// StartExamSession registers a student's active exam session for a session_id (eligibility +
// lock + content cookie, Requirements 7.1-7.4). session_id is required (audit M6).
func StartExamSession(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	var req struct {
		PesertaID int    `json:"peserta_id"`
		SessionID int    `json:"session_id"`
		Token     string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if req.PesertaID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid student ID")
	}
	if role == "student" {
		if userID, _ := c.Locals("user_id").(int); userID != req.PesertaID {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Students can only start their own exam session")
		}
	}

	attemptToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create exam attempt token")
	}

	cekRepo := repository.NewCekLoginRepository(db.DB)

	if req.SessionID <= 0 {
		// The legacy mapel_id path was removed (audit M6): it bypassed session eligibility,
		// the window check and the content cookie.
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "session_id is required")
	}

	svc := newSchedulingService()
	sessionRepo := repository.NewExamSessionRepository(db.DB)

	session, err := sessionRepo.GetByID(tenantID, req.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Session not found")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load session")
	}
	// M1: the caller must present this session's own token, so a token for one session
	// cannot be used to enter another session the student is also eligible for.
	if strings.TrimSpace(req.Token) == "" ||
		subtle.ConstantTimeCompare([]byte(req.Token), []byte(session.Token)) != 1 {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Token sesi tidak valid untuk sesi ini")
	}
	if reason := svc.NotEnterableReason(session); reason != "" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, reason)
	}
	eligible, err := svc.IsParticipantEligible(tenantID, req.PesertaID, req.SessionID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to verify eligibility")
	}
	if !eligible {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "You are not eligible for this session")
	}
	if locked, _ := cekRepo.IsLocked(tenantID, req.PesertaID, req.SessionID); locked {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Session is locked; contact your supervisor")
	}
	// Check if student has already submitted this exam session (P0-2)
	var sessionAlreadySubmitted bool
	err = db.DB.QueryRowContext(c.Context(), `
		SELECT 1 FROM hasil_tes
		WHERE tenant_id = ? AND peserta_id = ? AND exam_session_id = ? AND status = 'submitted'
		LIMIT 1
	`, tenantID, req.PesertaID, req.SessionID).Scan(&sessionAlreadySubmitted)
	if err == nil && sessionAlreadySubmitted {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Exam session has already been submitted")
	}

	if err := cekRepo.Start(tenantID, req.PesertaID, req.SessionID, attemptToken); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return utils.ErrorResponse(c, fiber.StatusConflict, "You already have an active exam session; submit or have it reset first")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to register exam session")
	}

	// Issue the content-serving cookie (Requirement 8.1, AD-2).
	if contentToken, err := utils.GenerateSecureToken(32); err == nil {
		_ = cekRepo.SetContentToken(tenantID, req.PesertaID, req.SessionID, contentToken)
		setContentCookie(c, contentToken)
	}
	return utils.SuccessResponse(c, fiber.Map{
		"attempt_token": attemptToken,
		"session_id":    req.SessionID,
	}, "Exam session registered successfully")
}

// GetRemainingTime computes remaining seconds. For session-based sessions it is
// min(duration, sessionEnd - now) clamped to 0 (Requirement 7.5, Property 6); the legacy
// mapel-based path keeps the original duration-minus-elapsed behavior (Req 6.6).
func GetRemainingTime(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	pesertaID := c.QueryInt("peserta_id", 0)
	sessionID := c.QueryInt("session_id", 0)
	mapelID := c.QueryInt("mapel_id", 0)

	if role == "student" {
		pesertaID = c.Locals("user_id").(int)
	}
	if pesertaID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid peserta_id")
	}

	if sessionID > 0 {
		cekRepo := repository.NewCekLoginRepository(db.DB)
		cek, err := cekRepo.GetBySession(tenantID, pesertaID, sessionID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return utils.SuccessResponse(c, fiber.Map{"remaining_seconds": 0, "is_active": false}, "No active session found, remaining time is 0")
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to check session active time")
		}
		session, err := repository.NewExamSessionRepository(db.DB).GetByID(tenantID, sessionID)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load session")
		}
		exam, err := repository.NewExamRepository(db.DB).GetByID(tenantID, session.ExamID)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load exam")
		}
		remaining := newSchedulingService().RemainingSeconds(session, exam, cek.LoginTime)
		return utils.SuccessResponse(c, fiber.Map{
			"remaining_seconds": remaining,
			"is_active":         remaining > 0 && !cek.Locked,
			"locked":            cek.Locked,
			// P2: when the server-authoritative clock hits 0, signal the shim to force-submit the
			// quiz (player.submitQuiz) so answers are captured even if the student never clicked
			// Submit in iSpring. The shim treats this as idempotent (guards with a flag) so a manual
			// submit followed by time-up does not double-submit. Polled by the shim every few seconds,
			// which is more reliable than a single push (a dropped WS/SSE at the moment of expiry
			// would otherwise lose the signal).
			"force_submit": remaining <= 0,
		}, "Remaining time calculated successfully")
	}

	// Legacy mapel-based path (transition).
	if mapelID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid session_id or mapel_id")
	}

	var loginTime time.Time
	var lockedInt int
	err := db.DB.QueryRow(`
		SELECT login_time, COALESCE(locked, 0)
		FROM cek_login
		WHERE tenant_id = ? AND peserta_id = ? AND mapel_id = ?
	`, tenantID, pesertaID, mapelID).Scan(&loginTime, &lockedInt)
	if err != nil {
		if err == sql.ErrNoRows {
			return utils.SuccessResponse(c, fiber.Map{
				"remaining_seconds": 0,
				"is_active":         false,
				"locked":            false,
				"force_submit":      false,
			}, "No active session found, remaining time is 0")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to check session active time")
	}

	var durasiMenit int = 90
	if err := db.DB.QueryRow(`SELECT COALESCE(durasi_menit, 90) FROM mapel WHERE tenant_id = ? AND id = ?`, tenantID, mapelID).Scan(&durasiMenit); err != nil {
		durasiMenit = 90
	}

	duration := time.Duration(durasiMenit) * time.Minute
	elapsed := time.Now().UTC().Sub(loginTime.UTC())
	remainingSeconds := int((duration - elapsed).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}

	isLocked := lockedInt == 1
	return utils.SuccessResponse(c, fiber.Map{
		"remaining_seconds": remainingSeconds,
		"login_time":        loginTime,
		"duration_minutes":  durasiMenit,
		"is_active":         remainingSeconds > 0 && !isLocked,
		"locked":            isLocked,
		"force_submit":      remainingSeconds <= 0,
	}, "Remaining time calculated successfully")
}
