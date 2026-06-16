package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/submission"
	"github.com/saroel01/aether-cbt/internal/utils"
)

func GetQueueStatus(q submission.Queue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := q.GetStats(c.Context())
		if err != nil {
			// Log the detail server-side; return a generic message so internal paths never
			// leak to the client (review F13, Task 28).
			log.Printf("[debug_queue] status error: %v", err)
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read queue status")
		}
		return utils.SuccessResponse(c, stats, "Queue status retrieved")
	}
}
