package middleware

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/saroel01/aether-cbt/internal/utils"
)

// AuthMiddleware validates JWT token
func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authorization header",
			})
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(utils.GetJWTSecret()), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		claims := token.Claims.(jwt.MapClaims)
		// Two-value type assertions so a token missing/mistyping a claim returns 401 instead
		// of panicking (review security finding #12, Task 40).
		userID, ok := claims["user_id"].(float64)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token claims"})
		}
		tenantID, ok := claims["tenant_id"].(float64)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token claims"})
		}
		role, ok := claims["role"].(string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token claims"})
		}
		// Tokens issued before claim "tv" existed count as version 0.
		tv := 0
		if v, ok := claims["tv"].(float64); ok {
			tv = int(v)
		}
		current, err := accountTokenVersion(c.UserContext(), role, int(tenantID), int(userID))
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, errUnknownRole) || (err == nil && current != tv) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Session revoked or account inactive"})
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to verify session"})
		}
		c.Locals("user_id", int(userID))
		c.Locals("tenant_id", int(tenantID))
		c.Locals("role", role)

		return c.Next()
	}
}
