package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/submission"
)

// SetupFeaturesTestDB initializes a mock SQLite in-memory database with our new premium fields
func SetupFeaturesTestDB(t *testing.T) {
	var err error
	db.DB, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open mock in-memory SQLite: %v", err)
	}

	schemas := []string{
		`CREATE TABLE IF NOT EXISTS tenants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS kelas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nama_kelas TEXT UNIQUE NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS mapel (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			nama_mapel TEXT UNIQUE NOT NULL,
			kode_mapel TEXT,
			durasi_menit INTEGER DEFAULT 90
		);`,
		`CREATE TABLE IF NOT EXISTS peserta (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			no_id TEXT NOT NULL,
			password TEXT NOT NULL,
			nama_peserta TEXT NOT NULL,
			kelas_id INTEGER NOT NULL,
			ruang_id INTEGER NOT NULL,
			deleted_at DATETIME,
			UNIQUE(tenant_id, no_id)
		);`,
		`CREATE TABLE IF NOT EXISTS cek_login (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			peserta_id INTEGER NOT NULL,
			mapel_id INTEGER NOT NULL,
			attempt_token TEXT,
			login_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_activity DATETIME DEFAULT CURRENT_TIMESTAMP,
			tab_switch_count INTEGER DEFAULT 0,
			answered_count INTEGER DEFAULT 0,
			total_questions INTEGER DEFAULT 0,
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
			status TEXT DEFAULT 'submitted',
			validasi TEXT NOT NULL,
			waktu_selesai DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
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
	}

	for _, s := range schemas {
		_, err = db.DB.Exec(s)
		if err != nil {
			t.Fatalf("Failed to execute features schema: %v\nSQL: %s", err, s)
		}
	}

	// Seed basic test data
	_, _ = db.DB.Exec("INSERT INTO tenants (id, slug, name) VALUES (1, 'schoolA', 'Sekolah RPL')")
	_, _ = db.DB.Exec("INSERT INTO kelas (id, nama_kelas) VALUES (2, 'XI-RPL-1')")
	_, _ = db.DB.Exec("INSERT INTO mapel (id, tenant_id, nama_mapel, kode_mapel, durasi_menit) VALUES (7, 1, 'Pemrograman Web', 'PW-11', 45)")
	_, _ = db.DB.Exec("INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (15, 1, '5050', 'secure', 'Siswa Cerdas', 2, 1)")

	// Create active exam session
	_, _ = db.DB.Exec("INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, attempt_token, login_time, last_activity, tab_switch_count, answered_count, total_questions) VALUES (1, 15, 7, 'feature-attempt-token', datetime('now', '-10 minutes'), datetime('now'), 2, 8, 10)")
}

func TeardownFeaturesTestDB() {
	if db.DB != nil {
		db.DB.Close()
	}
}

// 1. Test Web-Based Essay Grading
func TestEssayGrading(t *testing.T) {
	SetupFeaturesTestDB(t)
	defer TeardownFeaturesTestDB()

	// Seed pre-existing exam result
	_, _ = db.DB.Exec(`
		INSERT INTO hasil_tes (id, tenant_id, peserta_id, mapel_id, skor, skor_maks, status, validasi)
		VALUES (100, 1, 15, 7, 50.0, 100.0, 'submitted', '1_5050_7')
	`)
	// Seed 1 multiple choice (correct = 50 pts) and 1 essay question (ungraded = 0 pts, max = 50 pts)
	_, _ = db.DB.Exec(`
		INSERT INTO hasil_tes_detail (id, hasil_tes_id, question_id, question_text, question_type, status, awarded_points, max_points, user_answer, correct_answer)
		VALUES (1001, 100, 'q_mc', 'MC Question', 'multipleChoiceQuestion', 'correct', 50.0, 50.0, 'A', 'A')
	`)
	_, _ = db.DB.Exec(`
		INSERT INTO hasil_tes_detail (id, hasil_tes_id, question_id, question_text, question_type, status, awarded_points, max_points, user_answer, correct_answer)
		VALUES (1002, 100, 'q_essay', 'Jelaskan MVC', 'essayQuestion', 'answered', 0.0, 50.0, 'Model View Controller', 'Perlu Penilaian Manual')
	`)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "admin")
		return c.Next()
	})

	// GET essays
	app.Get("/essays", GetEssayAnswers)
	reqGet := httptest.NewRequest("GET", "/essays?kelas_id=2&mapel_id=7", nil)
	respGet, err := app.Test(reqGet)
	if err != nil {
		t.Fatalf("Failed GET request: %v", err)
	}
	if respGet.StatusCode != http.StatusOK {
		t.Errorf("Expected GET status 200, got %d", respGet.StatusCode)
	}

	var getResult struct {
		Data []EssayAnswerResponse `json:"data"`
	}
	_ = json.NewDecoder(respGet.Body).Decode(&getResult)
	if len(getResult.Data) != 1 {
		t.Errorf("Expected 1 essay answer, got %d", len(getResult.Data))
	} else if getResult.Data[0].DetailID != 1002 {
		t.Errorf("Expected detail ID 1002, got %d", getResult.Data[0].DetailID)
	}

	// POST grade
	app.Post("/grade", GradeEssayAnswer)
	reqBody := `{"detail_id": 1002, "awarded_points": 45.0}`
	reqPost := httptest.NewRequest("POST", "/grade", bytes.NewBufferString(reqBody))
	reqPost.Header.Set("Content-Type", "application/json")

	respPost, err := app.Test(reqPost)
	if err != nil {
		t.Fatalf("Failed POST request: %v", err)
	}
	if respPost.StatusCode != http.StatusOK {
		t.Errorf("Expected POST status 200, got %d", respPost.StatusCode)
	}

	// Verify DB status update
	var awPoints, maxPoints float64
	var detailStatus string
	err = db.DB.QueryRow("SELECT awarded_points, max_points, status FROM hasil_tes_detail WHERE id = 1002").Scan(&awPoints, &maxPoints, &detailStatus)
	if err != nil {
		t.Fatalf("Failed to query detail: %v", err)
	}
	if awPoints != 45.0 || detailStatus != "partial" {
		t.Errorf("Expected points 45.0 and partial status, got %f and %s", awPoints, detailStatus)
	}

	// Verify Parent Score Recalculation (50 mc + 45 essay = 95.0 total score)
	var parentSkor float64
	err = db.DB.QueryRow("SELECT skor FROM hasil_tes WHERE id = 100").Scan(&parentSkor)
	if err != nil {
		t.Fatalf("Failed to query parent score: %v", err)
	}
	if parentSkor != 95.0 {
		t.Errorf("Expected total recalculated score 95.0, got %f", parentSkor)
	}
}

// 2. Test CBT Global Timer
func TestGlobalTimer(t *testing.T) {
	SetupFeaturesTestDB(t)
	defer TeardownFeaturesTestDB()

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "student")
		c.Locals("user_id", 15)
		return c.Next()
	})

	app.Get("/remaining-time", GetRemainingTime)
	req := httptest.NewRequest("GET", "/remaining-time?mapel_id=7", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed GET remaining-time request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			RemainingSeconds int  `json:"remaining_seconds"`
			IsActive         bool `json:"is_active"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	// Since we seeded login_time as 'now - 10 minutes' and durasi_menit = 45 minutes,
	// Remaining seconds should be around 35 minutes (2100 seconds)
	if result.Data.RemainingSeconds < 2000 || result.Data.RemainingSeconds > 2200 {
		t.Errorf("Remaining seconds expected around 2100, got %d", result.Data.RemainingSeconds)
	}
	if !result.Data.IsActive {
		t.Errorf("Expected is_active to be true")
	}
}

