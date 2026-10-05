package soalpkg

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// zipEntry is an ordered entry for buildZipEntries; a nil content with a trailing-slash
// name produces a directory entry.
type zipEntry struct {
	name    string
	content string
	mode    os.FileMode // 0 = default
}

func buildZipEntries(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", e.name, err)
		}
		if e.content != "" {
			if _, err := io.WriteString(w, e.content); err != nil {
				t.Fatalf("write zip entry %q: %v", e.name, err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func storedFile(t *testing.T, baseDir string, res *StoreResult, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(baseDir, "default", res.PackageUUID, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("expected %s in package: %v", rel, err)
	}
	return string(b)
}

// Audit H2: directory entries used to be rejected by the IsRegular check.
func TestStore_AcceptsDirectoryEntries(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZipEntries(t, []zipEntry{
		{name: "data/"},
		{name: "data/fonts/"},
		{name: "index.html", content: plainIndexHTML()},
		{name: "data/a.js", content: "js"},
	})
	res, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if got := storedFile(t, baseDir, res, "data/a.js"); got != "js" {
		t.Fatalf("data/a.js = %q", got)
	}
	if fi, err := os.Stat(filepath.Join(baseDir, "default", res.PackageUUID, "data", "fonts")); err != nil || !fi.IsDir() {
		t.Fatalf("data/fonts dir missing: %v", err)
	}
}

// Audit H2: Windows Compress-Archive writes backslash separators ("data\player.js").
func TestStore_NormalizesBackslashNames(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZipEntries(t, []zipEntry{
		{name: "index.html", content: plainIndexHTML()},
		{name: `data\a.js`, content: "js"},
		{name: `data\fonts\f.woff`, content: "font"},
	})
	res, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if got := storedFile(t, baseDir, res, "data/a.js"); got != "js" {
		t.Fatalf("data/a.js = %q", got)
	}
	if got := storedFile(t, baseDir, res, "data/fonts/f.woff"); got != "font" {
		t.Fatalf("data/fonts/f.woff = %q", got)
	}
}

func TestStore_BackslashIndexAtRootIsAccepted(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZipEntries(t, []zipEntry{{name: `.\index.html`, content: plainIndexHTML()}})
	if _, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default")); err != nil {
		t.Fatalf("Store: %v", err)
	}
}

func TestStore_RejectsBackslashZipSlip(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZipEntries(t, []zipEntry{
		{name: "index.html", content: plainIndexHTML()},
		{name: `..\evil.txt`, content: "pwned"},
	})
	_, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default"))
	if !errors.Is(err, ErrZipSlip) {
		t.Fatalf("err = %v, want ErrZipSlip", err)
	}
	if _, statErr := os.Stat(filepath.Join(baseDir, "evil.txt")); statErr == nil {
		t.Fatal("zip-slip entry was written outside the package dir")
	}
	assertNoPackageLeft(t, baseDir)
}

func TestStore_RejectsSymlinkEntry(t *testing.T) {
	baseDir := t.TempDir()
	zipBytes := buildZipEntries(t, []zipEntry{
		{name: "index.html", content: plainIndexHTML()},
		{name: "link", content: "/etc/passwd", mode: os.ModeSymlink | 0o777},
	})
	if _, err := Store(bytes.NewReader(zipBytes), baseDir, defaultOpts("default")); err == nil {
		t.Fatal("expected symlink entry to be rejected")
	}
	assertNoPackageLeft(t, baseDir)
}

func assertNoPackageLeft(t *testing.T, baseDir string) {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(baseDir, "default"))
	if len(entries) != 0 {
		t.Fatalf("partial package left on disk: %d entries", len(entries))
	}
}

// Audit C1/D1: the answer key is extracted at upload time; failures do not fail the upload.
func TestStore_ExtractsAnswerKey(t *testing.T) {
	data := `{"d":{"sl":{"g":[{"s":{"st":"allQuestions"},"S":[{"i":"q1","tp":"TrueFalse","D":{"d":["Q"]},"s":{"e":{"t":"byQuestion","pt":5}},"C":{"chs":[{"c":true,"t":{"d":["True"]}},{"c":false,"t":{"d":["False"]}}]}}]}]}}}`
	index := `<html><script>var data = "` + base64.StdEncoding.EncodeToString([]byte(data)) + `";</script></html>`
	baseDir := t.TempDir()
	res, err := Store(bytes.NewReader(buildZip(t, map[string]string{"index.html": index})), baseDir, defaultOpts("default"))
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if res.AnswerKeyStatus != "full" || res.AnswerKeyJSON == "" {
		t.Fatalf("answer key status=%q json=%d bytes", res.AnswerKeyStatus, len(res.AnswerKeyJSON))
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(res.AnswerKeyJSON), &decoded); err != nil || decoded["max_score"] != 5.0 {
		t.Fatalf("answer key JSON invalid: %v %v", err, decoded["max_score"])
	}

	res, err = Store(bytes.NewReader(buildZip(t, map[string]string{"index.html": plainIndexHTML()})), baseDir, defaultOpts("default"))
	if err != nil {
		t.Fatalf("Store without player data: %v", err)
	}
	if res.AnswerKeyStatus != "none" || res.AnswerKeyJSON != "" {
		t.Fatalf("no-data package: status=%q json=%q", res.AnswerKeyStatus, res.AnswerKeyJSON)
	}
}

// Optional end-to-end check with the real Compress-Archive sample (gitignored).
func TestStore_SampleInformatikaZip(t *testing.T) {
	f, err := os.Open("../../contoh_soal/informatika-x-w.zip")
	if err != nil {
		t.Skipf("sample zip not available: %v", err)
	}
	defer f.Close()
	baseDir := t.TempDir()
	res, err := Store(f, baseDir, StoreOptions{TenantSlug: "default", MaxBytes: 100 * 1024 * 1024, MaxFiles: 5000})
	if err != nil {
		t.Fatalf("Store sample: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "default", res.PackageUUID, "data", "player.js")); err != nil {
		t.Fatalf("data/player.js not extracted into subfolder: %v", err)
	}
	if res.AnswerKeyStatus != "full" {
		t.Fatalf("sample answer key status = %q, want full", res.AnswerKeyStatus)
	}
}
