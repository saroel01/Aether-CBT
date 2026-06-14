package soalpkg

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// StoreOptions controls package extraction limits and tenant placement.
type StoreOptions struct {
	TenantSlug string // isolates packages under baseDir/{tenant_slug}/{uuid}
	MaxBytes   int64  // maximum uploaded archive size in bytes (Req 3.2)
	MaxFiles   int    // maximum number of entries in the archive (anti zip-bomb, Req 3.2)
}

// StoreResult describes an extracted package.
type StoreResult struct {
	PackageUUID    string  // the uuid folder name under data/soal/{slug}/
	EntryPath      string  // relative entry HTML, always "index.html"
	IspringVersion *string // best-effort iSpring version from index.html (Req 3.6a), nil if unknown
	TotalSize      int64   // uploaded archive size in bytes
	Checksum       string  // sha256 hex of the uploaded archive (audit/dedup)
}

// Extraction errors. Handlers map them to the HTTP statuses described in the design's
// Error Handling section.
var (
	ErrNotZip       = errors.New("soalpkg: uploaded file is not a valid ZIP archive")
	ErrMissingIndex = errors.New("soalpkg: package has no index.html at its root")
	ErrTooLarge     = errors.New("soalpkg: package exceeds the configured size limit")
	ErrTooManyFiles = errors.New("soalpkg: package exceeds the configured file-count limit")
	ErrZipSlip      = errors.New("soalpkg: package entry escapes the package directory (zip-slip)")
)

// decompressedBombFactor caps total decompressed size as a multiple of the archive limit,
// defending against zip bombs (a small archive that decompresses enormously).
const decompressedBombFactor = 10

// versionRe matches the iSpring version comment emitted by QuizMaker exports, e.g.
// `<!--version 11.9.0.4 -->`. Best-effort: absence is not an error (Req 3.6a).
var versionRe = regexp.MustCompile(`(?i)<!--version\s+([^>\s]+)\s*-->`)

// Store reads an uploaded iSpring ZIP, validates it, and extracts it under
// baseDir/{tenant_slug}/{uuid}/. It enforces size/count limits, rejects zip-slip and
// archives without a root index.html, computes a checksum, and detects the iSpring
// version best-effort. On any failure it removes the partial package so no corrupt
// package is left on disk (Requirements 3.1-3.7, 15.3; Properties 2, 3).
func Store(r io.Reader, baseDir string, opts StoreOptions) (*StoreResult, error) {
	// Bound the upload and hash it as we read (anti-oversize, Req 3.2).
	limited := io.LimitReader(r, opts.MaxBytes+1)
	var buf bytes.Buffer
	n, err := io.Copy(&buf, limited)
	if err != nil {
		return nil, fmt.Errorf("soalpkg: read upload: %w", err)
	}
	if n > opts.MaxBytes {
		return nil, ErrTooLarge
	}
	archiveBytes := buf.Bytes()
	sum := sha256.Sum256(archiveBytes)
	checksum := hex.EncodeToString(sum[:])

	zipReader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, ErrNotZip
	}

	if opts.MaxFiles > 0 && len(zipReader.File) > opts.MaxFiles {
		return nil, ErrTooManyFiles
	}
	if !hasRootIndex(zipReader) {
		return nil, ErrMissingIndex
	}
	if decompressedSize(zipReader) > opts.MaxBytes*decompressedBombFactor {
		return nil, ErrTooLarge
	}

	version := detectVersion(zipReader)

	packageUUID := uuid.NewString()
	destDir := filepath.Join(baseDir, opts.TenantSlug, packageUUID)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("soalpkg: create package dir: %w", err)
	}
	// Any failure after the directory exists triggers cleanup (Property 3).
	if err := extractZip(zipReader, destDir); err != nil {
		_ = os.RemoveAll(destDir)
		return nil, err
	}

	return &StoreResult{
		PackageUUID:    packageUUID,
		EntryPath:      "index.html",
		IspringVersion: version,
		TotalSize:      int64(len(archiveBytes)),
		Checksum:       checksum,
	}, nil
}

func hasRootIndex(zr *zip.Reader) bool {
	for _, f := range zr.File {
		if f.Name == "index.html" || f.Name == "./index.html" {
			return true
		}
	}
	return false
}

func decompressedSize(zr *zip.Reader) int64 {
	var total int64
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			total += int64(f.UncompressedSize64)
		}
	}
	return total
}

// detectVersion reads the head of index.html and returns the iSpring version if the
// marker comment is present, else nil. Never errors (best-effort, Req 3.6a).
func detectVersion(zr *zip.Reader) *string {
	for _, f := range zr.File {
		if f.Name != "index.html" && f.Name != "./index.html" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		head := make([]byte, 4096)
		n, _ := io.ReadFull(rc, head)
		_ = rc.Close()
		m := versionRe.FindSubmatch(head[:n])
		if m == nil {
			return nil
		}
		v := string(m[1])
		return &v
	}
	return nil
}

// extractZip writes every entry under destDir, rejecting any entry that resolves outside
// it (anti zip-slip, Req 3.3).
func extractZip(zr *zip.Reader, destDir string) error {
	cleanDest := filepath.Clean(destDir)
	for _, f := range zr.File {
		target := filepath.Join(destDir, f.Name)
		if !isWithin(cleanDest, target) {
			return fmt.Errorf("%w: %s", ErrZipSlip, f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := copyZipEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func copyZipEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// isWithin reports whether target resolves inside baseDir. It is the anti-traversal guard
// for both extraction (zip-slip) and serving.
func isWithin(baseDir, target string) bool {
	rel, err := filepath.Rel(baseDir, filepath.Clean(target))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// RemovePackage deletes a package directory from disk (Requirement 3.10). It is a no-op
// (returns nil) when the directory is already gone, so callers can invoke it
// unconditionally after removing the metadata row.
func RemovePackage(baseDir, tenantSlug, packageUUID string) error {
	return os.RemoveAll(filepath.Join(baseDir, tenantSlug, packageUUID))
}
