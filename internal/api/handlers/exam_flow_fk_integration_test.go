package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/submission"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// Gate 1 preservation test for codebase-bug-sweep clauses 3.1 and 3.3.
//
// Gate 1 turns FOREIGN KEY enforcement on for the first time in the project's history: until
// now the pragma was silently dropped by the driver, so none of the constraints declared across
// 30+ migrations were ever applied. That cannot be verified by unit tests alone — enforcement is
// a property of the whole write path, and any INSERT that had been relying on a dangling or
// sentinel reference will now fail at runtime, in production, during an exam.
//
// This file therefore walks the production path end to end against a migrated database with
// enforcement genuinely active: student login, attempt registration, submission through the
// public iSpring webhook, queue pickup, processing into hasil_tes and hasil_tes_detail, and
// finally the results export. Every FK-bearing write in the exam chain is exercised. If FK
// enforcement broke any of them, it breaks here.

// examFlowFixture is the wiring shared by these tests: a migrated database (enforcement on), a
// filesystem queue, and a Fiber app whose role is switchable, because one flow spans the student,
// public-webhook and admin surfaces.
type examFlowFixture struct {
	app      *fiber.App
	database *sql.DB
	queue    *submission.FilesystemQueue
	role     *string
}

func newExamFlowFixture(t *testing.T) (*examFlowFixture, func()) {
	t.Helper()

	database, cleanupDB := testutil.NewMigratedDB(t)
	db.DB = database

	// The premise of the whole file: enforcement really is on. Without this guard a future
	// fixture change could silently restore the pre-Gate-1 semantics and these tests would keep
	// passing while proving nothing.
	var foreignKeys int
	if err := database.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("precondition: foreign_keys = %d, want 1; this test only means something with enforcement active", foreignKeys)
	}

	queue, err := submission.NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("create filesystem queue: %v", err)
	}
	SetSubmissionQueue(queue)

	role := "student"
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", role)
		c.Locals("user_id", 1)
		return c.Next()
	})
	app.Post("/api/auth/student-login", StudentLogin)
	app.Post("/api/student/start", StartExamSession)
	app.Post("/webhook", ISpringWebhook)
	app.Get("/api/results/export", ExportResultsCSV)

	fixture := &examFlowFixture{app: app, database: database, queue: queue, role: &role}
	return fixture, func() {
		SetSubmissionQueue(nil)
		_ = database.Close()
		cleanupDB()
	}
}

// seedExamFlowMasterData creates one referentially complete tenant: class, room, participant,
// subject, package, exam, active session, and the session-class link the eligibility check needs.
func seedExamFlowMasterData(t *testing.T, database *sql.DB) {
	t.Helper()
	now := time.Now()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa Integrasi")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedSoalPackage(t, database, 10, 1, "Kimia XII UAS", "uuid-flow")
	testutil.SeedExam(t, database, 1, 1, 1, intPtr(10))
	testutil.SeedExamSession(t, database, 1, 1, 1,
		fmtTime(now.Add(-time.Hour)), fmtTime(now.Add(time.Hour)), "TOKENFLOW", "aktif")
	if _, err := database.Exec(`INSERT INTO exam_session_kelas (session_id, kelas_id) VALUES (1, 1)`); err != nil {
		t.Fatalf("link session to kelas: %v", err)
	}
}

// examFlowForeignKeyViolations returns one description per PRAGMA foreign_key_check row, so a
// flow can assert it left no referential damage behind.
func examFlowForeignKeyViolations(t *testing.T, database *sql.DB) []string {
	t.Helper()
	rows, err := database.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var table, parent sql.NullString
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			t.Fatalf("scan foreign_key_check: %v", err)
		}
		out = append(out, fmt.Sprintf("%s rowid=%d -> %s (fk #%d)", table.String, rowid.Int64, parent.String, fkid.Int64))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign_key_check: %v", err)
	}
	return out
}

const examFlowDetailXML = `<?xml version="1.0" encoding="UTF-8"?>
<quizReport version="1">
  <questions>
    <multipleChoiceQuestion id="q1" evaluationEnabled="true" maxPoints="10" awardedPoints="10" status="correct">
      <direction><text>Unsur dengan nomor atom 6?</text></direction>
      <answers correctAnswerIndex="0" userAnswerIndex="0">
        <answer><text>Karbon</text></answer>
        <answer><text>Nitrogen</text></answer>
      </answers>
    </multipleChoiceQuestion>
  </questions>
</quizReport>`

