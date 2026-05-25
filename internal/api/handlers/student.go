package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetStudents returns all students in current tenant
func GetStudents(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	rows, err := db.DB.Query(`
		SELECT id, no_id, nama_peserta, kelas_id, ruang_id, created_at 
		FROM peserta 
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch students")
	}
	defer rows.Close()

	type Student struct {
		ID          int    `json:"id"`
		NoID        string `json:"no_id"`
		NamaPeserta string `json:"nama_peserta"`
		KelasID     int    `json:"kelas_id"`
		RuangID     int    `json:"ruang_id"`
		CreatedAt   string `json:"created_at"`
	}

	var students []Student
	for rows.Next() {
		var s Student
		rows.Scan(&s.ID, &s.NoID, &s.NamaPeserta, &s.KelasID, &s.RuangID, &s.CreatedAt)
		students = append(students, s)
	}

	return utils.SuccessResponse(c, students, "Students retrieved")
}

// CreateStudent creates a new peserta
func CreateStudent(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		NoID         string `json:"no_id"`
		Password     string `json:"password"`
		NamaPeserta  string `json:"nama_peserta"`
		KelasID      int    `json:"kelas_id"`
		RuangID      int    `json:"ruang_id"`
		JenisKelamin string `json:"jenis_kelamin"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	if req.Password == "" {
		req.Password = "siswa123"
	}

	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to secure student password")
	}

	_, err = db.DB.Exec(`
		INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id, jenis_kelamin)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, tenantID, req.NoID, passwordHash, req.NamaPeserta, req.KelasID, req.RuangID, req.JenisKelamin)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create student")
	}

	return utils.SuccessResponse(c, nil, "Student created successfully")
}

// DeleteStudent soft-deletes a student record
func DeleteStudent(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators can delete students")
	}

	id := c.Params("id")
	_, err := db.DB.Exec(`
		UPDATE peserta 
		SET deleted_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND tenant_id = ?
	`, id, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete student")
	}

	return utils.SuccessResponse(c, nil, "Student deleted successfully")
}
