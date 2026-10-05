package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// globalBodyLimit is the transport-level cap for every request that is fully buffered (audit
// M2). Package uploads exceed it but are streamed (StreamRequestBody) and read by FormFile
// from the stream, so they are never held in memory as a whole; bodyLimitGuard caps them at
// SoalUploadMaxBytes instead.
const globalBodyLimit = 4 << 20

const (
	webhookBodyLimit = 1 << 20
	defaultBodyLimit = 2 << 20
)

// newFiberConfig builds the server config: small buffered body limit, streamed bodies for
// uploads, read/write/idle timeouts (M2) and the TRUSTED_PROXIES handling (H3).
func newFiberConfig() fiber.Config {
	cfg := fiber.Config{
		AppName:                      "Aether CBT v1.0",
		BodyLimit:                    globalBodyLimit,
		StreamRequestBody:            true,
		DisablePreParseMultipartForm: true,
		ReadTimeout:                  60 * time.Second,
		// fasthttp applies one write deadline per response, so this bounds the whole transfer
		// (large media under /api/exam/content/, big exports on a congested lab network).
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}
	// H3: behind a reverse proxy (Coolify/Traefik) c.IP() is the proxy's address unless
	// X-Forwarded-For is trusted. Only trust it from the configured proxy ranges.
	if proxies := proxyConfig(os.Getenv("TRUSTED_PROXIES")); len(proxies) > 0 {
		cfg.ProxyHeader = fiber.HeaderXForwardedFor
		cfg.EnableTrustedProxyCheck = true
		cfg.TrustedProxies = proxies
		cfg.EnableIPValidation = true
	}
	return cfg
}

// staticCacheControl sets Cache-Control on frontend responses after they are served (L11):
// SvelteKit's content-hashed /_app/immutable/ assets are cached for a year, every other
// non-API file (index.html, favicon, ...) must be revalidated so a deploy takes effect.
func staticCacheControl() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		p := c.Path()
		switch {
		case p == "/api" || strings.HasPrefix(p, "/api/"):
		case strings.HasPrefix(p, "/_app/immutable/") && c.Response().StatusCode() == fiber.StatusOK &&
			!strings.HasPrefix(string(c.Response().Header.ContentType()), fiber.MIMETextHTML):
			// Only a real hashed asset is immutable; a missing chunk falls through to the SPA
			// fallback (index.html) and must not be cached for a year.
			c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
		default:
			c.Set(fiber.HeaderCacheControl, "no-cache")
		}
		return err
	}
}

func isUploadPath(p string) bool {
	return strings.HasPrefix(p, "/api/admin/soal-packages/upload") || strings.HasPrefix(p, "/api/soal-packages/upload")
}

func payloadTooLarge(c *fiber.Ctx, msg string) error {
	// The unread (possibly streamed) body would otherwise be parsed as the next request on
	// a keep-alive connection; closing it also stops the client from uploading the rest.
	c.Context().SetConnectionClose()
	return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"status": "error", "message": msg})
}

// bodyLimitGuard enforces per-route body limits (P1-9, M2). Upload routes must declare a
// Content-Length (411 otherwise) no larger than uploadMax and are left streaming; the body
// is never buffered here. Everything else is capped at 1 MB (webhook) or 2 MB by
// Content-Length, or by a bounded read for chunked bodies.
func bodyLimitGuard(uploadMax int64) fiber.Handler {
	return func(c *fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}
		p := strings.ToLower(c.Path())
		cl := c.Request().Header.ContentLength()

		if isUploadPath(p) {
			if cl < 0 {
				return c.Status(fiber.StatusLengthRequired).JSON(fiber.Map{"status": "error", "message": "Content-Length wajib untuk unggah paket soal"})
			}
			if int64(cl) > uploadMax {
				return payloadTooLarge(c, fmt.Sprintf("Ukuran file paket soal melebihi batas maksimum (%d MB)", uploadMax/(1024*1024)))
			}
			return c.Next()
		}

		limit, msg := defaultBodyLimit, "Payload too large. Maximum body size is 2 MB."
		if strings.HasPrefix(p, "/api/ispring/webhook") {
			limit, msg = webhookBodyLimit, "Webhook payload exceeds maximum allowed size (1 MB)"
		}
		if cl > limit {
			return payloadTooLarge(c, msg)
		}
		if cl < 0 && c.Request().IsBodyStream() {
			// Chunked body of unknown size: buffer at most limit+1 bytes instead of letting
			// c.Body() drain an unbounded stream into memory.
			data, err := io.ReadAll(io.LimitReader(c.Context().RequestBodyStream(), int64(limit)+1))
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Failed to read request body"})
			}
			if len(data) > limit {
				return payloadTooLarge(c, msg)
			}
			c.Request().SetBody(data)
		}
		return c.Next()
	}
}
