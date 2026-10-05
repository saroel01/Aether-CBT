package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

type ItemDifficultyAnalysis struct {
	SoalPackageID            int     `json:"soal_package_id"`
	PackageNama              string  `json:"package_nama"`
	MapelID                  int     `json:"mapel_id"`
	QuestionID               string  `json:"question_id"`
	QuestionText             string  `json:"question_text"`
	QuestionType             string  `json:"question_type"`
	CorrectCount             int     `json:"correct_count"`
	TotalAttempts            int     `json:"total_attempts"`
	SuccessRate              float64 `json:"success_rate"`
	DifficultyClassification string  `json:"difficulty_classification"`
}

// itemStat is one question's aggregate within a soal package (M9).
type itemStat struct {
	SoalPackageID int
	PackageNama   string
	MapelID       int
	QuestionID    string
	QuestionText  string
	QuestionType  string
	CorrectCount  int
	TotalCount    int
}

// queryItemStats aggregates hasil_tes_detail per soal package, so the same question_id in
// two different packages is never merged (M9). Results without an exam session (legacy)
// fall into soal_package_id 0, grouped per mapel. Optional filters: mapel_id,
// package_id, session_id. Supervisors are scoped to their room (M3).
func queryItemStats(c *fiber.Ctx, tenantID int) ([]itemStat, error) {
	scope, scopeArgs, err := resultScope(c)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT COALESCE(e.soal_package_id, 0) AS pkg_id,
		       COALESCE(MAX(sp.nama), '') AS pkg_nama,
		       CASE WHEN e.soal_package_id IS NULL THEN h.mapel_id ELSE 0 END AS grp_mapel,
		       hd.question_id, hd.question_text, hd.question_type,
		       SUM(CASE WHEN hd.status = 'correct' THEN 1 ELSE 0 END) AS correct_count,
		       COUNT(hd.id) AS total_attempts
		FROM hasil_tes_detail hd
		JOIN hasil_tes h ON hd.hasil_tes_id = h.id
		JOIN peserta p ON h.peserta_id = p.id
		LEFT JOIN exam_session es ON es.id = h.exam_session_id
		LEFT JOIN exam e ON e.id = es.exam_id
		LEFT JOIN soal_package sp ON sp.id = e.soal_package_id
		WHERE h.tenant_id = ?` + scope
	args := append([]any{tenantID}, scopeArgs...)
	if v := c.QueryInt("mapel_id", 0); v > 0 {
		query += " AND h.mapel_id = ?"
		args = append(args, v)
	}
	if v := c.QueryInt("package_id", 0); v > 0 {
		query += " AND e.soal_package_id = ?"
		args = append(args, v)
	}
	if v := c.QueryInt("session_id", 0); v > 0 {
		query += " AND h.exam_session_id = ?"
		args = append(args, v)
	}
	query += ` GROUP BY pkg_id, grp_mapel, hd.question_id, hd.question_text, hd.question_type
		ORDER BY pkg_id, grp_mapel, hd.question_id`

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []itemStat
	for rows.Next() {
		var s itemStat
		if err := rows.Scan(&s.SoalPackageID, &s.PackageNama, &s.MapelID, &s.QuestionID, &s.QuestionText,
			&s.QuestionType, &s.CorrectCount, &s.TotalCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetItemAnalysis calculates question statistics and classifies difficulty based on pedagogical standards
func GetItemAnalysis(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	stats, err := queryItemStats(c, tenantID)
	if err == errScopeForbidden {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to calculate question analytics")
	}

	var list []ItemDifficultyAnalysis
	for _, s := range stats {
		a := ItemDifficultyAnalysis{
			SoalPackageID: s.SoalPackageID, PackageNama: s.PackageNama, MapelID: s.MapelID,
			QuestionID: s.QuestionID, QuestionText: s.QuestionText, QuestionType: s.QuestionType,
			CorrectCount: s.CorrectCount, TotalAttempts: s.TotalCount,
		}
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

	return utils.SuccessResponse(c, list, "Item difficulty analysis retrieved successfully")
}
