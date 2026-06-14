package soalpkg

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildZip builds an in-memory ZIP from a map of entry name -> content. Directories are
// inferred from path separators in entry names.
func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

const versionedIndexHTML = `<!DOCTYPE html>
<!-- Created with iSpring --><!--version 11.9.0.4 --><!--type html -->
<html><head></head><body><script src="data/player.js"></script></body></html>
`

func plainIndexHTML() string {
	return `<!DOCTYPE html><html><head></head><body>hello</body></html>`
}

func defaultOpts(slug string) StoreOptions {
	return StoreOptions{TenantSlug: slug, MaxBytes: 10 * 1024 * 1024, MaxFiles: 5000}
}

func TestStore_ValidPackageExtractsAndDetectsVersion(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZip(t, map[string]string{
		"index.html":         versionedIndexHTML,
		"data/player.js":     "console.log('player');",
		"data/asset.css":     "body{}",
	})

	res, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if res.PackageUUID == "" {
		t.Error("expected non-empty PackageUUID")
	}
	if res.EntryPath != "index.html" {
		t.Errorf("EntryPath = %q, want index.html", res.EntryPath)
	}
	if res.IspringVersion == nil || *res.IspringVersion != "11.9.0.4" {
		t.Errorf("IspringVersion = %v, want 11.9.0.4", res.IspringVersion)
	}
	if res.TotalSize != int64(len(zipBytes)) {
		t.Errorf("TotalSize = %d, want %d", res.TotalSize, len(zipBytes))
	}
	if res.Checksum == "" {
		t.Error("expected non-empty checksum")
	}

	// Files extracted under baseDir/{slug}/{uuid}/.
	extracted := filepath.Join(baseDir, "default", res.PackageUUID)
	for _, rel := range []string{"index.html", "data/player.js", "data/asset.css"} {
		if _, err := os.Stat(filepath.Join(extracted, rel)); err != nil {
			t.Errorf("expected extracted file %q: %v", rel, err)
		}
	}
}

func TestStore_VersionBestEffortWhenAbsent(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZip(t, map[string]string{"index.html": plainIndexHTML()})
	res, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default"))
	if err != nil {
		t.Fatalf("Store without version marker should still succeed: %v", err)
	}
	if res.IspringVersion != nil {
		t.Errorf("IspringVersion = %v, want nil when no marker", res.IspringVersion)
	}
}

func TestStore_RejectsNonZip(t *testing.T) {
	baseDir := t.TempDir()
	_, err := Store(strings.NewReader("this is not a zip"), baseDir, defaultOpts("default"))
	if err == nil {
		t.Fatal("expected error for non-ZIP input, got nil")
	}
	// No partial directory left behind (Property 3).
	matches, _ := filepath.Glob(filepath.Join(baseDir, "default", "*"))
	if len(matches) != 0 {
		t.Errorf("expected cleanup after failure, left %v", matches)
	}
}

func TestStore_RejectsMissingIndexHtml(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZip(t, map[string]string{"data/only.js": "x"})
	_, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default"))
	if err == nil {
		t.Fatal("expected error for ZIP without index.html, got nil")
	}
	matches, _ := filepath.Glob(filepath.Join(baseDir, "default", "*"))
	if len(matches) != 0 {
		t.Errorf("expected cleanup after failure, left %v", matches)
	}
}

func TestStore_RejectsZipSlipAndCleansUp(t *testing.T) {
	baseDir := t.TempDir()
	// A malicious entry that resolves outside the package directory.
	zipBytes := buildZip(t, map[string]string{
		"index.html":       plainIndexHTML(),
		"../escape.txt":    "pwned",
		"data/legit.js":    "ok",
	})
	_, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default"))
	if err == nil {
		t.Fatal("expected error for zip-slip entry, got nil")
	}
	// No partial package on disk (Property 3)...
	matches, _ := filepath.Glob(filepath.Join(baseDir, "default", "*"))
	if len(matches) != 0 {
		t.Errorf("expected cleanup after zip-slip, left %v", matches)
	}
	// ...and the escape file was NOT written outside baseDir.
	if _, err := os.Stat(filepath.Join(baseDir, "escape.txt")); err == nil {
		t.Error("zip-slip wrote a file outside the package directory")
	}
}

// incompressible produces n high-entropy bytes so the resulting ZIP does not shrink
// below the raw size (repeated "a" compresses to almost nothing, defeating size checks).
func incompressible(n int) string {
	b := make([]byte, n)
	var s uint64 = 0x1234567890abcdef
	for i := 0; i < n; i++ {
		s ^= s << 13
		s ^= s >> 7
		s ^= s << 17
		b[i] = byte(s)
	}
	return string(b)
}

func TestStore_RejectsOversizedArchive(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZip(t, map[string]string{"index.html": incompressible(5000)})
	_, err := Store(bytes.NewReader(zipBytes), baseDir, StoreOptions{TenantSlug: "default", MaxBytes: 1024, MaxFiles: 5000})
	if err == nil {
		t.Fatal("expected error for oversized archive, got nil")
	}
	matches, _ := filepath.Glob(filepath.Join(baseDir, "default", "*"))
	if len(matches) != 0 {
		t.Errorf("expected cleanup after oversize, left %v", matches)
	}
}

func TestStore_RejectsTooManyFiles(t *testing.T) {
	baseDir := t.TempDir()
	files := map[string]string{"index.html": plainIndexHTML()}
	for i := 0; i < 3; i++ {
		files["data/f"+string(rune('a'+i))+".js"] = "x"
	}
	zipBytes := buildZip(t, files)
	_, err := Store(bytes.NewReader(zipBytes), baseDir, StoreOptions{TenantSlug: "default", MaxBytes: 10 * 1024 * 1024, MaxFiles: 2})
	if err == nil {
		t.Fatal("expected error for too many files, got nil")
	}
	matches, _ := filepath.Glob(filepath.Join(baseDir, "default", "*"))
	if len(matches) != 0 {
		t.Errorf("expected cleanup after too-many-files, left %v", matches)
	}
}

func TestStore_TenantIsolationBySlug(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZip(t, map[string]string{"index.html": plainIndexHTML()})

	a, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("school-a"))
	if err != nil {
		t.Fatalf("Store A: %v", err)
	}
	b, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("school-b"))
	if err != nil {
		t.Fatalf("Store B: %v", err)
	}
	// Distinct tenant directories and distinct package UUIDs (Property 2).
	if filepath.Join(baseDir, "school-a", a.PackageUUID) == filepath.Join(baseDir, "school-b", b.PackageUUID) {
		t.Error("tenant packages must be isolated under distinct paths")
	}
	if _, err := os.Stat(filepath.Join(baseDir, "school-a", a.PackageUUID, "index.html")); err != nil {
		t.Errorf("school-a package missing index.html: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "school-b", b.PackageUUID, "index.html")); err != nil {
		t.Errorf("school-b package missing index.html: %v", err)
	}
}
