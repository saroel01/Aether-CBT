package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
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
		if err := rows.Scan(&m.ID, &m.NamaMapel, &m.KodeMapel, &m.CreatedAt); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read subject")
		}
		mapels = append(mapels, m)
	}
	if err := rows.Err(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to iterate subjects")
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

	// Clause 2.18, 2.19: validate ID int, require deleted_at IS NULL, check RowsAffected
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid subject ID")
	}

	res, err := db.DB.Exec(`
		UPDATE mapel 
		SET deleted_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, id, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete subject")
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to check subject deletion")
	}
	if affected == 0 {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Subject not found")
	}

	return utils.SuccessResponse(c, nil, "Subject deleted successfully")
}
