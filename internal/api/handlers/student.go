package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetStudents returns all students in current tenant
func GetStudents(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	// kelas_id / ruang_id became nullable with clause 2.2 (the sentinel 0 was replaced by a
	// real NULL). COALESCE keeps this payload byte-identical to the pre-fix response — an
	// unassigned reference is still reported as 0 — which preservation clause 3.13 requires.
	rows, err := db.DB.Query(`
		SELECT id, no_id, nama_peserta, COALESCE(kelas_id, 0), COALESCE(ruang_id, 0), created_at 
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
		if err := rows.Scan(&s.ID, &s.NoID, &s.NamaPeserta, &s.KelasID, &s.RuangID, &s.CreatedAt); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read student")
		}
		students = append(students, s)
	}
	if err := rows.Err(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to iterate students")
	}

	return utils.SuccessResponse(c, students, "Students retrieved")
}

// CreateStudent creates a new peserta
func CreateStudent(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	// kelas_id / ruang_id are pointers so "not supplied" is distinguishable from "supplied as
	// 0". Both mean "not assigned" and are normalized to SQL NULL before the INSERT, because
	// both columns are FOREIGN KEYs and no parent row with id 0 exists (clause 2.2).
	var req struct {
		NoID         string `json:"no_id"`
		Password     string `json:"password"`
		NamaPeserta  string `json:"nama_peserta"`
		KelasID      *int64 `json:"kelas_id"`
		RuangID      *int64 `json:"ruang_id"`
		JenisKelamin string `json:"jenis_kelamin"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	kelasRef := db.NullableFKPtr(req.KelasID)
	ruangRef := db.NullableFKPtr(req.RuangID)

	// Validation (review H14, Task 23): no_id required + unique within tenant; kelas/ruang
	// refs must belong to the caller's tenant.
	if strings.TrimSpace(req.NoID) == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "no_id is required")
	}
	var dup int
	_ = db.DB.QueryRowContext(c.Context(),
		`SELECT COUNT(*) FROM peserta WHERE tenant_id = ? AND no_id = ? AND deleted_at IS NULL`,
		tenantID, req.NoID,
	).Scan(&dup)
	if dup > 0 {
		return utils.ErrorResponse(c, fiber.StatusConflict, "no_id already exists in this tenant")
	}
	if kelasRef.Valid {
		var k int
		_ = db.DB.QueryRowContext(c.Context(),
			`SELECT COUNT(*) FROM kelas WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
			kelasRef.Int64, tenantID,
		).Scan(&k)
		if k == 0 {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "class not found in tenant")
		}
	}
	if ruangRef.Valid {
		var r int
		_ = db.DB.QueryRowContext(c.Context(),
			`SELECT COUNT(*) FROM ruang WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
			ruangRef.Int64, tenantID,
		).Scan(&r)
		if r == 0 {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "room not found in tenant")
		}
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
	`, tenantID, req.NoID, passwordHash, req.NamaPeserta, kelasRef, ruangRef, req.JenisKelamin)

	if err != nil {
		// Defensive: a concurrent create could win the uniqueness race; map it to 409.
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return utils.ErrorResponse(c, fiber.StatusConflict, "no_id already exists in this tenant")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create student")
	}

	return utils.SuccessResponse(c, nil, "Student created successfully")
}

// DeleteStudent soft-deletes a student record
func DeleteStudent(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	// Validate the id path param and return 404 when no row matches (review H3-handlers, Task 27).
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "invalid id")
	}
	res, err := db.DB.Exec(`
		UPDATE peserta
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, id, tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete student")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "student not found")
	}

	return utils.SuccessResponse(c, nil, "Student deleted successfully")
}
