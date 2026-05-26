package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/saroel01/aether-cbt/internal/submission"
)

func TestGetQueueStatusIncludesDoneCount(t *testing.T) {
	root := t.TempDir()
	q, err := submission.NewFilesystemQueue(root)
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	for dir, name := range map[string]string{
		"pending":    "p.json",
		"processing": "pr.json",
		"done":       "d.json",
		"failed":     "f.json",
	} {
		if err := os.WriteFile(filepath.Join(root, dir, name), []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", dir, err)
		}
	}

	app := fiber.New()
	app.Get("/debug/queue", GetQueueStatus(q))
	resp, err := app.Test(httptest.NewRequest("GET", "/debug/queue", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Data submission.QueueStats `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if body.Data.PendingCount != 1 || body.Data.ProcessingCount != 1 || body.Data.DoneCount != 1 || body.Data.FailedCount != 1 {
		t.Fatalf("stats = %+v, want all counts 1", body.Data)
	}
}

func TestGetQueueStatusReturns500WhenDirectoryUnreadable(t *testing.T) {
	root := t.TempDir()
	q, err := submission.NewFilesystemQueue(root)
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, "pending")); err != nil {
		t.Fatalf("RemoveAll pending: %v", err)
	}

	app := fiber.New()
	app.Get("/debug/queue", GetQueueStatus(q))
	resp, err := app.Test(httptest.NewRequest("GET", "/debug/queue", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("error body = %#v, want non-empty error", body)
	}
}
