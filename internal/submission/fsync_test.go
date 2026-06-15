package submission

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteDurablePersistsAndSyncs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job.json")
	if err := writeDurable(path, []byte(`{"x":1}`)); err != nil {
		t.Fatalf("writeDurable: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != `{"x":1}` {
		t.Fatalf("content = %q", got)
	}
}

func TestWriteDurableOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job.json")
	if err := writeDurable(path, []byte(`{"v":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := writeDurable(path, []byte(`{"v":2}`)); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != `{"v":2}` {
		t.Fatalf("content = %q, want v:2", got)
	}
}

func TestWriteDurableCreatesParentDirIfNeeded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "job.json")
	if err := writeDurable(path, []byte(`{"x":1}`)); err != nil {
		t.Fatalf("writeDurable should create parent dirs: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != `{"x":1}` {
		t.Fatalf("content = %q", got)
	}
}

func TestSyncDirIsBestEffortAndNeverPanics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job.json")
	if err := writeDurable(path, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	// syncDir is documented as best-effort: on platforms that reject directory fsync
	// (notably Windows, where it returns "Access is denied") an error is expected and
	// the production caller ignores it. We only assert it does not panic.
	_ = syncDir(path)
}
