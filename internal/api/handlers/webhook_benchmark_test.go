package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/submission"
)

func BenchmarkISpringWebhookConcurrent500(b *testing.B) {
	var err error
	db.DB, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	db.DB.SetMaxOpenConns(1)
	defer db.DB.Close()
	for _, stmt := range []string{
		`CREATE TABLE peserta (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, no_id TEXT NOT NULL, password TEXT, nama_peserta TEXT, kelas_id INTEGER, ruang_id INTEGER, UNIQUE(tenant_id,no_id));`,
		`CREATE TABLE cek_login (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, peserta_id INTEGER NOT NULL, mapel_id INTEGER NOT NULL, attempt_token TEXT, login_time DATETIME DEFAULT CURRENT_TIMESTAMP, UNIQUE(tenant_id,peserta_id,mapel_id));`,
		`CREATE INDEX idx_peserta_no_id ON peserta(tenant_id,no_id);`,
	} {
		if _, err := db.DB.Exec(stmt); err != nil {
			b.Fatalf("schema: %v", err)
		}
	}
	tx, _ := db.DB.Begin()
	for i := 0; i < 500; i++ {
		noID := fmt.Sprintf("E2E%04d", i)
		res, err := tx.Exec(`INSERT INTO peserta (tenant_id,no_id,password,nama_peserta,kelas_id,ruang_id) VALUES (1,?,?,?,?,1)`, noID, "p", noID, 1)
		if err != nil {
			b.Fatalf("insert peserta: %v", err)
		}
		id, _ := res.LastInsertId()
		if _, err := tx.Exec(`INSERT INTO cek_login (tenant_id,peserta_id,mapel_id,attempt_token) VALUES (1,?,?,?)`, id, 7, fmt.Sprintf("tok%04d", i)); err != nil {
			b.Fatalf("insert cek_login: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit seed: %v", err)
	}

	queue, err := submission.NewFilesystemQueue(b.TempDir())
	if err != nil {
		b.Fatalf("queue: %v", err)
	}
	SetSubmissionQueue(queue)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/webhook", ISpringWebhook)

	xml := `<?xml version="1.0"?><quizReport version="1"><questions><multipleChoiceQuestion id="q1" evaluationEnabled="true" maxPoints="5" awardedPoints="5" status="correct"><direction><text>Q</text></direction><answers correctAnswerIndex="0" userAnswerIndex="0"><answer><text>A</text></answer></answers></multipleChoiceQuestion></questions></quizReport>`

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		for i := 0; i < 500; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				form := url.Values{}
				form.Set("sid", fmt.Sprintf("E2E%04d", i))
				form.Set("sp", "80")
				form.Set("tp", "100")
				form.Set("dr", xml)
				form.Set("attempt_token", fmt.Sprintf("tok%04d", i))
				req := httptest.NewRequest("POST", "/webhook", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				resp, err := app.Test(req, -1)
				if err != nil {
					b.Errorf("request: %v", err)
					return
				}
				if resp.StatusCode != 200 {
					body, _ := io.ReadAll(resp.Body)
					b.Errorf("status=%d body=%s", resp.StatusCode, string(body))
				}
			}(i)
		}
		wg.Wait()
	}
}
