package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

var errScopeForbidden = errors.New("role not allowed")

// resultScope returns an extra WHERE fragment (joined on peserta alias "p") restricting
// results to the caller's room (M3). Admin/superadmin see the whole tenant (""); a
// supervisor (user_id = ruang.id) sees peserta assigned to the room, plus unassigned
// peserta whose class sits in a session held in that room — the same predicate as
// fetchRoomStatus / AssertSupervisorOwnsPeserta without a session filter.
func resultScope(c *fiber.Ctx) (string, []any, error) {
	role, _ := c.Locals("role").(string)
	switch role {
	case "admin", "superadmin":
		return "", nil, nil
	case "supervisor":
		ruangID, _ := c.Locals("user_id").(int)
		return ` AND (p.ruang_id = ? OR ((p.ruang_id IS NULL OR p.ruang_id = 0) AND EXISTS (
			SELECT 1 FROM exam_session_ruang esr
			JOIN exam_session_kelas esk ON esk.session_id = esr.session_id
			WHERE esr.ruang_id = ? AND esk.kelas_id = p.kelas_id)))`, []any{ruangID, ruangID}, nil
	default:
		return "", nil, errScopeForbidden
	}
}
