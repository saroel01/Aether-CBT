package submission

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupProcessorDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	schemas := []string{
		`CREATE TABLE peserta (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, no_id TEXT NOT NULL, password TEXT, nama_peserta TEXT, kelas_id INTEGER, ruang_id INTEGER);`,
		`CREATE TABLE mapel (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, nama_mapel TEXT, durasi_menit INTEGER DEFAULT 90);`,
		`CREATE TABLE cek_login (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, peserta_id INTEGER NOT NULL, mapel_id INTEGER NOT NULL, session_id INTEGER, attempt_token TEXT, login_time DATETIME DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE hasil_tes (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, peserta_id INTEGER NOT NULL, mapel_id INTEGER NOT NULL, exam_session_id INTEGER, skor REAL, skor_maks REAL, detail_xml TEXT, status TEXT, validasi TEXT NOT NULL, waktu_selesai DATETIME, UNIQUE(tenant_id, validasi));`,
		`CREATE TABLE hasil_tes_detail (id INTEGER PRIMARY KEY AUTOINCREMENT, hasil_tes_id INTEGER NOT NULL, question_id TEXT NOT NULL, question_text TEXT, question_type TEXT, status TEXT, awarded_points REAL, max_points REAL, user_answer TEXT, correct_answer TEXT);`,
	}
	for _, schema := range schemas {
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO peserta (id, tenant_id, no_id) VALUES (42, 1, 'S-001')`); err != nil {
		t.Fatalf("seed peserta: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO mapel (id, tenant_id, nama_mapel, durasi_menit) VALUES (7, 1, 'Matematika', 90)`); err != nil {
		t.Fatalf("seed mapel: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, attempt_token, login_time) VALUES (1, 42, 7, 'tok', ?)`, time.Now().UTC().Add(-5*time.Minute)); err != nil {
		t.Fatalf("seed cek_login: %v", err)
	}
	return db
}

func detailXMLWithQuestions() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<quizReport version="1">
  <questions>
    <multipleChoiceQuestion id="q1" evaluationEnabled="true" maxPoints="10" awardedPoints="10" status="correct">
      <direction><text>Question one?</text></direction>
      <answers correctAnswerIndex="0" userAnswerIndex="0"><answer><text>A</text></answer></answers>
    </multipleChoiceQuestion>
    <multipleChoiceQuestion id="q2" evaluationEnabled="true" maxPoints="10" awardedPoints="0" status="incorrect">
      <direction><text>Question two?</text></direction>
      <answers correctAnswerIndex="0" userAnswerIndex="0"><answer><text>B</text></answer></answers>
    </multipleChoiceQuestion>
  </questions>
</quizReport>`
}

func processorJob(score, xml string) *SubmissionJob {
	return &SubmissionJob{
		TenantID:     1,
		NoID:         "S-001",
		Validasi:     "1_S-001_7",
		Score:        score,
		MaxScore:     "100",
		AttemptToken: "tok",
		DetailXML:    xml,
	}
}

func TestProcessorProcessBatchInsertsDetailRows(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	err := NewProcessor(db).Process(context.Background(),
		processorJob("10", detailXMLWithQuestions()), // client score matches XML-derived 10 (Task 2)
	)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	var detailCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes_detail`).Scan(&detailCount); err != nil {
		t.Fatalf("count details: %v", err)
	}
	if detailCount != 2 {
		t.Fatalf("detail rows = %d, want 2", detailCount)
	}
}

func TestProcessorProcessBatchIsIdempotentForDuplicateValidasi(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	processor := NewProcessor(db)
	if err := processor.Process(context.Background(),
		processorJob("10", detailXMLWithQuestions()), // client score matches XML-derived 10 (Task 2)
	); err != nil {
		t.Fatalf("first Process: %v", err)
	}
	if err := processor.Process(context.Background(), processorJob("90", "")); err != nil {
		t.Fatalf("duplicate Process: %v", err)
	}

	var resultCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes WHERE tenant_id = 1 AND validasi = '1_S-001_7'`).Scan(&resultCount); err != nil {
		t.Fatalf("count hasil_tes: %v", err)
	}
	if resultCount != 1 {
		t.Fatalf("hasil_tes rows = %d, want 1", resultCount)
	}
	var score string
	if err := db.QueryRow(`SELECT CAST(skor AS TEXT) FROM hasil_tes WHERE tenant_id = 1 AND validasi = '1_S-001_7'`).Scan(&score); err != nil {
		t.Fatalf("select score: %v", err)
	}
	if !strings.HasPrefix(score, "90") {
		t.Fatalf("score after duplicate = %q, want 90", score)
	}
	var detailCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes_detail`).Scan(&detailCount); err != nil {
		t.Fatalf("count details: %v", err)
	}
	if detailCount != 0 {
		t.Fatalf("detail rows after duplicate replacement = %d, want 0", detailCount)
	}
}

