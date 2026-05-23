package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/utils"
)

type LinkRequest struct {
	KelasID int `json:"kelas_id"`
	MapelID int `json:"mapel_id"`
}

// LinkClassSubject creates a curriculum relationship between Class and Subject
func LinkClassSubject(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators can map curriculum")
	}

	var req LinkRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if req.KelasID <= 0 || req.MapelID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Class ID and Subject ID are required")
	}

	_, err := db.DB.Exec(`
		INSERT INTO kelas_mapel (kelas_id, mapel_id, is_active)
		VALUES (?, ?, TRUE)
		ON CONFLICT(kelas_id, mapel_id) DO UPDATE SET is_active = TRUE
	`, req.KelasID, req.MapelID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to map subject to class")
	}

	return utils.SuccessResponse(c, nil, "Subject successfully mapped to class")
}

// UnlinkClassSubject disables or unlinks curriculum mapping
func UnlinkClassSubject(c *fiber.Ctx) error {
	role := c.Locals("role").(string)
	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators can edit mappings")
	}

	var req LinkRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	_, err := db.DB.Exec(`
		DELETE FROM kelas_mapel 
		WHERE kelas_id = ? AND mapel_id = ?
	`, req.KelasID, req.MapelID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to unlink subject from class")
	}

	return utils.SuccessResponse(c, nil, "Subject successfully unlinked from class")
}

// GetClassSubjects retrieves subjects mapped to a specific Class ID
func GetClassSubjects(c *fiber.Ctx) error {
	kelasIDStr := c.Params("kelas_id")
	kelasID, err := strconv.Atoi(kelasIDStr)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid Class ID")
	}

	tenantID := c.Locals("tenant_id").(int)

	rows, err := db.DB.Query(`
		SELECT m.id, m.nama_mapel, m.kode_mapel 
		FROM mapel m
		JOIN kelas_mapel km ON m.id = km.mapel_id
		WHERE km.kelas_id = ? AND km.is_active = TRUE AND m.tenant_id = ? AND m.deleted_at IS NULL
	`, kelasID, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch mapped subjects")
	}
	defer rows.Close()

	type SubjectItem struct {
		ID        int    `json:"id"`
		NamaMapel string `json:"nama_mapel"`
		KodeMapel string `json:"kode_mapel"`
	}

	var list []SubjectItem
	for rows.Next() {
		var s SubjectItem
		rows.Scan(&s.ID, &s.NamaMapel, &s.KodeMapel)
		list = append(list, s)
	}

	return utils.SuccessResponse(c, list, "Class subjects retrieved")
}
