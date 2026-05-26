package handlers

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
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
	tenantID := c.Locals("tenant_id").(int)

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

	// Single SELECT: cek_login JOIN peserta (Requirement 4.7).
	var pesertaID, mapelID int
	var expectedToken string
	err := db.DB.QueryRowContext(c.Context(), `
		SELECT p.id, cl.mapel_id, COALESCE(cl.attempt_token, '')
		  FROM peserta p
		  JOIN cek_login cl ON cl.peserta_id = p.id AND cl.tenant_id = p.tenant_id
		 WHERE p.tenant_id = ? AND p.no_id = ?
		 LIMIT 1
	`, tenantID, noID).Scan(&pesertaID, &mapelID, &expectedToken)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Status(fiber.StatusForbidden).SendString("active session not found")
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("session lookup failed")
	}
	if expectedToken == "" || subtle.ConstantTimeCompare([]byte(attemptToken), []byte(expectedToken)) != 1 {
		return c.Status(fiber.StatusForbidden).SendString("invalid attempt token")
	}

	if detailXML != "" {
		if _, err := ispringparser.ParseDetailedResults(detailXML); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid iSpring detailed results XML")
		}
	}

	job := &submission.SubmissionJob{
		TenantID:     tenantID,
		NoID:         noID,
		Score:        score,
		MaxScore:     maxScore,
		DetailXML:    detailXML,
		AttemptToken: attemptToken,
		Validasi:     fmt.Sprintf("%d_%s_%d", tenantID, noID, mapelID),
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
		rows.Scan(&q.QuestionID, &q.QuestionText, &q.QuestionType, &q.CorrectCount, &q.TotalCount)
		list = append(list, q)
	}

	return utils.SuccessResponse(c, list, "Educational analysis retrieved")
}
