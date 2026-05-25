package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/db"
)

func TestGetSupervisorSettingsReturnsTokenForAuthorizedExamStaff(t *testing.T) {
	setupSupervisorSettingsTestDB(t)

	app := fiber.New()
	app.Get("/supervisor/settings", func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "supervisor")
		return GetSupervisorSettings(c)
	})

	req := httptest.NewRequest("GET", "/supervisor/settings", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			ExamTitle    string `json:"exam_title"`
			Token        string `json:"token"`
			IsExamActive bool   `json:"is_exam_active"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response")
	}
	if body.Data.Token != "ROOM2026" {
		t.Fatalf("expected supervisor token ROOM2026, got %q", body.Data.Token)
	}
	if body.Data.ExamTitle != "Ujian Ruang Aman" {
		t.Fatalf("expected exam title from settings, got %q", body.Data.ExamTitle)
	}
	if !body.Data.IsExamActive {
		t.Fatalf("expected active exam flag")
	}
}

func TestGetSupervisorSettingsRejectsStudentRole(t *testing.T) {
	setupSupervisorSettingsTestDB(t)

	app := fiber.New()
	app.Get("/supervisor/settings", func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "student")
		return GetSupervisorSettings(c)
	})

	req := httptest.NewRequest("GET", "/supervisor/settings", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func setupSupervisorSettingsTestDB(t *testing.T) {
	t.Helper()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	db.DB = database
	t.Cleanup(func() {
		_ = database.Close()
	})

	_, err = db.DB.Exec(`
		CREATE TABLE settings (
			tenant_id INTEGER PRIMARY KEY,
			exam_title TEXT NOT NULL,
			proctor_name TEXT,
			footer_text TEXT,
			token TEXT NOT NULL,
			is_exam_active BOOLEAN DEFAULT TRUE
		);
	`)
	if err != nil {
		t.Fatalf("create settings table: %v", err)
	}

	_, err = db.DB.Exec(`
		INSERT INTO settings (tenant_id, exam_title, proctor_name, footer_text, token, is_exam_active)
		VALUES (1, 'Ujian Ruang Aman', 'Pengawas', 'Aether CBT', 'ROOM2026', TRUE)
	`)
	if err != nil {
		t.Fatalf("insert settings: %v", err)
	}
}