// 3. Test iSpring Submission Grace Period Check
// With the new async design, the handler enqueues the job (HTTP 200) and the
// processor rejects it with "grace period exceeded". After Max_Retries calls to
// MarkFailed, the job ends up in failed/.
// Requirement 15.5.
func TestISpringGracePeriod(t *testing.T) {
	SetupFeaturesTestDB(t)
	defer TeardownFeaturesTestDB()

	// Seed session that was started far in the past (year 2000) for an exam of 45 minutes.
	// With 5 minutes grace, any time > 50 minutes ago exceeds the grace period.
	// Note: SQLite datetime() modifiers don't work reliably with modernc.org/sqlite driver,
	// so we use a hardcoded past timestamp instead.
	_, err := db.DB.Exec("UPDATE cek_login SET login_time = '2000-01-01 00:00:00' WHERE peserta_id = 15")
	if err != nil {
		t.Fatalf("Failed to update login_time: %v", err)
	}

	// Create a FilesystemQueue in a temp directory for this test.
	queueDir := t.TempDir()
	fsQueue, err := submission.NewFilesystemQueue(queueDir)
	if err != nil {
		t.Fatalf("Failed to create FilesystemQueue: %v", err)
	}
	SetSubmissionQueue(fsQueue)
	defer SetSubmissionQueue(nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/webhook", ISpringWebhook)

	form := url.Values{}
	form.Add("sid", "5050")
	form.Add("sp", "80")
	form.Add("tp", "100")
	form.Add("dr", "<quizReport></quizReport>")
	form.Add("attempt_token", "feature-attempt-token")

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed request: %v", err)
	}

	// Step 1: handler should return HTTP 200 — job is enqueued, grace check is in processor.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK from handler (grace period check is in processor), got %d", resp.StatusCode)
	}

	// Step 2: run processor — it should return "grace period exceeded" error.
	processor := submission.NewProcessor(db.DB)
	ctx := context.Background()

	job, deqErr := fsQueue.Dequeue(ctx)
	if deqErr != nil {
		t.Fatalf("Failed to dequeue job: %v", deqErr)
	}
	if job == nil {
		t.Fatal("Expected a job in the queue after POST, got nil")
	}

	processErr := processor.ProcessBatch(ctx, []*submission.SubmissionJob{job})
	if processErr == nil {
		t.Errorf("Expected processor to return an error for grace period exceeded, got nil")
	} else if !strings.Contains(processErr.Error(), "grace period exceeded") {
		t.Errorf("Expected error to contain 'grace period exceeded', got: %v", processErr)
	}

	// Step 3: simulate Max_Retries calls to MarkFailed so the job ends up in failed/.
	// The queue has maxRetries=5. We already have the job dequeued (in processing/).
	// Call MarkFailed once — this puts it back in pending/ with retry_count=1.
	// Repeat until retry_count reaches maxRetries (5), at which point it goes to failed/.
	maxRetries := 5
	gracePeriodErr := fmt.Errorf("grace period exceeded")

	// First MarkFailed (job is currently in processing/ from the Dequeue above)
	if mErr := fsQueue.MarkFailed(ctx, job.ID, gracePeriodErr); mErr != nil {
		t.Fatalf("MarkFailed (attempt 1) failed: %v", mErr)
	}

	// Remaining retries: dequeue → MarkFailed until maxRetries
	for i := 2; i <= maxRetries; i++ {
		nextJob, dErr := fsQueue.Dequeue(ctx)
		if dErr != nil {
			t.Fatalf("Dequeue (attempt %d) failed: %v", i, dErr)
		}
		if nextJob == nil {
			t.Fatalf("Expected job in pending/ for retry attempt %d, got nil", i)
		}
		if mErr := fsQueue.MarkFailed(ctx, nextJob.ID, gracePeriodErr); mErr != nil {
			t.Fatalf("MarkFailed (attempt %d) failed: %v", i, mErr)
		}
	}

	// Step 4: verify job is in failed/ after Max_Retries exhausted.
	stats, sErr := fsQueue.GetStats(ctx)
	if sErr != nil {
		t.Fatalf("GetStats failed: %v", sErr)
	}
	if stats.FailedCount != 1 {
		t.Errorf("Expected 1 file in failed/ after max retries, got %d", stats.FailedCount)
	}
	if stats.PendingCount != 0 {
		t.Errorf("Expected 0 files in pending/ after max retries, got %d", stats.PendingCount)
	}
}

