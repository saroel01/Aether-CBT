package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetTokenQRCode generates and returns a QR Code PNG image for a text parameter. The text
// length is capped to bound the cost of QR generation and deter abuse of the public endpoint
// (review F14, Task 28).
func GetTokenQRCode(c *fiber.Ctx) error {
	text := c.Query("text", "")
	if text == "" || len(text) > 256 {
		return c.Status(fiber.StatusBadRequest).SendString("invalid text")
	}

	pngBytes, err := utils.GenerateQRCode(text, 256)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate QR Code")
	}

	c.Set("Content-Type", "image/png")
	return c.Send(pngBytes)
}
