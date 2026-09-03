package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// contentCookieSecureMode controls the Secure attribute of the content cookie:
// "auto" (default): inspects TLS scheme and X-Forwarded-Proto header
// "true": always Secure
// "false": never Secure (Requirement 2.14, 3.12)
var contentCookieSecureMode = "auto"

// SetContentCookieSecureMode configures the tri-state cookie secure mode.
func SetContentCookieSecureMode(mode string) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "true" || mode == "false" {
		contentCookieSecureMode = mode
	} else {
		contentCookieSecureMode = "auto"
	}
}

// SetContentCookieSecure forces the content-session cookie to be Secure (backward compat).
func SetContentCookieSecure(force bool) {
	if force {
		contentCookieSecureMode = "true"
	} else {
		contentCookieSecureMode = "false"
	}
}

func isContentCookieSecure(c *fiber.Ctx) bool {
	switch contentCookieSecureMode {
	case "true":
		return true
	case "false":
		return false
	default: // "auto"
		if c.Secure() {
			return true
		}
		if strings.EqualFold(c.Protocol(), "https") {
			return true
		}
		proto := c.Get("X-Forwarded-Proto")
		return strings.EqualFold(proto, "https")
	}
}

// setContentCookie writes the same-origin content-session cookie (AD-2).
func setContentCookie(c *fiber.Ctx, contentToken string) {
	c.Cookie(&fiber.Cookie{
		Name:     "aether_exam",
		Value:    contentToken,
		Path:     "/api/exam/content",
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteStrictMode,
		Secure:   isContentCookieSecure(c),
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
	// Clause 2.12: whitespace/empty token must never match any exam_session
	if strings.TrimSpace(token) == "" {
		return resolvedSession{legacy: true}
	}
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
