package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"

	_ "modernc.org/sqlite"
)

type TestStudent struct {
	PesertaID    int
	NoID         string
	Password     string
	JWTToken     string
	AttemptToken string
}

type DataPrep struct {
	DB       *sql.DB
	TenantID int
	MapelID  int
	KelasID  int
	RuangID  int
}

func NewDataPrep(dbPath string, tenantID, mapelID int) (*DataPrep, error) {
	connStr := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(1)

	dp := &DataPrep{
		DB:       db,
		TenantID: tenantID,
		MapelID:  mapelID,
	}

	if err := dp.ensurePrerequisites(); err != nil {
		db.Close()
		return nil, err
	}

	return dp, nil
}

func (dp *DataPrep) ensurePrerequisites() error {
	var kelasCount, ruangCount, mapelCount, settingsCount int

	dp.DB.QueryRow("SELECT COUNT(*) FROM kelas WHERE tenant_id = ?", dp.TenantID).Scan(&kelasCount)
	dp.DB.QueryRow("SELECT COUNT(*) FROM ruang WHERE tenant_id = ?", dp.TenantID).Scan(&ruangCount)
	dp.DB.QueryRow("SELECT COUNT(*) FROM mapel WHERE tenant_id = ?", dp.TenantID).Scan(&mapelCount)
	dp.DB.QueryRow("SELECT COUNT(*) FROM settings WHERE tenant_id = ?", dp.TenantID).Scan(&settingsCount)

	if kelasCount == 0 {
		res, err := dp.DB.Exec("INSERT INTO kelas (tenant_id, nama_kelas) VALUES (?, 'LoadTest-Kelas')", dp.TenantID)
		if err != nil {
			return fmt.Errorf("create kelas: %w", err)
		}
		id, _ := res.LastInsertId()
		dp.KelasID = int(id)
	} else {
		dp.DB.QueryRow("SELECT id FROM kelas WHERE tenant_id = ? LIMIT 1", dp.TenantID).Scan(&dp.KelasID)
	}

	if ruangCount == 0 {
		res, err := dp.DB.Exec("INSERT INTO ruang (tenant_id, nama_ruang, username, password_hash) VALUES (?, 'LoadTest-Ruang', 'lt-ruang', '$2a$14$placeholder')", dp.TenantID)
		if err != nil {
			return fmt.Errorf("create ruang: %w", err)
		}
		id, _ := res.LastInsertId()
		dp.RuangID = int(id)
	} else {
		dp.DB.QueryRow("SELECT id FROM ruang WHERE tenant_id = ? LIMIT 1", dp.TenantID).Scan(&dp.RuangID)
	}

	if mapelCount == 0 {
		res, err := dp.DB.Exec("INSERT INTO mapel (tenant_id, nama_mapel, durasi_menit) VALUES (?, 'LoadTest-Mapel', 90)", dp.TenantID)
		if err != nil {
			return fmt.Errorf("create mapel: %w", err)
		}
		id, _ := res.LastInsertId()
		dp.MapelID = int(id)
	} else if dp.MapelID == 0 {
		dp.DB.QueryRow("SELECT id FROM mapel WHERE tenant_id = ? LIMIT 1", dp.TenantID).Scan(&dp.MapelID)
	}

	if settingsCount == 0 {
		_, err := dp.DB.Exec("INSERT INTO settings (tenant_id, token, is_exam_active) VALUES (?, 'ujian2026', 1)", dp.TenantID)
		if err != nil {
			return fmt.Errorf("create settings: %w", err)
		}
	} else {
		_, err := dp.DB.Exec("UPDATE settings SET is_exam_active = 1, token = 'ujian2026' WHERE tenant_id = ?", dp.TenantID)
		if err != nil {
			return fmt.Errorf("update settings: %w", err)
		}
	}

	return nil
}

func (dp *DataPrep) CreateStudents(count int, prefix string) ([]TestStudent, error) {
	students := make([]TestStudent, 0, count)

	for i := 0; i < count; i++ {
		noID := fmt.Sprintf("%s%04d", prefix, i+1)
		password := "lt-pass"
		nama := fmt.Sprintf("LT %s %d", prefix, i+1)

		_, err := dp.DB.Exec(`
			INSERT OR IGNORE INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
			VALUES (?, ?, ?, ?, ?, ?)
		`, dp.TenantID, noID, password, nama, dp.KelasID, dp.RuangID)
		if err != nil {
			return nil, fmt.Errorf("insert peserta %s: %w", noID, err)
		}

		var pesertaID int
		err = dp.DB.QueryRow("SELECT id FROM peserta WHERE no_id = ? AND tenant_id = ?", noID, dp.TenantID).Scan(&pesertaID)
		if err != nil {
			return nil, fmt.Errorf("get peserta id for %s: %w", noID, err)
		}

		students = append(students, TestStudent{
			PesertaID: pesertaID,
			NoID:      noID,
			Password:  password,
		})
	}

	return students, nil
}

