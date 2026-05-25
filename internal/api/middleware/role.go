package middleware

import "github.com/gofiber/fiber/v2"

func RequireRoles(allowedRoles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		allowed[role] = struct{}{}
	}

	return func(c *fiber.Ctx) error {
		roleValue := c.Locals("role")
		role, ok := roleValue.(string)
		if !ok || role == "" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   "Insufficient permissions",
			})
		}

		if _, ok := allowed[role]; !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   "Insufficient permissions",
			})
		}

		return c.Next()
	}
}
