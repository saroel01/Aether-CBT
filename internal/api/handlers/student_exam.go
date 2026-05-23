package handlers

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/utils"
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

	var rows *sql.Rows
	var err error

	if pesertaID > 0 {
		// Resolve student's class ID
		var kelasID int
		err = db.DB.QueryRow(`
			SELECT kelas_id 
			FROM peserta 
			WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
		`, pesertaID, tenantID).Scan(&kelasID)

		if err == nil {
			// Query only subjects mapped to this class
			rows, err = db.DB.Query(`
				SELECT m.id, m.nama_mapel, m.kode_mapel 
				FROM mapel m
				JOIN kelas_mapel km ON m.id = km.mapel_id
				WHERE km.kelas_id = ? AND km.is_active = TRUE AND m.tenant_id = ? AND m.deleted_at IS NULL
			`, kelasID, tenantID)
		}
	}

	// Fallback to all subjects if no peserta_id or class resolve failed
	if rows == nil {
		rows, err = db.DB.Query(`
			SELECT id, nama_mapel, kode_mapel 
			FROM mapel 
			WHERE tenant_id = ? AND deleted_at IS NULL
		`, tenantID)
	}

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch mapels")
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
		rows.Scan(&m.ID, &m.NamaMapel, &m.KodeMapel)
		list = append(list, m)
	}

	return utils.SuccessResponse(c, list, "Subjects list retrieved successfully")
}

// StartExamSession registers a student's active exam session in cek_login
func StartExamSession(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		PesertaID int `json:"peserta_id"`
		MapelID   int `json:"mapel_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if req.PesertaID <= 0 || req.MapelID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid student ID or subject ID")
	}

	// Insert or replace in cek_login (active sessions)
	_, err := db.DB.Exec(`
		INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, login_time, last_activity)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, peserta_id, mapel_id) DO UPDATE SET 
			login_time = CURRENT_TIMESTAMP,
			last_activity = CURRENT_TIMESTAMP
	`, tenantID, req.PesertaID, req.MapelID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to register active exam session")
	}

	return utils.SuccessResponse(c, nil, "Exam session registered successfully")
}