// submitViaWebhook posts a result exactly the way the iSpring player does and returns the
// response, so the flow test goes through the real public handler rather than the queue API.
func submitViaWebhook(t *testing.T, fixture *examFlowFixture, noID, attemptToken string) *http.Response {
	t.Helper()
	form := url.Values{}
	form.Add("sid", noID)
	form.Add("sp", "10")
	form.Add("tp", "10")
	form.Add("dr", examFlowDetailXML)
	form.Add("attempt_token", attemptToken)

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := fixture.app.Test(req, -1)
	if err != nil {
		t.Fatalf("webhook request: %v", err)
	}
	return resp
}

// TestExamFlowStudentLoginAndSessionStartUnderForeignKeys covers the first half of the exam
// chain on the session-based path: authenticate against the session token, then register the
// attempt. The cek_login write here is FK-bound to peserta and tenants, so it is one of the
// writes Gate 1 could have broken.
func TestExamFlowStudentLoginAndSessionStartUnderForeignKeys(t *testing.T) {
	fixture, cleanup := newExamFlowFixture(t)
	defer cleanup()
	seedExamFlowMasterData(t, fixture.database)

	loginResp := doJSON(t, fixture.app, "POST", "/api/auth/student-login",
		strings.NewReader(`{"no_id":"2026001","password":"siswa123","token":"TOKENFLOW"}`))
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("student login status = %d, want 200 (body=%v)", loginResp.StatusCode, decodeJSON(t, loginResp))
	}
	loginData, _ := decodeJSON(t, loginResp)["data"].(map[string]interface{})
	if sessionID, _ := loginData["session_id"].(float64); sessionID != 1 {
		t.Fatalf("login returned session_id = %v, want 1", loginData["session_id"])
	}

	startResp := doJSON(t, fixture.app, "POST", "/api/student/start",
		strings.NewReader(`{"peserta_id":1,"session_id":1}`))
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d, want 200 (body=%v)", startResp.StatusCode, decodeJSON(t, startResp))
	}
	startData, _ := decodeJSON(t, startResp)["data"].(map[string]interface{})
	if attemptToken, _ := startData["attempt_token"].(string); attemptToken == "" {
		t.Fatalf("start did not return an attempt_token: %v", startData)
	}
	// The content cookie is part of the same write (SetContentToken), so its absence would also
	// point at a rejected update.
	if cookie := startResp.Header.Get("Set-Cookie"); !strings.Contains(cookie, "aether_exam=") {
		t.Errorf("expected the aether_exam content cookie, got %q", cookie)
	}

	var storedPeserta int
	var storedSession sql.NullInt64
	if err := fixture.database.QueryRow(
		`SELECT peserta_id, session_id FROM cek_login WHERE tenant_id = 1`,
	).Scan(&storedPeserta, &storedSession); err != nil {
		t.Fatalf("read cek_login written by the session start: %v", err)
	}
	if storedPeserta != 1 || storedSession != (sql.NullInt64{Int64: 1, Valid: true}) {
		t.Errorf("cek_login = (peserta %d, session %v), want (1, 1)", storedPeserta, storedSession)
	}

	if violations := examFlowForeignKeyViolations(t, fixture.database); len(violations) != 0 {
		t.Errorf("login + session start left %d foreign key violation(s): %v", len(violations), violations)
	}
}

