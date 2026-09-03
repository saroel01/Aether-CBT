package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetClasses returns all classes in current tenant
func GetClasses(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	rows, err := db.DB.Query(`
		SELECT id, nama_kelas, tingkat, created_at
		FROM kelas
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch classes")
	}
	defer rows.Close()

	type Class struct {
		ID        int     `json:"id"`
		NamaKelas string  `json:"nama_kelas"`
		Tingkat   *string `json:"tingkat"`
		CreatedAt string  `json:"created_at"`
	}

	var classes []Class
	for rows.Next() {
		var k Class
		if err := rows.Scan(&k.ID, &k.NamaKelas, &k.Tingkat, &k.CreatedAt); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read classes")
		}
		classes = append(classes, k)
	}

	return utils.SuccessResponse(c, classes, "Classes retrieved")
}

// CreateClass creates a new class
func CreateClass(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		NamaKelas string `json:"nama_kelas"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	_, err := db.DB.Exec(`
		INSERT INTO kelas (tenant_id, nama_kelas) VALUES (?, ?)
	`, tenantID, req.NamaKelas)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create class")
	}

	return utils.SuccessResponse(c, nil, "Class created successfully")
}

// DeleteClass soft-deletes a class record
func DeleteClass(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	// Clause 2.18, 2.19: validate ID int, require deleted_at IS NULL, check RowsAffected
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid class ID")
	}

	res, err := db.DB.Exec(`
		UPDATE kelas 
		SET deleted_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, id, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete class")
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to check class deletion")
	}
	if affected == 0 {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Class not found")
	}

	return utils.SuccessResponse(c, nil, "Class deleted successfully")
}
