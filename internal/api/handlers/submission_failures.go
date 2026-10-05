package handlers

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// SubmissionFailure is one dead-lettered submission shown to admin/pengawas (audit H1).
type SubmissionFailure struct {
	ID           int     `json:"id"`
	NoID         string  `json:"no_id"`
	Nama         *string `json:"nama,omitempty"`
	ErrorMessage string  `json:"error_message"`
	SubmittedAt  *string `json:"submitted_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// GetSubmissionFailures lists dead-lettered submissions for the caller's tenant, newest
// first. A supervisor only sees peserta of their own room (user_id = ruang.id, as in
// GetRoomStatus); an admin sees the whole tenant.
func GetSubmissionFailures(c *fiber.Ctx) error {
	tenantID, _ := c.Locals("tenant_id").(int)

	query := `
		SELECT sf.id, sf.no_id, p.nama_peserta, sf.error_message,
		       CAST(sf.submitted_at AS TEXT), CAST(sf.created_at AS TEXT)
		FROM submission_failure sf
		LEFT JOIN peserta p ON p.no_id = sf.no_id AND p.tenant_id = sf.tenant_id AND p.deleted_at IS NULL
		WHERE sf.tenant_id = ?`
	args := []any{tenantID}
	scope, scopeArgs, err := resultScope(c)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Access denied")
	}
	query += scope
	args = append(args, scopeArgs...)
	query += ` ORDER BY sf.created_at DESC, sf.id DESC LIMIT 200`

	rows, err := db.DB.QueryContext(c.Context(), query, args...)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load submission failures")
	}
	defer rows.Close()

	list := []SubmissionFailure{}
	for rows.Next() {
		var f SubmissionFailure
		var nama, submittedAt, createdAt sql.NullString
		if err := rows.Scan(&f.ID, &f.NoID, &nama, &f.ErrorMessage, &submittedAt, &createdAt); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read submission failures")
		}
		if nama.Valid {
			f.Nama = &nama.String
		}
		if submittedAt.Valid {
			f.SubmittedAt = &submittedAt.String
		}
		f.CreatedAt = createdAt.String
		list = append(list, f)
	}
	if err := rows.Err(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to iterate submission failures")
	}
	return utils.SuccessResponse(c, list, "Submission failures retrieved")
}