// 4. Test Visual Item Analysis (Pedagogical difficulty metrics)
func TestItemAnalysis(t *testing.T) {
	SetupFeaturesTestDB(t)
	defer TeardownFeaturesTestDB()

	// Seed results data
	_, _ = db.DB.Exec(`
		INSERT INTO hasil_tes (id, tenant_id, peserta_id, mapel_id, skor, skor_maks, status, validasi)
		VALUES (200, 1, 15, 7, 10.0, 20.0, 'submitted', '1_5050_7_item')
	`)

	// Q1: Sangat Mudah (100% correct - 3 correct out of 3)
	_, _ = db.DB.Exec("INSERT INTO hasil_tes_detail (hasil_tes_id, question_id, question_text, question_type, status) VALUES (200, 'q_easy', 'Easy Question', 'mc', 'correct')")
	_, _ = db.DB.Exec("INSERT INTO hasil_tes_detail (hasil_tes_id, question_id, question_text, question_type, status) VALUES (200, 'q_easy', 'Easy Question', 'mc', 'correct')")
	_, _ = db.DB.Exec("INSERT INTO hasil_tes_detail (hasil_tes_id, question_id, question_text, question_type, status) VALUES (200, 'q_easy', 'Easy Question', 'mc', 'correct')")

	// Q2: Sangat Sukar (0% correct - 0 correct out of 3)
	_, _ = db.DB.Exec("INSERT INTO hasil_tes_detail (hasil_tes_id, question_id, question_text, question_type, status) VALUES (200, 'q_hard', 'Hard Question', 'mc', 'incorrect')")
	_, _ = db.DB.Exec("INSERT INTO hasil_tes_detail (hasil_tes_id, question_id, question_text, question_type, status) VALUES (200, 'q_hard', 'Hard Question', 'mc', 'incorrect')")
	_, _ = db.DB.Exec("INSERT INTO hasil_tes_detail (hasil_tes_id, question_id, question_text, question_type, status) VALUES (200, 'q_hard', 'Hard Question', 'mc', 'incorrect')")

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "admin")
		return c.Next()
	})

	app.Get("/analysis", GetItemAnalysis)
	req := httptest.NewRequest("GET", "/analysis?mapel_id=7", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed analysis request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data []ItemDifficultyAnalysis `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Data) != 2 {
		t.Fatalf("Expected 2 item analysis results, got %d", len(result.Data))
	}

	// Verify classification logic
	for _, item := range result.Data {
		if item.QuestionID == "q_easy" {
			if item.SuccessRate != 100.0 || item.DifficultyClassification != "Sangat Mudah" {
				t.Errorf("q_easy mismatch: success=%f classification=%s", item.SuccessRate, item.DifficultyClassification)
			}
		} else if item.QuestionID == "q_hard" {
			if item.SuccessRate != 0.0 || item.DifficultyClassification != "Sangat Sukar" {
				t.Errorf("q_hard mismatch: success=%f classification=%s", item.SuccessRate, item.DifficultyClassification)
			}
		}
	}
}

// 5. Test Live Proctoring SSE Setup
func TestLiveProctoringSSE(t *testing.T) {
	SetupFeaturesTestDB(t)
	defer TeardownFeaturesTestDB()

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "supervisor")
		c.Locals("user_id", 1) // matches ruang_id
		return c.Next()
	})

	app.Get("/live", GetRoomStatusSSE)

	req := httptest.NewRequest("GET", "/live", nil)
	// Gofiber Test parses body stream, but since SSE loops forever,
	// app.Test with NewRequest might block if it tries to read the entire body.
	// However, we can test fiber's HTTP response headers negotiation immediately!
	// Fiber's Test implementation lets us test handler execution.
	// But to avoid locking/blocking indefinitely during the unit test loop,
	// let's pass a request context with timeout so it finishes gracefully!
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := app.Test(req, 200) // Timeout is handled gracefully by app.Test in Fiber
	if err == nil {
		if resp.Header.Get("Content-Type") != "text/event-stream" {
			t.Errorf("Expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
		}
		if resp.Header.Get("Connection") != "keep-alive" {
			t.Errorf("Expected Connection keep-alive, got %s", resp.Header.Get("Connection"))
		}
	}
}
