package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// contentCookieForceSecure, when true, marks the content cookie Secure even on a plain
// HTTP request (e.g. behind a TLS-terminating proxy). Defaults to following the request
// scheme (Requirement 8, AD-2).
var contentCookieForceSecure bool

// SetContentCookieSecure forces the content-session cookie to be Secure.
func SetContentCookieSecure(force bool) { contentCookieForceSecure = force }

// setContentCookie writes the same-origin content-session cookie (AD-2).
func setContentCookie(c *fiber.Ctx, contentToken string) {
	c.Cookie(&fiber.Cookie{
		Name:     "aether_exam",
		Value:    contentToken,
		Path:     "/api/exam/content",
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteStrictMode,
		Secure:   contentCookieForceSecure || c.Secure(),
	})
}

// resolvedSession is the outcome of matching a login token to an exam session.
type resolvedSession struct {
	session      *models.ExamSession
	notEnterable string // human reason when a session matched but cannot be entered now
	legacy       bool   // true when no session matched; use the legacy global token
}

// resolveSessionForToken matches the token to an enterable session. If a session matches
// but none is currently enterable, notEnterable explains why (Requirement 6.3). If no
// session matches, legacy=true so the caller falls back to settings.token (Requirement 6.6).
func resolveSessionForToken(tenantID int, token string) resolvedSession {
	svc := newSchedulingService()
	sessions, err := repository.NewExamSessionRepository(db.DB).FindByToken(tenantID, token)
	if err != nil || len(sessions) == 0 {
		return resolvedSession{legacy: true}
	}
	for i := range sessions {
		if svc.EffectiveEnterable(&sessions[i]) {
			return resolvedSession{session: &sessions[i]}
		}
	}
	// Matched but none enterable: report why (prefer the clearest reason).
	reason := "session is not active"
	for i := range sessions {
		if r := svc.NotEnterableReason(&sessions[i]); r != "" {
			reason = r
			if r == "session has ended" {
				break
			}
		}
	}
	return resolvedSession{notEnterable: reason}
}

// MySessions returns the sessions the student may enter now (Requirement 5.3, 6.4).
func MySessions(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	pesertaID := c.Locals("user_id").(int)

	sessions, err := repository.NewExamSessionRepository(db.DB).SessionsForPeserta(tenantID, pesertaID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to list sessions")
	}
	svc := newSchedulingService()
	type sessionItem struct {
		models.ExamSession
		Enterable bool `json:"enterable"`
	}
	out := make([]sessionItem, 0, len(sessions))
	for i := range sessions {
		out = append(out, sessionItem{ExamSession: sessions[i], Enterable: svc.EffectiveEnterable(&sessions[i])})
	}
	return utils.SuccessResponse(c, out, "Sessions retrieved")
}
