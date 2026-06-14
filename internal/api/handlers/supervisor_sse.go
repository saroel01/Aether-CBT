package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetRoomStatusSSE streams real-time room status to supervisors or admins using Server-Sent Events (SSE)
func GetRoomStatusSSE(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)
	ruangID := c.Locals("user_id").(int) // for supervisor, user_id maps to ruang_id

	if role != "supervisor" && role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	if role == "admin" {
		queryRoomID := c.QueryInt("room_id", 0)
		if queryRoomID > 0 {
			ruangID = queryRoomID
		} else {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Room ID required for admin")
		}
	}

	// Set headers for SSE
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	// Capture request-scoped values as locals BEFORE installing the stream writer: the
	// closure runs in a separate goroutine after fasthttp has recycled the Ctx, so c must
	// not be touched inside it (only these locals + db.DB).
	sessionID := c.QueryInt("session_id", 0) // optional: scope to one exam_session

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Send initial heartbeat / comment to establish connection
		fmt.Fprintf(w, ": ok\n\n")
		w.Flush()

		// Stream loop
		for {
			list, err := fetchRoomStatus(tenantID, ruangID, sessionID)
			if err != nil {
				// Database connection or query error, terminate stream
				break
			}

			jsonData, err := json.Marshal(list)
			if err == nil {
				_, err = fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
				if err != nil {
					// Client disconnected
					break
				}
				err = w.Flush()
				if err != nil {
					// Client disconnected
					break
				}
			}

			time.Sleep(2 * time.Second)
		}
	})

	return nil
}
