package handlers

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
	"pgregory.net/rapid"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/submission"
)

// setupTestDB initializes an in-memory SQLite database and creates the necessary schemas.
// Returns a cleanup function that closes the DB.
func setupTestDB(t *testing.T) func() {
	t.Helper()
	var err error
	db.DB, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite: %v", err)
	}

	schemas := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS peserta (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			no_id TEXT NOT NULL,
			password TEXT NOT NULL,
			nama_peserta TEXT NOT NULL,
			kelas_id INTEGER NOT NULL,
			ruang_id INTEGER NOT NULL,
			UNIQUE(tenant_id, no_id)
		);`,
		`CREATE TABLE IF NOT EXISTS cek_login (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			peserta_id INTEGER NOT NULL,
			mapel_id INTEGER NOT NULL,
			attempt_token TEXT,
			login_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(tenant_id, peserta_id, mapel_id)
		);`,
		`CREATE TABLE IF NOT EXISTS hasil_tes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			peserta_id INTEGER NOT NULL,
			mapel_id INTEGER NOT NULL,
			skor REAL,
			skor_maks REAL,
			detail_xml TEXT,
			status TEXT,
			validasi TEXT NOT NULL,
			waktu_selesai DATETIME,
			UNIQUE(tenant_id, validasi)
		);`,
		`CREATE TABLE IF NOT EXISTS hasil_tes_detail (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hasil_tes_id INTEGER NOT NULL,
			question_id TEXT NOT NULL,
			question_text TEXT,
			question_type TEXT,
			status TEXT,
			awarded_points REAL,
			max_points REAL,
			user_answer TEXT,
			correct_answer TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS kelas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nama_kelas TEXT UNIQUE NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS mapel (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL DEFAULT 1,
			nama_mapel TEXT UNIQUE NOT NULL,
			durasi_menit INTEGER DEFAULT 90
		);`,
	}

	for _, s := range schemas {
		_, err = db.DB.Exec(s)
		if err != nil {
			t.Fatalf("Failed to execute schema setup: %v\nSQL: %s", err, s)
		}
	}

	// Seed basic test data
	_, _ = db.DB.Exec("INSERT INTO tenants (id, slug, name) VALUES (1, 'default', 'Sekolah Contoh')")
	_, _ = db.DB.Exec("INSERT INTO kelas (id, nama_kelas) VALUES (10, 'XII-RPL')")
	_, _ = db.DB.Exec("INSERT INTO mapel (id, tenant_id, nama_mapel, durasi_menit) VALUES (5, 1, 'Matematika', 90)")
	_, _ = db.DB.Exec("INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (42, 1, '2026001', 'siswa123', 'Syahrul Hamdi', 10, 1)")
	_, _ = db.DB.Exec("INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, attempt_token) VALUES (1, 42, 5, 'attempt-secret')") // Active Exam Session

	return func() {
		if db.DB != nil {
			db.DB.Close()
		}
	}
}

// setupISpringTestApp creates a fresh in-memory SQLite DB, a FilesystemQueue in t.TempDir(),
// wires them together, and returns the Fiber app, the queue, and a cleanup function.
// Each call produces an independent test environment (fresh dir, fresh db).
// Requirement 15.1.
func setupISpringTestApp(t *testing.T) (app *fiber.App, fsQueue *submission.FilesystemQueue, cleanup func()) {
	t.Helper()

	// Fresh in-memory DB
	dbCleanup := setupTestDB(t)

	// Fresh FilesystemQueue in a temp directory
	queueDir := t.TempDir()
	var err error
	fsQueue, err = submission.NewFilesystemQueue(queueDir)
	if err != nil {
		t.Fatalf("Failed to create FilesystemQueue: %v", err)
	}

	// Wire the queue into the handler
	SetSubmissionQueue(fsQueue)

	// Build Fiber app with tenant middleware
	app = fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/webhook", ISpringWebhook)

	cleanup = func() {
		dbCleanup()
		// Reset global queue to avoid leaking state between tests
		SetSubmissionQueue(nil)
	}

	return app, fsQueue, cleanup
}

// SetupTestDB is kept for backward compatibility with other tests in this package.
func SetupTestDB(t *testing.T) {
	setupTestDB(t)
}

func TeardownTestDB() {
	if db.DB != nil {
		db.DB.Close()
	}
}

// TestISpringWebhookSuccess verifies the full happy path:
// POST → HTTP 200 → file in pending/ → processor runs → hasil_tes in DB.
// Requirement 15.2.
func TestISpringWebhookSuccess(t *testing.T) {
	app, fsQueue, cleanup := setupISpringTestApp(t)
	defer cleanup()

	// Mock iSpring XML Detailed Results (dr) containing a multipleChoiceQuestion and an essayQuestion
	mockXML := `<?xml version="1.0" encoding="UTF-8"?>
	<quizReport version="1">
		<questions>
			<multipleChoiceQuestion id="q1" evaluationEnabled="true" maxPoints="10" awardedPoints="10" status="correct">
				<direction><text>Siapakah nama penemu gravitasi?</text></direction>
				<answers correctAnswerIndex="1" userAnswerIndex="1">
					<answer><text>Albert Einstein</text></answer>
					<answer><text>Isaac Newton</text></answer>
					<answer><text>Galileo Galilei</text></answer>
				</answers>
			</multipleChoiceQuestion>
			<essayQuestion id="q2" evaluationEnabled="false" maxPoints="20" awardedPoints="0" status="answered">
				<direction><text>Jelaskan teori relativitas secara singkat.</text></direction>
				<userAnswer>Teori relativitas adalah teori fisika yang dikembangkan oleh Einstein...</userAnswer>
			</essayQuestion>
		</questions>
	</quizReport>`

	form := url.Values{}
	form.Add("sid", "2026001")
	form.Add("sp", "10")
	form.Add("tp", "30")
	form.Add("dr", mockXML)
	form.Add("attempt_token", "attempt-secret")

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	// Step 1: verify HTTP 200
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Logf("Response body: %s", string(bodyBytes))
	}

	// Step 2: verify a file exists in pending/
	stats, err := fsQueue.GetStats(context.Background())
	if err != nil {
		t.Fatalf("Failed to get queue stats: %v", err)
	}
	if stats.PendingCount != 1 {
		t.Errorf("Expected 1 file in pending/, got %d", stats.PendingCount)
	}

	// Step 3: run processor directly to process the queued job
	processor := submission.NewProcessor(db.DB)
	job, err := fsQueue.Dequeue(context.Background())
	if err != nil {
		t.Fatalf("Failed to dequeue job: %v", err)
	}
	if job == nil {
		t.Fatal("Expected a job in the queue, got nil")
	}

	processErr := processor.ProcessBatch(context.Background(), []*submission.SubmissionJob{job})
	if processErr != nil {
		t.Fatalf("ProcessBatch failed: %v", processErr)
	}

	// Mark job completed
	if err := fsQueue.MarkCompleted(context.Background(), job.ID); err != nil {
		t.Fatalf("MarkCompleted failed: %v", err)
	}

	// Step 4: verify DB results

	// 4a. Verify hasil_tes was created
	var score, maxScore float64
	var status string
	err = db.DB.QueryRow("SELECT skor, skor_maks, status FROM hasil_tes WHERE peserta_id = 42").Scan(&score, &maxScore, &status)
	if err != nil {
		t.Fatalf("Failed to query hasil_tes: %v", err)
	}
	if score != 10 || maxScore != 30 || status != "submitted" {
		t.Errorf("hasil_tes mismatch: score=%f, maxScore=%f, status=%s", score, maxScore, status)
	}

	// 4b. Verify hasil_tes_detail rows were created and index-based answers parsed correctly
	// q1: Multiple Choice
	var qText1, qType1, userAns1, correctAns1, status1 string
	var awPoints1, maxPoints1 float64
	err = db.DB.QueryRow("SELECT question_text, question_type, user_answer, correct_answer, status, awarded_points, max_points FROM hasil_tes_detail WHERE question_id = 'q1'").
		Scan(&qText1, &qType1, &userAns1, &correctAns1, &status1, &awPoints1, &maxPoints1)
	if err != nil {
		t.Fatalf("Failed to query details for q1: %v", err)
	}
	if qText1 != "Siapakah nama penemu gravitasi?" || qType1 != "multipleChoiceQuestion" ||
		userAns1 != "Isaac Newton" || correctAns1 != "Isaac Newton" || status1 != "correct" ||
		awPoints1 != 10 || maxPoints1 != 10 {
		t.Errorf("q1 details mismatch! userAns1=%s, correctAns1=%s, qText1=%s", userAns1, correctAns1, qText1)
	}

	// q2: Essay
	var qText2, qType2, userAns2, correctAns2, status2 string
	var awPoints2, maxPoints2 float64
	err = db.DB.QueryRow("SELECT question_text, question_type, user_answer, correct_answer, status, awarded_points, max_points FROM hasil_tes_detail WHERE question_id = 'q2'").
		Scan(&qText2, &qType2, &userAns2, &correctAns2, &status2, &awPoints2, &maxPoints2)
	if err != nil {
		t.Fatalf("Failed to query details for q2: %v", err)
	}
	if qText2 != "Jelaskan teori relativitas secara singkat." || qType2 != "essayQuestion" ||
		!strings.Contains(userAns2, "Teori relativitas") || correctAns2 != "Perlu Penilaian Manual" || status2 != "answered" ||
		awPoints2 != 0 || maxPoints2 != 20 {
		t.Errorf("q2 essay details mismatch! userAns2=%s, correctAns2=%s, qText2=%s", userAns2, correctAns2, qText2)
	}

	// 4c. Verify cek_login record was deleted (processor clears it)
	var count int
	db.DB.QueryRow("SELECT COUNT(*) FROM cek_login WHERE peserta_id = 42").Scan(&count)
	if count != 0 {
		t.Errorf("cek_login session was not cleared after successful completion! count=%d", count)
	}
}

// TestISpringWebhookForbidden verifies that direct postings without active cek_login session fail.
// Requirement 15.3.
func TestISpringWebhookForbidden(t *testing.T) {
	app, fsQueue, cleanup := setupISpringTestApp(t)
	defer cleanup()

	// Clear the active login session to simulate unauthorized submit (cheating attempt)
	_, _ = db.DB.Exec("DELETE FROM cek_login")

	form := url.Values{}
	form.Add("sid", "2026001")
	form.Add("sp", "10")
	form.Add("tp", "30")

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403 Forbidden for unauthorized cheat attempt, got %d", resp.StatusCode)
	}

	// Verify pending/ is empty — no job should have been enqueued
	stats, err := fsQueue.GetStats(context.Background())
	if err != nil {
		t.Fatalf("Failed to get queue stats: %v", err)
	}
	if stats.PendingCount != 0 {
		t.Errorf("Expected pending/ to be empty after 403, got %d files", stats.PendingCount)
	}
}

// TestISpringWebhookRejectsMissingAttemptToken verifies that missing or wrong attempt_token returns 403.
// Requirement 15.4.
func TestISpringWebhookRejectsMissingAttemptToken(t *testing.T) {
	app, fsQueue, cleanup := setupISpringTestApp(t)
	defer cleanup()

	form := url.Values{}
	form.Add("sid", "2026001")
	form.Add("sp", "10")
	form.Add("tp", "30")
	// No attempt_token field — should be rejected

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("Expected status 403 for missing attempt_token, got %d", resp.StatusCode)
	}

	// Verify pending/ is empty — no job should have been enqueued
	stats, err := fsQueue.GetStats(context.Background())
	if err != nil {
		t.Fatalf("Failed to get queue stats: %v", err)
	}
	if stats.PendingCount != 0 {
		t.Errorf("Expected pending/ to be empty after 403, got %d files", stats.PendingCount)
	}
}

func TestPropertyISpringWebhookHappyPathEnqueuesMatchingJob(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		app, fsQueue, cleanup := setupISpringTestApp(t)
		defer cleanup()

		noID := rapid.StringMatching(`[A-Za-z0-9_-]{1,16}`).Draw(rt, "no_id")
		token := rapid.StringMatching(`[0-9a-f]{32}`).Draw(rt, "attempt_token")
		score := rapid.StringMatching(`[0-9]{1,3}`).Draw(rt, "score")
		maxScore := rapid.StringMatching(`[0-9]{1,3}`).Draw(rt, "max_score")

		if _, err := db.DB.Exec("UPDATE peserta SET no_id = ? WHERE id = 42", noID); err != nil {
			rt.Fatalf("update peserta no_id: %v", err)
		}
		if _, err := db.DB.Exec("UPDATE cek_login SET attempt_token = ? WHERE peserta_id = 42", token); err != nil {
			rt.Fatalf("update attempt_token: %v", err)
		}

		form := url.Values{}
		form.Add("sid", noID)
		form.Add("sp", score)
		form.Add("tp", maxScore)
		form.Add("attempt_token", token)

		req := httptest.NewRequest("POST", "/webhook", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := app.Test(req)
		if err != nil {
			rt.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			rt.Fatalf("status = %d body = %q, want 200", resp.StatusCode, string(body))
		}

		job, err := fsQueue.Dequeue(context.Background())
		if err != nil {
			rt.Fatalf("Dequeue: %v", err)
		}
		if job == nil {
			rt.Fatal("expected queued job")
		}
		if job.NoID != noID || job.Score != score || job.MaxScore != maxScore || job.AttemptToken != token {
			rt.Fatalf("job mismatch: %+v", job)
		}
		if job.Validasi != "1_"+noID+"_5" {
			rt.Fatalf("validasi = %q, want %q", job.Validasi, "1_"+noID+"_5")
		}
	})
}

// TestExportEssayResults verifies exporting to CSV, XLSX, and PDF
func TestExportEssayResults(t *testing.T) {
	SetupTestDB(t)
	defer TeardownTestDB()

	// Insert mock essay answers
	_, _ = db.DB.Exec(`
		INSERT INTO hasil_tes (id, tenant_id, peserta_id, mapel_id, skor, skor_maks, status, validasi)
		VALUES (100, 1, 42, 5, 0, 20, 'submitted', '1_2026001_5')
	`)
	_, _ = db.DB.Exec(`
		INSERT INTO hasil_tes_detail (hasil_tes_id, question_id, question_text, question_type, status, awarded_points, max_points, user_answer, correct_answer)
		VALUES (100, 'q2', 'Jelaskan teori relativitas', 'essayQuestion', 'answered', 0, 20, 'Einstein relativitas', 'Perlu Penilaian Manual')
	`)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "admin")
		return c.Next()
	})
	app.Get("/export/:format", ExportEssayResults)

	// 1. Test CSV format
	reqCSV := httptest.NewRequest("GET", "/export/csv", nil)
	respCSV, _ := app.Test(reqCSV)
	if respCSV.StatusCode != http.StatusOK {
		t.Errorf("CSV Export failed: status=%d", respCSV.StatusCode)
	}
	if respCSV.Header.Get("Content-Type") != "text/csv" {
		t.Errorf("Expected Content-Type text/csv, got %s", respCSV.Header.Get("Content-Type"))
	}
	bodyCSV, _ := io.ReadAll(respCSV.Body)
	if !strings.Contains(string(bodyCSV), "Einstein relativitas") {
		t.Errorf("CSV output does not contain mock student answer!")
	}

	// 2. Test XLSX format (Excelize)
	reqXLSX := httptest.NewRequest("GET", "/export/xlsx", nil)
	respXLSX, _ := app.Test(reqXLSX)
	if respXLSX.StatusCode != http.StatusOK {
		t.Errorf("XLSX Export failed: status=%d", respXLSX.StatusCode)
	}
	if !strings.Contains(respXLSX.Header.Get("Content-Type"), "sheet") {
		t.Errorf("Expected spreadsheet Content-Type, got %s", respXLSX.Header.Get("Content-Type"))
	}
	bodyXLSX, _ := io.ReadAll(respXLSX.Body)
	if len(bodyXLSX) == 0 {
		t.Errorf("XLSX output body is empty!")
	}

	// 3. Test PDF format (Gofpdf)
	reqPDF := httptest.NewRequest("GET", "/export/pdf", nil)
	respPDF, _ := app.Test(reqPDF)
	if respPDF.StatusCode != http.StatusOK {
		t.Errorf("PDF Export failed: status=%d", respPDF.StatusCode)
	}
	if respPDF.Header.Get("Content-Type") != "application/pdf" {
		t.Errorf("Expected Content-Type application/pdf, got %s", respPDF.Header.Get("Content-Type"))
	}
	bodyPDF, _ := io.ReadAll(respPDF.Body)
	if len(bodyPDF) < 100 { // PDF header is usually around a few hundred bytes minimum
		t.Errorf("PDF output body is empty or too short!")
	}

	// Verify PDF Header signature (%PDF-1.3)
	if len(bodyPDF) >= 4 {
		sig := string(bodyPDF[:4])
		if sig != "%PDF" {
			t.Errorf("Exported PDF does not feature valid PDF signature header! sig=%s", sig)
		}
	}
}

// TestMain allows manual testing run via go command
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
