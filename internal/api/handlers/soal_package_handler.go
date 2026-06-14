package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/soalpkg"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// soalUploadLimits holds the upload caps injected at startup (Requirement 3.2, 10.6).
var soalUploadLimits = struct {
	MaxBytes int64
	MaxFiles int
}{
	MaxBytes: 100 * 1024 * 1024,
	MaxFiles: 5000,
}

// SetSoalUploadLimits configures the upload caps from the loaded configuration.
func SetSoalUploadLimits(maxBytes int64, maxFiles int) {
	soalUploadLimits.MaxBytes = maxBytes
	soalUploadLimits.MaxFiles = maxFiles
}

// soalStorageDir is the on-disk root for extracted packages (data/soal). It is a package
// variable so tests can redirect it to a temp directory.
var soalStorageDir = "data/soal"

// SetSoalStorageDir overrides the package storage root (primarily for tests).
func SetSoalStorageDir(dir string) { soalStorageDir = dir }

// UploadSoalPackage accepts a ZIP multipart upload, stores it safely, and records its
// metadata (Requirement 3.1, 3.8).
func UploadSoalPackage(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	uploadedBy, _ := c.Locals("user_id").(int)

	file, err := c.FormFile("file")
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Missing 'file' field in upload")
	}
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Only .zip packages are accepted")
	}

	slug, err := tenantSlug(tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to resolve tenant")
	}

	src, err := file.Open()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Failed to read uploaded file")
	}
	defer src.Close()

	res, err := soalpkg.Store(src, soalStorageDir, soalpkg.StoreOptions{
		TenantSlug: slug,
		MaxBytes:   soalUploadLimits.MaxBytes,
		MaxFiles:   soalUploadLimits.MaxFiles,
	})
	if err != nil {
		if code, msg, ok := soalStoreErrorToHTTP(err); ok {
			return utils.ErrorResponse(c, code, msg)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to store package")
	}

	nama := strings.TrimSuffix(file.Filename, strings.ToLower(".zip"))
	if nama == "" {
		nama = res.PackageUUID
	}
	checksum := res.Checksum
	pkg, err := repository.NewSoalPackageRepository(db.DB).Create(tenantID, repository.SoalPackageInput{
		Nama:           nama,
		PackageUUID:    res.PackageUUID,
		EntryPath:      res.EntryPath,
		IspringVersion: res.IspringVersion,
		TotalSize:      res.TotalSize,
		Checksum:       &checksum,
		UploadedBy:     &uploadedBy,
	})
	if err != nil {
		// Best-effort cleanup of orphaned files if metadata insert fails (Property 3).
		_ = soalpkg.RemovePackage(soalStorageDir, slug, res.PackageUUID)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to record package metadata")
	}
	return utils.SuccessResponse(c, pkg, "Package uploaded")
}

// ListSoalPackages returns the tenant's packages (Requirement 3.9).
func ListSoalPackages(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	packages, err := repository.NewSoalPackageRepository(db.DB).List(tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to list packages")
	}
	return utils.SuccessResponse(c, packages, "Packages retrieved")
}

// DeleteSoalPackage removes an unlinked package's metadata and disk files (Requirement 3.10).
func DeleteSoalPackage(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid package id")
	}

	repo := repository.NewSoalPackageRepository(db.DB)
	pkg, err := repo.GetByID(tenantID, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Package not found")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to find package")
	}
	if err := repo.Delete(tenantID, id); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return utils.ErrorResponse(c, fiber.StatusConflict, "Package is linked to an exam; unlink it first")
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to delete package")
	}
	slug, _ := tenantSlug(tenantID)
	_ = soalpkg.RemovePackage(soalStorageDir, slug, pkg.PackageUUID)
	return utils.SuccessResponse(c, nil, "Package deleted")
}

// tenantSlug resolves the tenant's slug for filesystem isolation paths.
func tenantSlug(tenantID int) (string, error) {
	var slug string
	err := db.DB.QueryRow(`SELECT slug FROM tenants WHERE id = ?`, tenantID).Scan(&slug)
	return slug, err
}

// soalStoreErrorToHTTP maps soalpkg extraction errors to HTTP statuses (design Error Handling).
func soalStoreErrorToHTTP(err error) (int, string, bool) {
	switch {
	case errors.Is(err, soalpkg.ErrTooLarge):
		return fiber.StatusRequestEntityTooLarge, "Package exceeds the configured size limit", true
	case errors.Is(err, soalpkg.ErrNotZip):
		return fiber.StatusBadRequest, "Uploaded file is not a valid ZIP archive", true
	case errors.Is(err, soalpkg.ErrMissingIndex):
		return fiber.StatusBadRequest, "Package must contain index.html at its root", true
	case errors.Is(err, soalpkg.ErrTooManyFiles):
		return fiber.StatusBadRequest, "Package exceeds the configured file-count limit", true
	case errors.Is(err, soalpkg.ErrZipSlip):
		return fiber.StatusBadRequest, "Package contains an unsafe path entry (zip-slip)", true
	}
	return 0, "", false
}
