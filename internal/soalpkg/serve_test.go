package soalpkg

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePath_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{"../../etc/passwd", "data/../../../secret", ".."} {
		if _, err := ResolvePath(dir, rel); err != ErrPathTraversal {
			t.Errorf("ResolvePath(%q) = %v, want ErrPathTraversal", rel, err)
		}
	}
}

func TestResolvePath_AcceptsValid(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{"index.html", "data/player.js", "data/sub/asset.css"} {
		if _, err := ResolvePath(dir, rel); err != nil {
			t.Errorf("ResolvePath(%q) = %v, want nil", rel, err)
		}
	}
}

func TestServeContent_StreamsAsset(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "player.js"), []byte("PLAYER_JS"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ServeContent(&buf, dir, "data/player.js"); err != nil {
		t.Fatalf("ServeContent: %v", err)
	}
	if buf.String() != "PLAYER_JS" {
		t.Errorf("served %q, want PLAYER_JS", buf.String())
	}
}

func TestServeContent_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	if err := ServeContent(io.Discard, dir, "../../etc/passwd"); err != ErrPathTraversal {
		t.Errorf("ServeContent traversal: got %v, want ErrPathTraversal", err)
	}
}

func TestServeIndexWithShim_InjectsAfterHeadAndLeavesDiskUnchanged(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	original := []byte(`<!DOCTYPE html><html><head><title>Q</title></head><body><script src="data/player.js"></script></body></html>`)
	if err := os.WriteFile(indexPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	ctx := ShimContext{Webhook: "/api/ispring/webhook", AttemptToken: "TOK-123", TenantID: "7", SID: "2026001"}
	if err := ServeIndexWithShim(&buf, dir, "index.html", ctx); err != nil {
		t.Fatalf("ServeIndexWithShim: %v", err)
	}
	served := buf.String()
	if !strings.Contains(served, "window.__AETHER__=") {
		t.Error("served index missing window.__AETHER__")
	}
	if !strings.Contains(served, `"attemptToken":"TOK-123"`) {
		t.Error("served index missing attempt token context")
	}
	// The shim block must appear right after <head>, before <title>.
	idxHead := strings.Index(served, "<head>")
	idxAether := strings.Index(served, "window.__AETHER__")
	idxTitle := strings.Index(served, "<title>")
	if idxHead < 0 || idxAether < 0 || idxTitle < 0 || idxAether < idxHead || idxAether > idxTitle {
		t.Errorf("shim must be injected between <head> and <title> (head=%d aether=%d title=%d)", idxHead, idxAether, idxTitle)
	}

	// Disk file unchanged (Property 12).
	onDisk, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, original) {
		t.Error("index.html on disk was modified by serving")
	}
}

func TestServeIndexWithShim_NoHeadPrepends(t *testing.T) {
	dir := t.TempDir()
	original := []byte("<html><body>no head</body></html>")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), original, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ServeIndexWithShim(&buf, dir, "index.html", ShimContext{Webhook: "/wh"}); err != nil {
		t.Fatalf("ServeIndexWithShim: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "<script>") {
		t.Error("expected shim prepended when no <head> present")
	}
}
