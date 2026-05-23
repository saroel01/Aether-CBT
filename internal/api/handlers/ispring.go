package handlers

import (
	"encoding/xml"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/utils"
)

// XML structures mapping to iSpring detailed results format
type XMLQuestion struct {
	ID            string  `xml:"id,attr"`
	Type          string  `xml:"type,attr"`
	Status        string  `xml:"status,attr"`
	AwardedPoints float64 `xml:"awardedPoints,attr"`
	MaxPoints     float64 `xml:"maxPoints,attr"`
	Body          string  `xml:"body"`
	UserAnswer    string  `xml:"userAnswer"`
	CorrectAnswer string  `xml:"correctAnswer"`
}

type XMLQuiz struct {
	Title     string        `xml:"title,attr"`
	Score     float64       `xml:"score,attr"`
	MaxScore  float64       `xml:"maxScore,attr"`
	Questions []XMLQuestion `xml:"question"`
}

type XMLReport struct {
	XMLName xml.Name `xml:"report"`
	Quiz    XMLQuiz  `xml:"quiz"`
}

// ISpringWebhook receives quiz results from iSpring, parses XML, and logs educational analysis details
func ISpringWebhook(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	noID := c.FormValue("sid") // User ID from iSpring (no_id)
	score := c.FormValue("sp")
	maxScore := c.FormValue("tp")
	detailXML := c.FormValue("dr") // Detailed results XML

	if noID == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing student ID")
	}

	// Find peserta
	var pesertaID int
	err := db.DB.QueryRow("SELECT id FROM peserta WHERE no_id = ? AND tenant_id = ?", noID, tenantID).Scan(&pesertaID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Student not found")
	}

	// Resolve active subject (mapel) from cek_login session
	var mapelID int
	db.DB.QueryRow("SELECT mapel_id FROM cek_login WHERE peserta_id = ? AND tenant_id = ?", pesertaID, tenantID).Scan(&mapelID)
	
	if mapelID <= 0 {
		// Fallback: use first mapel in tenant
		db.DB.QueryRow("SELECT id FROM mapel WHERE tenant_id = ? LIMIT 1", tenantID).Scan(&mapelID)
	}

	validasi := fmt.Sprintf("%d_%s_%d", tenantID, noID, mapelID)

	// Save to hasil_tes
	res, err := db.DB.Exec(`
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, skor, skor_maks, detail_xml, status, validasi, waktu_selesai)
		VALUES (?, ?, ?, ?, ?, ?, 'submitted', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, validasi) DO UPDATE SET
			skor = excluded.skor,
			skor_maks = excluded.skor_maks,
			detail_xml = excluded.detail_xml,
			status = 'submitted',
			waktu_selesai = CURRENT_TIMESTAMP
	`, tenantID, pesertaID, mapelID, score, maxScore, detailXML, validasi)

	if err != nil {
		log.Printf("Failed to insert hasil_tes: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to save result")
	}

	// Get the last inserted ID
	lastID, _ := res.LastInsertId()
	var hasilTesID int
	if lastID > 0 {
		hasilTesID = int(lastID)
	} else {
		// If ON CONFLICT was triggered, fetch the existing ID
		db.DB.QueryRow("SELECT id FROM hasil_tes WHERE tenant_id = ? AND validasi = ?", tenantID, validasi).Scan(&hasilTesID)
	}

	// Parse XML detailed results
	if detailXML != "" && hasilTesID > 0 {
		var report XMLReport
		if err := xml.Unmarshal([]byte(detailXML), &report); err == nil {
			// Clear existing details if any (to support re-submissions safely)
			db.DB.Exec("DELETE FROM hasil_tes_detail WHERE hasil_tes_id = ?", hasilTesID)

			// Insert each parsed question
			for _, q := range report.Quiz.Questions {
				_, err = db.DB.Exec(`
					INSERT INTO hasil_tes_detail (
						hasil_tes_id, question_id, question_text, question_type, 
						status, awarded_points, max_points, user_answer, correct_answer
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, hasilTesID, q.ID, q.Body, q.Type, q.Status, q.AwardedPoints, q.MaxPoints, q.UserAnswer, q.CorrectAnswer)
				if err != nil {
					log.Printf("Failed to save question detail: %v", err)
				}
			}
		} else {
			log.Printf("XML Unmarshal failed: %v", err)
		}
	}

	// Remove from active cek_login session
	_, _ = db.DB.Exec("DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ?", pesertaID, tenantID)

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