// TestExamFlowSubmissionToExportUnderForeignKeys covers the second half of the chain — the part
// that persists a student's result — with enforcement active: webhook, queue, processor,
// hasil_tes, hasil_tes_detail, cek_login cleanup, and the admin export that reads it all back.
//
// The attempt is registered through the mapel-bound start path rather than the session-based one.
// That is not a convenience: an attempt created by the session-based path stores cek_login with
// mapel_id = NULL, and both ISpringWebhook and Processor scan that column into a non-nullable
// int, so submission fails before any of the writes below are reached. That defect is unrelated
// to foreign keys and is recorded in TestExamFlowSessionBasedSubmissionBlockedByNullMapelID.
func TestExamFlowSubmissionToExportUnderForeignKeys(t *testing.T) {
	fixture, cleanup := newExamFlowFixture(t)
	defer cleanup()
	seedExamFlowMasterData(t, fixture.database)

	startResp := doJSON(t, fixture.app, "POST", "/api/student/start",
		strings.NewReader(`{"peserta_id":1,"mapel_id":1}`))
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d, want 200 (body=%v)", startResp.StatusCode, decodeJSON(t, startResp))
	}
	startData, _ := decodeJSON(t, startResp)["data"].(map[string]interface{})
	attemptToken, _ := startData["attempt_token"].(string)
	if attemptToken == "" {
		t.Fatalf("start did not return an attempt_token: %v", startData)
	}

	webhookResp := submitViaWebhook(t, fixture, "2026001", attemptToken)
	if webhookResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(webhookResp.Body)
		t.Fatalf("webhook status = %d, want 200 (body=%s)", webhookResp.StatusCode, body)
	}

	ctx := context.Background()
	stats, err := fixture.queue.GetStats(ctx)
	if err != nil {
		t.Fatalf("queue stats: %v", err)
	}
	if stats.PendingCount != 1 {
		t.Fatalf("pending jobs = %d, want 1", stats.PendingCount)
	}
	job, err := fixture.queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if job == nil {
		t.Fatal("expected a queued job, got nil")
	}
	if err := submission.NewProcessor(fixture.database).Process(ctx, job); err != nil {
		t.Fatalf("Process under foreign_keys=ON: %v", err)
	}
	if err := fixture.queue.MarkCompleted(ctx, job.ID); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	var skor, skorMaks float64
	var status string
	if err := fixture.database.QueryRow(
		`SELECT skor, skor_maks, status FROM hasil_tes WHERE tenant_id = 1 AND peserta_id = 1`,
	).Scan(&skor, &skorMaks, &status); err != nil {
		t.Fatalf("read hasil_tes: %v", err)
	}
	if skor != 10 || skorMaks != 10 || status != "submitted" {
		t.Errorf("hasil_tes = (skor %v, skor_maks %v, status %q), want (10, 10, \"submitted\")", skor, skorMaks, status)
	}

	// hasil_tes_detail is FK-bound to hasil_tes, so the per-question write is a second, dependent
	// insert that enforcement could have rejected.
	var detailCount int
	if err := fixture.database.QueryRow(`SELECT COUNT(*) FROM hasil_tes_detail`).Scan(&detailCount); err != nil {
		t.Fatalf("count hasil_tes_detail: %v", err)
	}
	if detailCount != 1 {
		t.Errorf("hasil_tes_detail rows = %d, want 1", detailCount)
	}

	var remainingLogins int
	if err := fixture.database.QueryRow(`SELECT COUNT(*) FROM cek_login WHERE peserta_id = 1`).Scan(&remainingLogins); err != nil {
		t.Fatalf("count cek_login: %v", err)
	}
	if remainingLogins != 0 {
		t.Errorf("cek_login rows after processing = %d, want 0 (the processor clears the attempt)", remainingLogins)
	}

	*fixture.role = "admin"
	exportResp := doJSON(t, fixture.app, "GET", "/api/results/export", nil)
	if exportResp.StatusCode != http.StatusOK {
		t.Fatalf("export status = %d, want 200", exportResp.StatusCode)
	}
	exportBody, _ := io.ReadAll(exportResp.Body)
	for _, want := range []string{"NO_ID", "2026001", "Siswa Integrasi", "XII IPA 1", "Kimia", "10.00"} {
		if !strings.Contains(string(exportBody), want) {
			t.Errorf("export CSV does not contain %q; got:\n%s", want, exportBody)
		}
	}

	if violations := examFlowForeignKeyViolations(t, fixture.database); len(violations) != 0 {
		t.Errorf("the submission flow left %d foreign key violation(s): %v", len(violations), violations)
	}
}

