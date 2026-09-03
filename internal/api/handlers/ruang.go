package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetRooms returns all rooms in current tenant
func GetRooms(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	rows, err := db.DB.Query(`
		SELECT id, nama_ruang, username, created_at 
		FROM ruang 
		WHERE tenant_id = ? AND deleted_at IS NULL
	`, tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch rooms")
	}
	defer rows.Close()

	type Room struct {
		ID        int    `json:"id"`
		NamaRuang string `json:"nama_ruang"`
		Username  string `json:"username"`
		CreatedAt string `json:"created_at"`
	}

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.NamaRuang, &r.Username, &r.CreatedAt); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read room")
		}
		rooms = append(rooms, r)
	}
	if err := rows.Err(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to iterate rooms")
	}

	return utils.SuccessResponse(c, rooms, "Rooms retrieved")
}

// CreateRoom creates a new room
func CreateRoom(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	var req struct {
		NamaRuang string `json:"nama_ruang"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request")
	}

	// Clause 2.21: validate required fields
	if strings.TrimSpace(req.NamaRuang) == "" || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "nama_ruang, username, and password are required")
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to secure room password")
	}

	_, err = db.DB.Exec(`
		INSERT INTO ruang (tenant_id, nama_ruang, username, password_hash) 
		VALUES (?, ?, ?, ?)
	`, tenantID, req.NamaRuang, req.Username, hash)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return utils.ErrorResponse(c, fiber.StatusConflict, "Username already exists in this tenant")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create room")
	}

	return utils.SuccessResponse(c, nil, "Room created successfully")
}

// DeleteRoom soft-deletes an exam room
func DeleteRoom(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	// Clause 2.18, 2.19: validate ID int, require deleted_at IS NULL, check RowsAffected
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid room ID")
	}

	res, err := db.DB.Exec(`
		UPDATE ruang 
		SET deleted_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL
	`, id, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete room")
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to check room deletion")
	}
	if affected == 0 {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Room not found")
	}

	return utils.SuccessResponse(c, nil, "Room deleted successfully")
}
