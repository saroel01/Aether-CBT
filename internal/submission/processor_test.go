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
		`CREATE TABLE cek_login (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, peserta_id INTEGER NOT NULL, mapel_id INTEGER NOT NULL, attempt_token TEXT, login_time DATETIME DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE hasil_tes (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, peserta_id INTEGER NOT NULL, mapel_id INTEGER NOT NULL, skor REAL, skor_maks REAL, detail_xml TEXT, status TEXT, validasi TEXT NOT NULL, waktu_selesai DATETIME, UNIQUE(tenant_id, validasi));`,
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

	err := NewProcessor(db).ProcessBatch(context.Background(), []*SubmissionJob{
		processorJob("80", detailXMLWithQuestions()),
	})
	if err != nil {
		t.Fatalf("ProcessBatch: %v", err)
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
	if err := processor.ProcessBatch(context.Background(), []*SubmissionJob{
		processorJob("80", detailXMLWithQuestions()),
	}); err != nil {
		t.Fatalf("first ProcessBatch: %v", err)
	}
	if err := processor.ProcessBatch(context.Background(), []*SubmissionJob{
		processorJob("90", ""),
	}); err != nil {
		t.Fatalf("duplicate ProcessBatch: %v", err)
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

func TestProcessorProcessBatchRollsBackWholeBatch(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	valid := processorJob("80", "")
	invalid := processorJob("90", "")
	invalid.NoID = "missing"
	invalid.Validasi = "1_missing_7"

	err := NewProcessor(db).ProcessBatch(context.Background(), []*SubmissionJob{valid, invalid})
	if err == nil {
		t.Fatal("ProcessBatch returned nil, want error")
	}

	var resultCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes`).Scan(&resultCount); err != nil {
		t.Fatalf("count hasil_tes: %v", err)
	}
	if resultCount != 0 {
		t.Fatalf("hasil_tes rows after rollback = %d, want 0", resultCount)
	}
}
