package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetTokenQRCode generates and returns a QR Code PNG image for a text parameter
func GetTokenQRCode(c *fiber.Ctx) error {
	text := c.Query("text", "")
	if text == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing text parameter")
	}

	pngBytes, err := utils.GenerateQRCode(text, 256)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to generate QR Code")
	}

	c.Set("Content-Type", "image/png")
	return c.Send(pngBytes)
}
