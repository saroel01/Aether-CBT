package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/saroel01/aether-cbt/internal/submission"
)

func GetQueueStatus(q submission.Queue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := q.GetStats(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"success": true,
			"data":    stats,
			"message": "Queue status retrieved",
		})
	}
}