// TestExamFlowSessionBasedSubmissionBlockedByNullMapelID records a defect found while building
// the Gate 1 preservation test. It is NOT a foreign-key problem and it is NOT in the scope of any
// clause in this spec, so it is left unfixed here rather than blended into the gate that flips
// FK enforcement — see design.md's rule against combining risky changes at one verification
// point.
//
// The defect: repository.CekLoginRepository.Start (the session-based attempt registration used
// by StartExamSession when session_id is given) never writes cek_login.mapel_id, so the column is
// NULL. Both consumers then scan it into a non-nullable int:
//
//   - handlers.ISpringWebhook: SELECT cl.tenant_id, cl.mapel_id, ... -> Scan(&mapelID) with
//     mapelID declared int, so the scan errors and the student receives HTTP 500
//     "session lookup failed". The submission is never enqueued.
//   - submission.Processor.processOneInTx: SELECT mapel_id, login_time, session_id -> the same
//     non-nullable scan, so even with the webhook fixed the job would fail and eventually land
//     in failed/.
//
// Consequence: for any attempt started on the session-based path, the student's result is lost.
// A fix has to decide where mapel_id comes from (exam_session -> exam -> mapel_id is the obvious
// source) and touches the repository, the handler and the processor, which is a behavioural
// change well outside clause 2.1.
//
// Unskip this test as the failing-first test for that work.
func TestExamFlowSessionBasedSubmissionBlockedByNullMapelID(t *testing.T) {
	fixture, cleanup := newExamFlowFixture(t)
	defer cleanup()
	seedExamFlowMasterData(t, fixture.database)

	startResp := doJSON(t, fixture.app, "POST", "/api/student/start",
		strings.NewReader(`{"peserta_id":1,"session_id":1}`))
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d, want 200", startResp.StatusCode)
	}
	startData, _ := decodeJSON(t, startResp)["data"].(map[string]interface{})
	attemptToken, _ := startData["attempt_token"].(string)

	webhookResp := submitViaWebhook(t, fixture, "2026001", attemptToken)
	if webhookResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(webhookResp.Body)
		t.Fatalf("webhook status = %d, want 200 (body=%s)", webhookResp.StatusCode, body)
	}

	ctx := context.Background()
	job, err := fixture.queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if job == nil {
		t.Fatal("expected a queued job, got nil")
	}
	if err := submission.NewProcessor(fixture.database).Process(ctx, job); err != nil {
		t.Fatalf("Process under foreign_keys=ON: %v", err)
	}
	if err := fixture.queue.MarkCompleted(ctx, job.ID); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	var skor, skorMaks float64
	var status string
	if err := fixture.database.QueryRow(
		`SELECT skor, skor_maks, status FROM hasil_tes WHERE tenant_id = 1 AND peserta_id = 1`,
	).Scan(&skor, &skorMaks, &status); err != nil {
		t.Fatalf("read hasil_tes: %v", err)
	}
	if skor != 10 || skorMaks != 10 || status != "submitted" {
		t.Errorf("hasil_tes = (skor %v, skor_maks %v, status %q), want (10, 10, \"submitted\")", skor, skorMaks, status)
	}
}

// TestExamFlowRejectsDanglingReferencesUnderEnforcement is the other half of clause 3.1: the
// enforcement that must not break the flows above must nevertheless actually reject a bad write.
// Without this, a fixture that quietly lost the pragma would still pass the flow tests.
func TestExamFlowRejectsDanglingReferencesUnderEnforcement(t *testing.T) {
	fixture, cleanup := newExamFlowFixture(t)
	defer cleanup()
	seedExamFlowMasterData(t, fixture.database)

	for _, tc := range []struct {
		name  string
		query string
	}{
		{"hasil_tes with unknown peserta", `INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, validasi) VALUES (1, 4242, 1, 'dangling-peserta')`},
		{"cek_login with unknown peserta", `INSERT INTO cek_login (tenant_id, peserta_id, mapel_id) VALUES (1, 4242, 1)`},
		{"peserta with unknown kelas", `INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id) VALUES (1, 'bad', 'hash', 'Siswa', 4242)`},
	} {
		if _, err := fixture.database.Exec(tc.query); err == nil {
			t.Errorf("%s was accepted; FOREIGN KEY enforcement is not active on this connection", tc.name)
		}
	}

	// The sentinel that Gate 0 exists to eliminate must also be rejected now.
	if _, err := fixture.database.Exec(
		`INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (1, 'sentinel', 'hash', 'Siswa', 0, 0)`,
	); err == nil {
		t.Error("a sentinel-0 peserta row was accepted; the Gate 0 / Gate 1 combination is not in effect")
	}

	// And NULL — the representation Gate 0 introduced for "not assigned" — must be accepted.
	if _, err := fixture.database.Exec(
		`INSERT INTO peserta (tenant_id, no_id, password, nama_peserta) VALUES (1, 'unassigned', 'hash', 'Siswa Tanpa Kelas')`,
	); err != nil {
		t.Errorf("a participant with no class or room was rejected: %v", err)
	}
}
