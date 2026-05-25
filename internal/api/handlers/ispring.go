package handlers

import (
	"encoding/xml"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// XML structures mapping to iSpring detailed results format
type XMLReport struct {
	XMLName   xml.Name      `xml:"quizReport"`
	Version   string        `xml:"version,attr"`
	Questions XMLQuestions `xml:"questions"`
}

type XMLQuestions struct {
	List []RawXMLQuestion `xml:",any"` // Uses xml:",any" to catch polymorphic tags dynamically
}

type RawXMLQuestion struct {
	XMLName       xml.Name // Contains the actual tag: multipleChoiceQuestion, trueFalseQuestion, etc.
	ID            string   `xml:"id,attr"`
	Status        string   `xml:"status,attr"`
	AwardedPoints float64  `xml:"awardedPoints,attr"`
	MaxPoints     float64  `xml:"maxPoints,attr"`
	
	// Question text
	Direction struct {
		Text string `xml:"text"`
	} `xml:"direction"`

	// For Multiple Choice & True/False (Index-based answers)
	Answers struct {
		CorrectAnswerIndex string `xml:"correctAnswerIndex,attr"`
		UserAnswerIndex    string `xml:"userAnswerIndex,attr"`
		List               []struct {
			Text string `xml:"text"`
		} `xml:"answer"`
	} `xml:"answers"`

	// For Fill in the Blank / Type In (userAnswer attribute & acceptableAnswers)
	UserAnswerAttr    string `xml:"userAnswer,attr"`
	AcceptableAnswers struct {
		List []string `xml:"answer"`
	} `xml:"acceptableAnswers"`

	// For Essay (userAnswer child tag)
	UserAnswerTag string `xml:"userAnswer"`
}

// ISpringWebhook receives quiz results from iSpring, parses XML with Substitution Groups support,
// performs anti-cheat active session checks, and saves the detailed student scores.
func ISpringWebhook(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	// User ID from iSpring (no_id) - check "sid" first, then fallback to "USER_NAME"
	noID := c.FormValue("sid")
	if noID == "" {
		noID = c.FormValue("USER_NAME")
	}

	score := c.FormValue("sp")
	maxScore := c.FormValue("tp")
	detailXML := c.FormValue("dr") // Detailed results XML

	if noID == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing student identifier (sid / USER_NAME)")
	}

	// Find peserta
	var pesertaID int
	err := db.DB.QueryRow("SELECT id FROM peserta WHERE no_id = ? AND tenant_id = ?", noID, tenantID).Scan(&pesertaID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString(fmt.Sprintf("Student with ID %s not found in tenant %d", noID, tenantID))
	}

	// Resolve active subject (mapel) and login_time from cek_login session (Anti-Cheat check)
	var mapelID int
	var loginTime time.Time
	err = db.DB.QueryRow("SELECT mapel_id, login_time FROM cek_login WHERE peserta_id = ? AND tenant_id = ?", pesertaID, tenantID).Scan(&mapelID, &loginTime)
	
	if err != nil || mapelID <= 0 {
		// Strictly enforce active session check for anti-cheat protection.
		return c.Status(fiber.StatusForbidden).SendString("Unauthorized exam attempt: student session not found in room monitor")
	}

	// Fetch durasi_menit from mapel to validate grace period
	var durasiMenit int = 90
	err = db.DB.QueryRow("SELECT COALESCE(durasi_menit, 90) FROM mapel WHERE id = ? AND tenant_id = ?", mapelID, tenantID).Scan(&durasiMenit)
	if err != nil {
		durasiMenit = 90
	}

	// Grace Period: 5 minutes after official exam time limit
	maxAllowedDuration := time.Duration(durasiMenit)*time.Minute + 5*time.Minute
	actualDuration := time.Now().UTC().Sub(loginTime.UTC())

	if actualDuration > maxAllowedDuration {
		return c.Status(fiber.StatusForbidden).SendString("Exam submission rejected: exceeded 5-minute grace period toleration (late submission)")
	}

	validasi := fmt.Sprintf("%d_%s_%d", tenantID, noID, mapelID)

	// Save summary to hasil_tes
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
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to save result summary")
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

	// Parse XML detailed results dynamically
	if detailXML != "" && hasilTesID > 0 {
		var report XMLReport
		if err := xml.Unmarshal([]byte(detailXML), &report); err == nil {
			// Clear existing details if any (to support re-submissions safely)
			db.DB.Exec("DELETE FROM hasil_tes_detail WHERE hasil_tes_id = ?", hasilTesID)

			// Insert each parsed question
			for _, q := range report.Questions.List {
				questionText := q.Direction.Text
				if questionText == "" {
					questionText = "Teks soal tidak tersedia"
				}

				var userAnswer, correctAnswer string
				qType := q.XMLName.Local // e.g., multipleChoiceQuestion, trueFalseQuestion

				switch qType {
				case "multipleChoiceQuestion", "trueFalseQuestion":
					options := q.Answers.List
					
					// Resolve userAnswer from index
					if q.Answers.UserAnswerIndex != "" {
						if idx, err := strconv.Atoi(q.Answers.UserAnswerIndex); err == nil && idx >= 0 && idx < len(options) {
							userAnswer = options[idx].Text
						}
					}
					
					// Resolve correctAnswer from index
					if q.Answers.CorrectAnswerIndex != "" {
						if idx, err := strconv.Atoi(q.Answers.CorrectAnswerIndex); err == nil && idx >= 0 && idx < len(options) {
							correctAnswer = options[idx].Text
						}
					}

				case "multipleResponseQuestion":
					// Multiple response question (Pilihan Ganda Kompleks)
					var userAnsList []string
					for _, opt := range q.Answers.List {
						userAnsList = append(userAnsList, opt.Text)
					}
					userAnswer = "Banyak Pilihan: " + strings.Join(userAnsList, ", ")
					correctAnswer = "Lihat skema kuis asli"

				case "fillInTheBlankQuestion", "typeInQuestion":
					userAnswer = q.UserAnswerAttr
					correctAnswer = strings.Join(q.AcceptableAnswers.List, " ; ")

				case "essayQuestion":
					userAnswer = q.UserAnswerTag
					correctAnswer = "Perlu Penilaian Manual"

				default:
					// Fallback for custom or unsupported question types
					userAnswer = q.UserAnswerAttr
					if userAnswer == "" && q.UserAnswerTag != "" {
						userAnswer = q.UserAnswerTag
					}
					correctAnswer = "Tipe soal kustom"
				}

				_, err = db.DB.Exec(`
					INSERT INTO hasil_tes_detail (
						hasil_tes_id, question_id, question_text, question_type, 
						status, awarded_points, max_points, user_answer, correct_answer
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, hasilTesID, q.ID, questionText, qType, q.Status, q.AwardedPoints, q.MaxPoints, userAnswer, correctAnswer)
				if err != nil {
					log.Printf("Failed to save question detail: %v", err)
				}
			}
		} else {
			log.Printf("XML Unmarshal failed: %v", err)
		}
	}

	// Remove from active cek_login session (student has completed the exam successfully)
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
