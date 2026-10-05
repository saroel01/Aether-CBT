package middleware

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// LoginRateLimitKey builds the login limiter key as tenant|account|IP (H3).
// Keying on IP alone locks out a whole computer lab behind one NAT address;
// adding the account keeps brute force against a single account bounded.
// The account is read from the JSON body (username for admin/supervisor,
// no_id for students); an unparsable body yields an empty account.
func LoginRateLimitKey(c *fiber.Ctx) string {
	tenant := ""
	if t := c.Locals("tenant_id"); t != nil {
		tenant = fmt.Sprint(t)
	}
	var body struct {
		Username string `json:"username"`
		NoID     string `json:"no_id"`
	}
	_ = json.Unmarshal(c.Body(), &body)
	account := body.Username
	if strings.TrimSpace(account) == "" {
		account = body.NoID
	}
	account = strings.ToLower(strings.TrimSpace(account))
	return tenant + "|" + account + "|" + c.IP()
}

// LoginIPRateLimitKey keys the second, looser login limiter on tenant|IP so one IP cannot
// spray passwords across unlimited accounts (review finding on H3).
func LoginIPRateLimitKey(c *fiber.Ctx) string {
	tenant := ""
	if t := c.Locals("tenant_id"); t != nil {
		tenant = fmt.Sprint(t)
	}
	return tenant + "|" + c.IP()
}
