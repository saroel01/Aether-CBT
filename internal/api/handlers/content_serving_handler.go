package handlers

import (
	"errors"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/service"
	"github.com/saroel01/aether-cbt/internal/soalpkg"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// newContentSessionService wires the content-session service over the global DB (cheap to
// construct per request).
func newContentSessionService() *service.ContentSessionService {
	return service.NewContentSessionService(db.DB)
}

// ServeExamContent serves an authorized iSpring package: it validates the content-session
// cookie, resolves the exam's package directory (scoped to the token's tenant), and streams
// the entry HTML with the result-submission shim injected, or a sub-asset verbatim
// (Requirements 8.1-8.6; AD-2, AD-3).
//
// The tenant is derived from the cookie token (not the request header): the iSpring player
// loads sub-assets via plain HTML tags that carry no Authorization or tenant header, so the
// token is the sole authority. It is an unguessable per-session capability, held 1:1 to a
// cek_login row by a unique index (migration 026), and the downstream session/exam/package
// lookups are all scoped to that row's tenant - so a holder of only their own token can
// never resolve another tenant's content. This route is therefore registered outside the
// Bearer AuthMiddleware group and exempted from TenantMiddleware's production header
// requirement.
func ServeExamContent(c *fiber.Ctx) error {
	contentToken := c.Cookies("aether_exam")
	if contentToken == "" {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Missing exam content session")
	}

	ctx, err := newContentSessionService().Authorize(contentToken)
	if err != nil {
		return mapContentError(c, err)
	}

	slug, err := tenantSlug(ctx.TenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to resolve tenant")
	}
	packageDir := filepath.Join(soalStorageDir, slug, ctx.PackageUUID)

	// Normalize the requested sub-path and decide entry vs. asset. Clean collapses any
	// "./" so the entry is detected regardless of how the player references it.
	relPath := c.Params("*")
	if relPath == "" {
		relPath = ctx.EntryPath
	}
	relPath = strings.TrimPrefix(relPath, "/")
	rel := filepath.Clean(relPath)
	entry := filepath.Clean(ctx.EntryPath)
	isEntry := rel == entry

	// Content is session-bound and must never be cached across students/attempts.
	c.Set("Cache-Control", "no-store")

	if isEntry {
		shimCtx := soalpkg.ShimContext{
			Webhook:      "/api/ispring/webhook",
			AttemptToken: ctx.AttemptToken,
			TenantID:     strconv.Itoa(ctx.TenantID),
			SID:          ctx.SID,
		}
		// Resolve + read the entry first so a missing/traversal error is reported before any
		// byte is written (ServeIndexWithShim reads the whole file then writes once).
		c.Set("Content-Type", "text/html; charset=utf-8")
		if err := soalpkg.ServeIndexWithShim(c.Context().Response.BodyWriter(), packageDir, ctx.EntryPath, shimCtx); err != nil {
			return mapServeError(c, err)
		}
		return nil
	}

	// Asset: ServeContent resolves the path (traversal -> ErrPathTraversal -> 400) and
	// streams the file (missing -> os.PathError -> 404); mapServeError maps both. One
	// resolve, one open - the handler does not duplicate that work (Requirement 8.6).
	c.Set("Content-Type", contentTypeFor(rel))
	if err := soalpkg.ServeContent(c.Context().Response.BodyWriter(), packageDir, rel); err != nil {
		return mapServeError(c, err)
	}
	return nil
}

// contentTypeFor returns the MIME type for a package asset. Common iSpring asset types are
// hard-coded (so behavior does not depend on the host MIME registry, e.g. Windows), with
// mime.TypeByExtension as a fallback and a safe default.
func contentTypeFor(relPath string) string {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".webp":
		return "image/webp"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".wasm":
		return "application/wasm"
	}
	if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(relPath))); t != "" {
		return t
	}
	return "application/octet-stream"
}

// mapContentError maps content-session authorization errors to HTTP responses (design Error
// Handling): invalid token -> 401, locked/closed window -> 403, missing package -> 404.
func mapContentError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrContentUnauthorized):
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Exam content session is invalid or expired")
	case errors.Is(err, service.ErrContentLocked):
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Session is locked; contact your supervisor")
	case errors.Is(err, service.ErrContentWindowClosed):
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Exam session is not currently active")
	case errors.Is(err, service.ErrContentPackageMissing):
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Exam content is not available")
	}
	return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to authorize content")
}

// mapServeError maps package-serving errors: traversal -> 400, missing file -> 404, else 500.
func mapServeError(c *fiber.Ctx, err error) error {
	if errors.Is(err, soalpkg.ErrPathTraversal) {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Unsafe content path")
	}
	if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Content not found")
	}
	return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to serve content")
}
