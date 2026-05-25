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
		SELECT id, nama_kelas, created_at 
		FROM kelas 
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch classes")
	}
	defer rows.Close()

	type Class struct {
		ID        int    `json:"id"`
		NamaKelas string `json:"nama_kelas"`
		CreatedAt string `json:"created_at"`
	}

	var classes []Class
	for rows.Next() {
		var k Class
		rows.Scan(&k.ID, &k.NamaKelas, &k.CreatedAt)
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
	role := c.Locals("role").(string)

	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators can delete classes")
	}

	id := c.Params("id")
	_, err := db.DB.Exec(`
		UPDATE kelas 
		SET deleted_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND tenant_id = ?
	`, id, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete class")
	}

	return utils.SuccessResponse(c, nil, "Class deleted successfully")
}