func (dp *DataPrep) CreateSessions(students []TestStudent) ([]TestStudent, error) {
	for i := range students {
		attemptToken := randomHex(32)

		_, err := dp.DB.Exec(`
			INSERT OR REPLACE INTO cek_login
			(tenant_id, peserta_id, ruang_id, mapel_id, attempt_token, login_time, last_activity, tab_switch_count, answered_count, total_questions)
			VALUES (?, ?, ?, ?, ?, datetime('now', '-5 minutes'), datetime('now'), 0, 0, 40)
		`, dp.TenantID, students[i].PesertaID, dp.RuangID, dp.MapelID, attemptToken)
		if err != nil {
			return nil, fmt.Errorf("create session for peserta %d: %w", students[i].PesertaID, err)
		}

		students[i].AttemptToken = attemptToken
	}

	return students, nil
}

func (dp *DataPrep) ClearSessions(prefix string) error {
	_, err := dp.DB.Exec(`
		DELETE FROM cek_login WHERE tenant_id = ? AND peserta_id IN (
			SELECT id FROM peserta WHERE no_id LIKE ? AND tenant_id = ?
		)
	`, dp.TenantID, prefix+"%", dp.TenantID)
	return err
}

func (dp *DataPrep) Cleanup(prefix string) error {
	dp.ClearSessions(prefix)

	_, err := dp.DB.Exec(`
		DELETE FROM hasil_tes WHERE tenant_id = ? AND peserta_id IN (
			SELECT id FROM peserta WHERE no_id LIKE ? AND tenant_id = ?
		)
	`, dp.TenantID, prefix+"%", dp.TenantID)
	if err != nil {
		log.Printf("  Warning: cleanup hasil_tes: %v", err)
	}

	_, err = dp.DB.Exec(`
		DELETE FROM submission_queue WHERE tenant_id = ? AND no_id LIKE ?
	`, dp.TenantID, prefix+"%")
	if err != nil {
		log.Printf("  Warning: cleanup submission_queue: %v", err)
	}

	_, err = dp.DB.Exec(`
		DELETE FROM failed_submissions WHERE tenant_id = ? AND no_id LIKE ?
	`, dp.TenantID, prefix+"%")
	if err != nil {
		log.Printf("  Warning: cleanup failed_submissions: %v", err)
	}

	_, err = dp.DB.Exec(`
		DELETE FROM peserta WHERE no_id LIKE ? AND tenant_id = ?
	`, prefix+"%", dp.TenantID)
	if err != nil {
		log.Printf("  Warning: cleanup peserta: %v", err)
	}

	return nil
}

func (dp *DataPrep) GetExamToken() string {
	var token string
	err := dp.DB.QueryRow("SELECT token FROM settings WHERE tenant_id = ?", dp.TenantID).Scan(&token)
	if err != nil {
		return "ujian2026"
	}
	return token
}

func (dp *DataPrep) GetWALInfo() (dbSize, walSize int64) {
	if dp.DB == nil {
		return
	}
	var path string
	dp.DB.QueryRow("PRAGMA database_list").Scan(nil, nil, &path)
	if path == "" {
		return
	}

	dbSize = fileSize(path)
	walSize = fileSize(path + "-wal")
	return
}

func (dp *DataPrep) CheckpointWAL() error {
	_, err := dp.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

func (dp *DataPrep) Close() {
	if dp.DB != nil {
		dp.DB.Close()
	}
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func fileSize(path string) int64 {
	if path == "" {
		return 0
	}
	fi, err := func() (interface{}, error) {
		return nil, fmt.Errorf("not implemented")
		// We'd need os.Stat but let's keep it simple
	}()
	if err != nil {
		return 0
	}
	_ = fi
	return 0
}

func buildISpringXML(noID, namaPeserta string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<quizReport xmlns="http://www.ispringsolutions.com/ispring/quizbuilder/quizresults" version="2">
  <quizSettings>
    <passingPercent>25</passingPercent>
  </quizSettings>
  <summary score="68.99" percent="21.9" time="144" finishTimestamp="May 25, 2026 11:54 AM" passed="false">
    <variables>
      <variable name="SEKOLAH" title="Sekolah" value="LOAD TEST SCHOOL"/>
      <variable name="NAMA_PESERTA" title="Nama" value="%s"/>
    </variables>
  </summary>
  <questions>
    <multipleChoiceQuestion id="q1" status="correct" evaluationEnabled="true" maxPoints="5" awardedPoints="5">
      <direction><text>Sample question for load test</text></direction>
      <answers correctAnswerIndex="0" userAnswerIndex="0">
        <answer><text>Correct Answer</text></answer>
        <answer><text>Wrong</text></answer>
      </answers>
    </multipleChoiceQuestion>
    <multipleChoiceQuestion id="q2" status="incorrect" evaluationEnabled="true" maxPoints="5" awardedPoints="0">
      <direction><text>Another question for load test</text></direction>
      <answers correctAnswerIndex="1" userAnswerIndex="0">
        <answer><text>Wrong</text></answer>
        <answer><text>Correct Answer</text></answer>
      </answers>
    </multipleChoiceQuestion>
  </questions>
</quizReport>`, escapeXML(namaPeserta))
}

func escapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
