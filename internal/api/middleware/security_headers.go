package middleware

import "github.com/gofiber/fiber/v2"

// SecurityHeaders sets baseline browser-security response headers on every API response
// (review security finding #14, Task 41). X-Frame-Options / frame-ancestors keep the app from
// being framed by unrelated origins; the iSpring exam iframe is same-origin and unaffected.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "SAMEORIGIN")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// L2: script-src is deliberately not restricted: the SvelteKit static adapter emits
		// inline bootstrap scripts and the iSpring player relies on inline script/eval.
		c.Set("Content-Security-Policy", "frame-ancestors 'self'; object-src 'none'; base-uri 'self'")
		if c.Protocol() == "https" {
			c.Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		}
		return c.Next()
	}
}
