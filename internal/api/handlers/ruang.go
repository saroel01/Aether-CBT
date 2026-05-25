package handlers

import (
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
		rows.Scan(&r.ID, &r.NamaRuang, &r.Username, &r.CreatedAt)
		rooms = append(rooms, r)
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

	hash, _ := utils.HashPassword(req.Password)

	_, err := db.DB.Exec(`
		INSERT INTO ruang (tenant_id, nama_ruang, username, password_hash) 
		VALUES (?, ?, ?, ?)
	`, tenantID, req.NamaRuang, req.Username, hash)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to create room")
	}

	return utils.SuccessResponse(c, nil, "Room created successfully")
}

// DeleteRoom soft-deletes an exam room
func DeleteRoom(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators can delete rooms")
	}

	id := c.Params("id")
	_, err := db.DB.Exec(`
		UPDATE ruang 
		SET deleted_at = CURRENT_TIMESTAMP 
		WHERE id = ? AND tenant_id = ?
	`, id, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete room")
	}

	return utils.SuccessResponse(c, nil, "Room deleted successfully")
}
