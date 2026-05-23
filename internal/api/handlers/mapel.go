package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/utils"
)

// GetMapel returns all subjects in current tenant
func GetMapel(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	rows, err := db.DB.Query(`
		SELECT id, nama_mapel, kode_mapel, created_at 
		FROM mapel 
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch subjects")
	}
	defer rows.Close()

	type Mapel struct {
		ID         int    `json:"id"`
		NamaMapel  string `json:"nama_mapel"`
		KodeMapel  string `json:"kode_mapel"`
		CreatedAt  string `json:"created_at"`
	}

	var mapels []Mapel
	for rows.Next() {
		var m Mapel
		rows.Scan(&m.ID, &m.NamaMapel, &m.KodeMapel, &m.CreatedAt)
		mapels = append(mapels, m)
	}

	return utils.SuccessResponse(c, mapels, "Subjects retrieved")
}

// CreateMapel creates a new subject
func CreateMapel(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		NamaMapel string `json:"nama_mapel"`
		KodeMapel string `json:"kode_mapel"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	_, err := db.DB.Exec(`
		INSERT INTO mapel (tenant_id, nama_mapel, kode_mapel) VALUES (?, ?, ?)
	`, tenantID, req.NamaMapel, req.KodeMapel)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create subject")
	}

	return utils.SuccessResponse(c, nil, "Subject created successfully")
}

// DeleteMapel soft-deletes a subject
func DeleteMapel(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators can delete subjects")
	}

	id := c.Params("id")
	_, err := db.DB.Exec(`
		UPDATE mapel 
		SET deleted_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND tenant_id = ?
	`, id, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete subject")
	}

	return utils.SuccessResponse(c, nil, "Subject deleted successfully")
}
