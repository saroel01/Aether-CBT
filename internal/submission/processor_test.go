package submission

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	ispringparser "github.com/saroel01/aether-cbt/internal/ispring"
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
		`CREATE TABLE hasil_tes (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, peserta_id INTEGER NOT NULL, mapel_id INTEGER NOT NULL, exam_session_id INTEGER, skor REAL, skor_maks REAL, score_source TEXT NOT NULL DEFAULT 'client', detail_xml TEXT, status TEXT, validasi TEXT NOT NULL, waktu_selesai DATETIME, UNIQUE(tenant_id, validasi));`,
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
	jobA := processorJob("10", detailXMLWithQuestions())
	if err := processor.Process(context.Background(), jobA); err != nil {
		t.Fatalf("first Process: %v", err)
	}

	// Identical replay: exact same payload must be an idempotent no-op (return nil)
	replayJob := processorJob("10", detailXMLWithQuestions())
	if err := processor.Process(context.Background(), replayJob); err != nil {
		t.Fatalf("identical replay Process failed: %v", err)
	}

	var resultCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes WHERE tenant_id = 1 AND validasi = '1_S-001_7'`).Scan(&resultCount); err != nil {
		t.Fatalf("count hasil_tes: %v", err)
	}
	if resultCount != 1 {
		t.Fatalf("hasil_tes rows = %d, want 1", resultCount)
	}
	var score float64
	if err := db.QueryRow(`SELECT skor FROM hasil_tes WHERE tenant_id = 1 AND validasi = '1_S-001_7'`).Scan(&score); err != nil {
		t.Fatalf("select score: %v", err)
	}
	if score != 10.0 {
		t.Fatalf("score after duplicate = %v, want 10", score)
	}
	var detailCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes_detail`).Scan(&detailCount); err != nil {
		t.Fatalf("count details: %v", err)
	}
	if detailCount != 2 {
		t.Fatalf("detail rows = %d, want 2", detailCount)
	}

	// Overwrite attempt: a different payload must be rejected with an error (P0-2). The replay
	// check compares detail_xml only, since the score is derived from it (audit C1).
	overwriteJob := processorJob("20", strings.Replace(detailXMLWithQuestions(), `awardedPoints="0"`, `awardedPoints="10"`, 1))
	if err := processor.Process(context.Background(), overwriteJob); err == nil {
		t.Fatal("expected error on overwrite attempt of submitted hasil_tes, got nil")
	}

	// Verify hasil_tes and details are completely unmodified
	if err := db.QueryRow(`SELECT skor FROM hasil_tes WHERE tenant_id = 1 AND validasi = '1_S-001_7'`).Scan(&score); err != nil {
		t.Fatalf("select score: %v", err)
	}
	if score != 10.0 {
		t.Fatalf("score was overwritten to %v, want 10.0", score)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes_detail`).Scan(&detailCount); err != nil {
		t.Fatalf("count details: %v", err)
	}
	if detailCount != 2 {
		t.Fatalf("detail rows after overwrite attempt = %d, want 2", detailCount)
	}
}

// TestProcessorRejectsXMLlessSubmission (P0-1 regression test)
func TestProcessorRejectsXMLlessSubmission(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	p := NewProcessor(db)
	job := &SubmissionJob{
		TenantID:     1,
		NoID:         "S-001",
		Score:        "100",
		MaxScore:     "100",
		AttemptToken: "tok",
		Validasi:     "1_S-001_7",
		DetailXML:    "",
	}
	if err := p.Process(context.Background(), job); err == nil {
		t.Fatal("accepted XML-less submission, want error")
	}
}

// TestProcessorDerivesMaxScoreFromXML (P0-1 regression test)
func TestProcessorDerivesMaxScoreFromXML(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	xmlOneQuestion := `<?xml version="1.0" encoding="UTF-8"?>
<quizReport version="1"><questions>
  <multipleChoiceQuestion id="q1" evaluationEnabled="true" maxPoints="1" awardedPoints="1" status="correct">
    <direction><text>1+1?</text></direction>
    <answers correctAnswerIndex="0" userAnswerIndex="0"><answer><text>2</text></answer></answers>
  </multipleChoiceQuestion>
</questions></quizReport>`

	p := NewProcessor(db)
	job := &SubmissionJob{
		TenantID:     1,
		NoID:         "S-001",
		Score:        "1",
		MaxScore:     "1000000", // Client claims 1,000,000 max score
		AttemptToken: "tok",
		Validasi:     "1_S-001_7",
		DetailXML:    xmlOneQuestion,
	}
	if err := p.Process(context.Background(), job); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	var storedMaxScore float64
	if err := db.QueryRow(`SELECT skor_maks FROM hasil_tes WHERE tenant_id = 1 AND validasi = '1_S-001_7'`).Scan(&storedMaxScore); err != nil {
		t.Fatalf("select skor_maks: %v", err)
	}
	if storedMaxScore != 1.0 {
		t.Fatalf("skor_maks = %v, want 1.0 (derived from XML, ignoring client tp)", storedMaxScore)
	}
}

// TestProcessorPreservesEssayGradingOnDuplicateAttempt (P0-2 regression test)
func TestProcessorPreservesEssayGradingOnDuplicateAttempt(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	p := NewProcessor(db)
	jobA := processorJob("10", detailXMLWithQuestions())
	if err := p.Process(context.Background(), jobA); err != nil {
		t.Fatalf("jobA Process: %v", err)
	}

	var hasilTesID int
	if err := db.QueryRow(`SELECT id FROM hasil_tes WHERE tenant_id = 1 AND validasi = '1_S-001_7'`).Scan(&hasilTesID); err != nil {
		t.Fatalf("get hasilTesID: %v", err)
	}

	// Teacher grades an essay question: awards 5 points, updates total score to 15
	if _, err := db.Exec(`UPDATE hasil_tes_detail SET awarded_points = 5 WHERE hasil_tes_id = ? AND question_id = 'q2'`, hasilTesID); err != nil {
		t.Fatalf("grade essay: %v", err)
	}
	if _, err := db.Exec(`UPDATE hasil_tes SET skor = 15.0 WHERE id = ?`, hasilTesID); err != nil {
		t.Fatalf("update hasil_tes score: %v", err)
	}

	// Subsequent student attempt arrives with same Validasi and a different detail XML
	jobB := processorJob("20", strings.Replace(detailXMLWithQuestions(), `awardedPoints="0"`, `awardedPoints="10"`, 1))
	err := p.Process(context.Background(), jobB)
	if err == nil {
		t.Fatal("jobB expected error on duplicate overwrite attempt, got nil")
	}

	// Assert: score was NOT reverted/overwritten, teacher's 15 remains
	var scoreAfter float64
	if err := db.QueryRow(`SELECT skor FROM hasil_tes WHERE id = ?`, hasilTesID).Scan(&scoreAfter); err != nil {
		t.Fatalf("read score: %v", err)
	}
	if scoreAfter != 15.0 {
		t.Fatalf("hasil_tes.skor = %v, want 15.0 (teacher grade preserved)", scoreAfter)
	}

	// Assert: detail count remains exactly 2
	var detailCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes_detail WHERE hasil_tes_id = ?`, hasilTesID).Scan(&detailCount); err != nil {
		t.Fatalf("count details: %v", err)
	}
	if detailCount != 2 {
		t.Fatalf("hasil_tes_detail count = %d, want 2", detailCount)
	}

	// Assert: teacher's essay points (5) was preserved
	var q2Points float64
	if err := db.QueryRow(`SELECT awarded_points FROM hasil_tes_detail WHERE hasil_tes_id = ? AND question_id = 'q2'`, hasilTesID).Scan(&q2Points); err != nil {
		t.Fatalf("read q2 points: %v", err)
	}
	if q2Points != 5.0 {
		t.Fatalf("q2 awarded_points = %v, want 5.0 (teacher grade preserved)", q2Points)
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

	job := processorJob("10", detailXMLWithQuestions()) // AttemptToken = "tok" -> targets only the first session
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

// TestProcessorGradesFromAnswerKey: when the exam's soal_package has an answer key, the score
// comes from the key, not from the client's sp or awardedPoints (audit C1).
func TestProcessorGradesFromAnswerKey(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()
	for _, s := range []string{
		`CREATE TABLE exam_session (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, exam_id INTEGER NOT NULL);`,
		`CREATE TABLE exam (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, mapel_id INTEGER, durasi_menit INTEGER, soal_package_id INTEGER);`,
		`CREATE TABLE soal_package (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, answer_key TEXT);`,
		`INSERT INTO exam_session (id, tenant_id, exam_id) VALUES (3, 1, 4);`,
		`INSERT INTO exam (id, tenant_id, mapel_id, durasi_menit, soal_package_id) VALUES (4, 1, 7, 90, 5);`,
		`UPDATE cek_login SET session_id = 3 WHERE attempt_token = 'tok';`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	key := ispringparser.AnswerKey{Version: 1, Status: ispringparser.AnswerKeyFull, MaxScore: 20, MaxScoreKnown: true,
		Questions: []ispringparser.KeyQuestion{
			{ID: "q1", Type: ispringparser.KeyTypeMultipleChoice, Text: "Question one?", Points: 10, Gradable: true, Choices: []string{"A", "Z"}, Correct: []string{"A"}},
			{ID: "q2", Type: ispringparser.KeyTypeMultipleChoice, Text: "Question two?", Points: 10, Gradable: true, Choices: []string{"B", "Z"}, Correct: []string{"Z"}},
		}}
	raw, _ := json.Marshal(key)
	if _, err := db.Exec(`INSERT INTO soal_package (id, tenant_id, answer_key) VALUES (5, 1, ?)`, string(raw)); err != nil {
		t.Fatalf("seed soal_package: %v", err)
	}

	// Client inflates both sp and awardedPoints: it claims 20/20, but q2's answer "B" is wrong.
	xml := strings.ReplaceAll(detailXMLWithQuestions(), `awardedPoints="0"`, `awardedPoints="10"`)
	job := processorJob("20", xml)
	job.Validasi = ""
	if err := NewProcessor(db).Process(context.Background(), job); err != nil {
		t.Fatalf("Process: %v", err)
	}
	var skor, maks float64
	var source string
	if err := db.QueryRow(`SELECT skor, skor_maks, score_source FROM hasil_tes WHERE validasi = '1_S-001_3'`).Scan(&skor, &maks, &source); err != nil {
		t.Fatalf("select hasil_tes: %v", err)
	}
	if skor != 10 || maks != 20 || source != "server" {
		t.Fatalf("hasil_tes = %v/%v %s, want 10/20 server", skor, maks, source)
	}
	var q2 float64
	if err := db.QueryRow(`SELECT awarded_points FROM hasil_tes_detail WHERE question_id = 'q2'`).Scan(&q2); err != nil {
		t.Fatalf("select detail: %v", err)
	}
	if q2 != 0 {
		t.Fatalf("q2 awarded_points = %v, want 0", q2)
	}
}

// TestProcessorStoresUnmatchedScoreSource (review b): a key whose questions match none of
// the dr questions is persisted as score_source = unmatched.
func TestProcessorStoresUnmatchedScoreSource(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()
	for _, s := range []string{
		`CREATE TABLE exam_session (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, exam_id INTEGER NOT NULL);`,
		`CREATE TABLE exam (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, mapel_id INTEGER, durasi_menit INTEGER, soal_package_id INTEGER);`,
		`CREATE TABLE soal_package (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, answer_key TEXT);`,
		`INSERT INTO exam_session (id, tenant_id, exam_id) VALUES (3, 1, 4);`,
		`INSERT INTO exam (id, tenant_id, mapel_id, durasi_menit, soal_package_id) VALUES (4, 1, 7, 90, 5);`,
		`UPDATE cek_login SET session_id = 3 WHERE attempt_token = 'tok';`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	key := ispringparser.AnswerKey{Version: 1, Status: ispringparser.AnswerKeyFull, MaxScore: 10, MaxScoreKnown: true,
		Questions: []ispringparser.KeyQuestion{
			{ID: "other", Type: ispringparser.KeyTypeMultipleChoice, Text: "Pertanyaan paket lain?", Points: 10, Gradable: true, Choices: []string{"A", "B"}, Correct: []string{"A"}},
		}}
	raw, _ := json.Marshal(key)
	if _, err := db.Exec(`INSERT INTO soal_package (id, tenant_id, answer_key) VALUES (5, 1, ?)`, string(raw)); err != nil {
		t.Fatalf("seed soal_package: %v", err)
	}
	job := processorJob("10", detailXMLWithQuestions())
	job.Validasi = ""
	if err := NewProcessor(db).Process(context.Background(), job); err != nil {
		t.Fatalf("Process: %v", err)
	}
	var skor float64
	var source string
	if err := db.QueryRow(`SELECT skor, score_source FROM hasil_tes WHERE validasi = '1_S-001_3'`).Scan(&skor, &source); err != nil {
		t.Fatalf("select hasil_tes: %v", err)
	}
	if source != ispringparser.ScoreSourceUnmatched || skor != 0 {
		t.Fatalf("hasil_tes = %v %s, want 0 unmatched", skor, source)
	}
}

// TestProcessorWithoutKeyIsClientSource: no session/package -> legacy path, labelled client.
func TestProcessorWithoutKeyIsClientSource(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()
	if err := NewProcessor(db).Process(context.Background(), processorJob("10", detailXMLWithQuestions())); err != nil {
		t.Fatalf("Process: %v", err)
	}
	var source string
	if err := db.QueryRow(`SELECT score_source FROM hasil_tes WHERE validasi = '1_S-001_7'`).Scan(&source); err != nil {
		t.Fatalf("select: %v", err)
	}
	if source != "client" {
		t.Fatalf("score_source = %q, want client", source)
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

	valid := processorJob("10", detailXMLWithQuestions())
	invalid := processorJob("10", detailXMLWithQuestions())
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

func TestProcessor_GracePeriodUsesEnqueuedAt(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	// Student logged in 100 minutes ago for 90m exam (allowed grace: 95m)
	loginTime := time.Now().UTC().Add(-100 * time.Minute)
	if _, err := db.Exec(`UPDATE cek_login SET login_time = ? WHERE tenant_id = 1 AND peserta_id = 42`, loginTime); err != nil {
		t.Fatalf("update login_time: %v", err)
	}

	// Job was enqueued at 80 minutes after login (well within the 95m grace period)
	enqueuedAt := loginTime.Add(80 * time.Minute)

	job := processorJob("10", detailXMLWithQuestions())
	job.EnqueuedAt = enqueuedAt

	p := NewProcessor(db)
	jobErrs, txErr := p.ProcessBatch(context.Background(), []*SubmissionJob{job})
	if txErr != nil {
		t.Fatalf("txErr = %v", txErr)
	}
	if jobErrs[0] != nil {
		t.Fatalf("jobErrs[0] = %v, want nil because job was enqueued within grace period", jobErrs[0])
	}

	// Student 43: Job enqueued at 96 minutes (> 95m grace period) must be rejected
	if _, err := db.Exec(`INSERT INTO peserta (id, tenant_id, no_id) VALUES (43, 1, 'S-002')`); err != nil {
		t.Fatalf("seed peserta 43: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, attempt_token, login_time) VALUES (1, 43, 7, 'tok-late', ?)`, loginTime); err != nil {
		t.Fatalf("seed cek_login 43: %v", err)
	}
	lateJob := processorJob("10", detailXMLWithQuestions())
	lateJob.NoID = "S-002"
	lateJob.AttemptToken = "tok-late"
	lateJob.Validasi = "1_S-002_7"
	lateJob.EnqueuedAt = loginTime.Add(96 * time.Minute)
	lateErrs, _ := p.ProcessBatch(context.Background(), []*SubmissionJob{lateJob})
	if lateErrs[0] == nil || !strings.Contains(lateErrs[0].Error(), "grace period exceeded") {
		t.Fatalf("expected grace period exceeded error, got: %v", lateErrs[0])
	}
}