// TestProcessorCleanupTargetsOnlyTheSubmittedSession verifies that cek_login cleanup after a
// result is processed removes ONLY the session that owns the submitted attempt_token, leaving
// any other active session for the same peserta intact (Requirement 11.3, Task 10.3).
func TestProcessorCleanupTargetsOnlyTheSubmittedSession(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	// A second active session for the same peserta with a different attempt_token.
	if _, err := db.Exec(`INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, attempt_token, login_time) VALUES (1, 42, 7, 'tok-other', ?)`, time.Now().UTC().Add(-5*time.Minute)); err != nil {
		t.Fatalf("seed second cek_login: %v", err)
	}

	job := processorJob("80", "") // AttemptToken = "tok" -> targets only the first session
	if err := NewProcessor(db).Process(context.Background(), job); err != nil {
		t.Fatalf("Process: %v", err)
	}

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cek_login WHERE peserta_id = 42 AND tenant_id = 1 AND attempt_token = 'tok-other'`).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("second session was also deleted; remaining=%d, want 1 (cleanup must target only the submitted session)", remaining)
	}
}

// TestProcessorRejectsInflatedClientScore verifies that the processor rejects a
// client-supplied score that diverges from the server-derived score parsed from the
// iSpring detail XML. The webhook is unauthenticated and attempt_token is in the
// student's hands, so sp/tp cannot be trusted (review Critical #1, Task 2).
func TestProcessorRejectsInflatedClientScore(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	// XML where the student actually scored 2/8.
	xml := `<quizReport version="1"><questions>` +
		`<multipleChoiceQuestion id="Q1" evaluationEnabled="true" awardedPoints="2" maxPoints="8" status="incorrect">` +
		`<direction><text>q</text></direction>` +
		`<answers correctAnswerIndex="1" userAnswerIndex="0"><answer correct="false"><text>a</text></answer><answer correct="true"><text>b</text></answer></answers>` +
		`</multipleChoiceQuestion>` +
		`</questions></quizReport>`

	job := processorJob("8", xml) // client claims 8 — TAMPERED (derived is 2)
	err := NewProcessor(db).Process(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for inflated client score, got nil")
	}
	if !strings.Contains(err.Error(), "score mismatch") {
		t.Fatalf("expected 'score mismatch' error, got: %v", err)
	}
}

// TestProcessorProcessBatchIsolatesFailingJob replaces the former
// TestProcessorProcessBatchRollsBackWholeBatch, which asserted the defect described by clause
// 1.4: one bad job rolled back the entire transaction, and the worker then dead-lettered every
// valid submission in the same batch. Per clause 2.4 a failure is now confined to the job that
// caused it — the valid job in the same batch commits and only the invalid one reports an error.
func TestProcessorProcessBatchIsolatesFailingJob(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	valid := processorJob("80", "")
	invalid := processorJob("90", "")
	invalid.NoID = "missing" // no such peserta -> per-job failure
	invalid.Validasi = "1_missing_7"

	jobErrs, txErr := NewProcessor(db).ProcessBatch(context.Background(), []*SubmissionJob{valid, invalid})
	if txErr != nil {
		t.Fatalf("txErr = %v, want nil (a single bad job is not a transaction-level failure)", txErr)
	}
	if len(jobErrs) != 2 {
		t.Fatalf("len(jobErrs) = %d, want 2 (parallel to jobs)", len(jobErrs))
	}
	if jobErrs[0] != nil {
		t.Fatalf("jobErrs[0] = %v, want nil (the valid job must succeed)", jobErrs[0])
	}
	if jobErrs[1] == nil {
		t.Fatal("jobErrs[1] = nil, want an error for the job whose peserta does not exist")
	}

	var resultCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes`).Scan(&resultCount); err != nil {
		t.Fatalf("count hasil_tes: %v", err)
	}
	if resultCount != 1 {
		t.Fatalf("hasil_tes rows = %d, want 1 (only the valid job is persisted)", resultCount)
	}
	var validasi string
	if err := db.QueryRow(`SELECT validasi FROM hasil_tes`).Scan(&validasi); err != nil {
		t.Fatalf("select validasi: %v", err)
	}
	if validasi != "1_S-001_7" {
		t.Fatalf("persisted validasi = %q, want the valid job's 1_S-001_7", validasi)
	}
}
