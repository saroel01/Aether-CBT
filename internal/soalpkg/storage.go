package soalpkg

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/saroel01/aether-cbt/internal/ispring"
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
	// AnswerKeyJSON is the serialized ispring.AnswerKey extracted from index.html ("" when
	// none); AnswerKeyStatus is full|partial|none (audit C1, D1). Server-only data.
	AnswerKeyJSON   string
	AnswerKeyStatus string
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

	keyJSON, keyStatus := extractAnswerKey(destDir)
	return &StoreResult{
		PackageUUID:     packageUUID,
		EntryPath:       "index.html",
		IspringVersion:  version,
		TotalSize:       int64(len(archiveBytes)),
		Checksum:        checksum,
		AnswerKeyJSON:   keyJSON,
		AnswerKeyStatus: keyStatus,
	}, nil
}

// extractAnswerKey reads the extracted index.html and serializes its iSpring answer key
// (audit C1, D1). Extraction is best-effort: any failure yields ("", "none") and never fails
// the upload, so packages without player data still upload and are scored client-side.
func extractAnswerKey(destDir string) (string, string) {
	html, err := os.ReadFile(filepath.Join(destDir, "index.html"))
	if err != nil {
		return "", ispring.AnswerKeyNone
	}
	key, err := ispring.ExtractAnswerKey(html)
	if err != nil || key.Status == ispring.AnswerKeyNone {
		return "", ispring.AnswerKeyNone
	}
	raw, err := json.Marshal(key)
	if err != nil {
		return "", ispring.AnswerKeyNone
	}
	return string(raw), key.Status
}

// entryName returns the archive entry name with backslashes normalised to forward
// slashes (Windows Compress-Archive writes "data\player.js", audit H2).
func entryName(f *zip.File) string {
	return strings.ReplaceAll(f.Name, "\\", "/")
}

// isRootIndex reports whether the entry is the package's root index.html.
func isRootIndex(f *zip.File) bool {
	name := entryName(f)
	return name == "index.html" || name == "./index.html"
}

func hasRootIndex(zr *zip.Reader) bool {
	for _, f := range zr.File {
		if isRootIndex(f) {
			return true
		}
	}
	return false
}

func decompressedSize(zr *zip.Reader) int64 {
	var total int64
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() && !strings.HasSuffix(entryName(f), "/") {
			total += int64(f.UncompressedSize64)
		}
	}
	return total
}

// detectVersion reads the head of index.html and returns the iSpring version if the
// marker comment is present, else nil. Never errors (best-effort, Req 3.6a).
func detectVersion(zr *zip.Reader) *string {
	for _, f := range zr.File {
		if !isRootIndex(f) {
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
		// Windows Compress-Archive (PowerShell 5.1) writes "data\player.js"; on Linux that
		// would become a flat file literally named "data\player.js" and break index.html
		// (audit H2). Normalise to forward slashes before any path handling.
		name := entryName(f)
		mode := f.FileInfo().Mode()
		// Reject symlinks so an archive cannot plant a link that escapes the package dir on
		// extraction (review iSpring F6, Task 30).
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to extract non-regular entry: %s", f.Name)
		}
		target := filepath.Join(destDir, filepath.FromSlash(name))
		if !isWithin(cleanDest, target) {
			return fmt.Errorf("%w: %s", ErrZipSlip, f.Name)
		}
		// Directory entries ("data/") are legitimate in most ZIP tools; they used to be
		// rejected by the IsRegular check below (audit H2).
		if f.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		// Reject remaining non-regular entries (devices, pipes, sockets).
		if !mode.IsRegular() {
			return fmt.Errorf("refusing to extract non-regular entry: %s", f.Name)
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

// perEntrySizeLimit caps the decompressed size of a single zip entry (256 MiB). A zip bomb
// inflates a tiny compressed entry into gigabytes; the cap aborts the copy before that
// exhausts disk/memory (review iSpring F6, Task 30).
const perEntrySizeLimit = 256 * 1024 * 1024

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
	n, err := io.Copy(out, io.LimitReader(rc, perEntrySizeLimit+1))
	if err != nil {
		return err
	}
	if n > perEntrySizeLimit {
		return fmt.Errorf("entry %s decompressed beyond %d-byte limit", f.Name, perEntrySizeLimit)
	}
	return nil
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
