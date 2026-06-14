package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// SetClassTingkat sets the grade level (X/XI/XII) on a class (Requirement 1.1). Admin-only
// via route middleware; tenant-scoped via c.Locals.
func SetClassTingkat(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	kelasID, err := c.ParamsInt("id")
	if err != nil || kelasID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid class id")
	}

	var req struct {
		Tingkat string `json:"tingkat"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	repo := repository.NewGradeRepository(db.DB)
	if err := repo.SetTingkat(tenantID, kelasID, req.Tingkat); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Class not found")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to update grade level")
	}
	return utils.SuccessResponse(c, fiber.Map{"tingkat": req.Tingkat}, "Grade level updated")
}
