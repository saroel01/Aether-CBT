package soalpkg

import (
	_ "embed"
	"encoding/json"
)

// shimScript is the client-side result-submission shim, embedded at build time so the
// binary is self-contained (Requirement 9.1-9.5).
//
//go:embed assets/ispring-shim.js
var shimScript []byte

// ShimContext carries the per-session values the shim appends to redirected result
// submissions. The server populates it when serving index.html (content_session_service,
// Requirement 9.2).
type ShimContext struct {
	Webhook      string // relative same-origin URL, e.g. "/api/ispring/webhook"
	AttemptToken string
	TenantID     string
	SID          string // student no_id
}

// InjectionHTML returns the script block written into the served index.html: it exposes
// window.__AETHER__ with the session context and then runs the shim. The package file on
// disk is never modified - this block appears only in the HTTP response (Property 12).
func InjectionHTML(ctx ShimContext) string {
	return "<script>window.__AETHER__=" + aetherJSON(ctx) + ";" + string(shimScript) + "</script>"
}

// aetherJSON marshals the context safely (escaping any special characters in the values).
func aetherJSON(ctx ShimContext) string {
	b, err := json.Marshal(map[string]string{
		"webhook":      ctx.Webhook,
		"attemptToken": ctx.AttemptToken,
		"tenantId":     ctx.TenantID,
		"sid":          ctx.SID,
	})
	if err != nil {
		// Marshal of a string map cannot fail in practice; fall back to empty context.
		return "{}"
	}
	return string(b)
}
