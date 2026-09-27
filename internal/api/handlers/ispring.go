package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	ispringparser "github.com/saroel01/aether-cbt/internal/ispring"
	"github.com/saroel01/aether-cbt/internal/submission"
	"github.com/saroel01/aether-cbt/internal/utils"
)

var SubmissionQueue submission.Queue

func SetSubmissionQueue(q submission.Queue) {
	SubmissionQueue = q
}

func ISpringWebhook(c *fiber.Ctx) error {
	var tenantID int
	if tid, ok := c.Locals("tenant_id").(int); ok {
		tenantID = tid
	}

	noID := strings.TrimSpace(c.FormValue("sid"))
	if noID == "" {
		noID = strings.TrimSpace(c.FormValue("USER_NAME"))
	}
	if noID == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing student identifier (sid / USER_NAME)")
	}

	score := c.FormValue("sp")
	maxScore := c.FormValue("tp")
	detailXML := c.FormValue("dr")
	attemptToken := c.FormValue("attempt_token")
	if attemptToken == "" {
		attemptToken = c.FormValue("AETHER_ATTEMPT_TOKEN")
	}

	// Clause 2.13: explicit guard for empty attempt token before query execution
	if strings.TrimSpace(attemptToken) == "" {
		return c.Status(fiber.StatusForbidden).SendString("invalid attempt token")
	}

	// P0-1: Numeric validation for score (sp) and maxScore (tp)
	clientScore, err := strconv.ParseFloat(score, 64)
	if err != nil || math.IsNaN(clientScore) || math.IsInf(clientScore, 0) || clientScore < 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid or missing score (sp)")
	}

	clientMaxScore, err := strconv.ParseFloat(maxScore, 64)
	if err != nil || math.IsNaN(clientMaxScore) || math.IsInf(clientMaxScore, 0) || clientMaxScore <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid or missing max score (tp)")
	}

	// P0-1: detail XML (dr) is mandatory
	if strings.TrimSpace(detailXML) == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing iSpring detailed results XML (dr is required)")
	}
	if _, err := ispringparser.ParseDetailedResults(detailXML); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid iSpring detailed results XML")
	}

	// Resolve tenant + session from the attempt_token alone. The webhook is public and
	// the client-controlled X-Tenant-ID (held in c.Locals("tenant_id")) is NOT trusted
	// (review finding H6 / iSpring F3): the attempt_token is crypto-random and globally
	// unique, so it is the authoritative key for both tenant and session.
	var resolvedTenantID int
	var mapelID sql.NullInt64
	var sessionID sql.NullInt64
	var locked bool
	sanitizedNoID := SanitizeFormulaField(noID)
	err = db.DB.QueryRowContext(c.Context(), `
		SELECT cl.tenant_id, cl.mapel_id, cl.session_id, COALESCE(cl.locked, 0)
		  FROM cek_login cl
		  JOIN peserta p ON cl.peserta_id = p.id AND cl.tenant_id = p.tenant_id
		 WHERE cl.attempt_token = ? AND (p.no_id = ? OR p.no_id = ?)
		 LIMIT 1
	`, attemptToken, noID, sanitizedNoID).Scan(&resolvedTenantID, &mapelID, &sessionID, &locked)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Status(fiber.StatusForbidden).SendString("active session not found")
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("session lookup failed")
	}
	if locked {
		return c.Status(fiber.StatusForbidden).SendString("session is locked")
	}
	tenantID = resolvedTenantID // override the header-derived tenant with the authoritative value
	c.Locals("tenant_id", resolvedTenantID)

	if !mapelID.Valid && sessionID.Valid {
		_ = db.DB.QueryRowContext(c.Context(), `
			SELECT e.mapel_id
			FROM exam_session es
			JOIN exam e ON es.exam_id = e.id
			WHERE es.id = ? AND es.tenant_id = ?
		`, sessionID.Int64, tenantID).Scan(&mapelID)
	}

	// validasi: session-based key (tenant_noID_sessionID) for new sessions; the legacy
	// mapel-based key for sessions without a session_id so old results stay reachable
	// (Requirement 14.2, AD-1). The unique index hasil_tes(tenant_id, validasi) is unchanged.
	validasi := fmt.Sprintf("%d_%s_%d", tenantID, noID, mapelID.Int64)
	if sessionID.Valid {
		validasi = fmt.Sprintf("%d_%s_%d", tenantID, noID, int(sessionID.Int64))
	}

	// P0-2: Immutability check - if already submitted, acknowledge idempotently without re-queuing
	var existingStatus string
	_ = db.DB.QueryRowContext(c.Context(), `
		SELECT status FROM hasil_tes WHERE tenant_id = ? AND validasi = ?
	`, tenantID, validasi).Scan(&existingStatus)
	if existingStatus == "submitted" {
		return c.SendString("Result already submitted successfully")
	}

	job := &submission.SubmissionJob{
		TenantID:     tenantID,
		NoID:         noID,
		Score:        score,
		MaxScore:     maxScore,
		DetailXML:    detailXML,
		AttemptToken: attemptToken,
		Validasi:     validasi,
	}
	if err := SubmissionQueue.Enqueue(c.Context(), job); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to queue result")
	}
	return c.SendString("Result received successfully")
}

// GetEducationalAnalysis returns educational breakdowns for mapels
func GetEducationalAnalysis(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	rows, err := db.DB.Query(`
		SELECT hd.question_id, hd.question_text, hd.question_type,
		       SUM(CASE WHEN hd.status = 'correct' THEN 1 ELSE 0 END) as correct_count,
		       COUNT(hd.id) as total_attempts
		FROM hasil_tes_detail hd
		JOIN hasil_tes h ON hd.hasil_tes_id = h.id
		WHERE h.tenant_id = ?
		GROUP BY hd.question_id, hd.question_text, hd.question_type
	`, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load question analytics")
	}
	defer rows.Close()

	type QuestionMetric struct {
		QuestionID   string `json:"question_id"`
		QuestionText string `json:"question_text"`
		QuestionType string `json:"question_type"`
		CorrectCount int    `json:"correct_count"`
		TotalCount   int    `json:"total_count"`
	}

	var list []QuestionMetric
	for rows.Next() {
		var q QuestionMetric
		if err := rows.Scan(&q.QuestionID, &q.QuestionText, &q.QuestionType, &q.CorrectCount, &q.TotalCount); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read question analytics")
		}
		list = append(list, q)
	}
	if err := rows.Err(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to iterate question analytics")
	}

	return utils.SuccessResponse(c, list, "Educational analysis retrieved")
}
