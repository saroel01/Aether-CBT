package submission

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	ispringparser "github.com/saroel01/aether-cbt/internal/ispring"
	"github.com/saroel01/aether-cbt/internal/utils"
)

type Processor struct {
	db *sql.DB
}

func NewProcessor(db *sql.DB) *Processor {
	return &Processor{db: db}
}

// ProcessBatch writes results for the whole batch in ONE transaction and reports the outcome
// PER JOB.
//
// Return contract (klausa 2.4):
//   - jobErrs is parallel to jobs: jobErrs[i] == nil means job i was written and committed;
//     non-nil means only that job failed. It is always len(jobs) long.
//   - txErr is a transaction-level failure (BeginTx, Commit, or a savepoint operation that
//     could not be applied). It covers the ENTIRE batch: no job was committed, so the caller
//     must fail all of them.
//
// Each job runs inside its own SAVEPOINT, so one bad submission is rolled back on its own
// instead of taking the rest of the batch down with it. An all-valid batch is still a single
// BeginTx/Commit — one transaction, one WAL fsync — which is what keeps clause 3.5 intact.
//
// Memenuhi Requirement 5.1, 5.2, 5.3, 5.4, 5.5, 14.1, 14.3, 17.3, 17.4.
func (p *Processor) ProcessBatch(ctx context.Context, jobs []*SubmissionJob) (jobErrs []error, txErr error) {
	jobErrs = make([]error, len(jobs))
	if len(jobs) == 0 {
		return jobErrs, nil
	}

	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return jobErrs, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for i, job := range jobs {
		sp := fmt.Sprintf("sp_job_%d", i)
		if _, err := tx.ExecContext(ctx, "SAVEPOINT "+sp); err != nil {
			return jobErrs, fmt.Errorf("savepoint %s: %w", sp, err)
		}

		if procErr := p.processOneInTx(ctx, tx, job); procErr != nil {
			// Undo just this job, keep everything already written in the batch.
			if _, rbErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+sp); rbErr != nil {
				// The transaction is no longer in a state we can reason about (e.g. the
				// connection died): escalate to a batch-level failure.
				return jobErrs, fmt.Errorf("rollback to savepoint %s after job error %v: %w", sp, procErr, rbErr)
			}
			if _, relErr := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+sp); relErr != nil {
				return jobErrs, fmt.Errorf("release savepoint %s after rollback: %w", sp, relErr)
			}
			jobErrs[i] = procErr
			continue
		}

		if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+sp); err != nil {
			return jobErrs, fmt.Errorf("release savepoint %s: %w", sp, err)
		}
	}

	if err := tx.Commit(); err != nil {
		// Transaction-level failure: nothing was persisted, so per-job outcomes are void.
		return make([]error, len(jobs)), fmt.Errorf("commit batch: %w", err)
	}
	committed = true
	return jobErrs, nil
}

// processOneInTx does NOT validate cek_login token (already done in handler).
// Does: lookup peserta_id+mapel_id, check grace period, parse detail_xml,
// UPSERT hasil_tes, replace hasil_tes_detail, DELETE cek_login.
// All within the passed tx.
func (p *Processor) processOneInTx(ctx context.Context, tx *sql.Tx, job *SubmissionJob) error {
	// Step 1: lookup peserta_id from peserta table using no_id and tenant_id.
	var pesertaID int
	sanitizedNoID := utils.SanitizeFormulaField(job.NoID)
	err := tx.QueryRowContext(ctx,
		"SELECT id FROM peserta WHERE (no_id = ? OR no_id = ?) AND tenant_id = ? LIMIT 1",
		job.NoID, sanitizedNoID, job.TenantID,
	).Scan(&pesertaID)
	if err != nil {
		err = fmt.Errorf("peserta not found (no_id=%s, tenant=%d): %w", job.NoID, job.TenantID, err)
		return permanentIfNoRows(err)
	}

	// P0-2: Immutability check if validasi is already known.
	// If hasil_tes is already 'submitted', an identical replay is an idempotent no-op (return nil),
	// whereas any overwrite (different score/XML/status) is strictly rejected to protect essay grades.
	validasi := job.Validasi
	if validasi != "" {
		var existingID int
		var existingStatus string
		var existingXML sql.NullString
		errHT := tx.QueryRowContext(ctx, `
			SELECT id, status, detail_xml
			FROM hasil_tes
			WHERE tenant_id = ? AND validasi = ?
		`, job.TenantID, validasi).Scan(&existingID, &existingStatus, &existingXML)
		if errHT == nil && existingStatus == "submitted" {
			// The stored score is derived from detail_xml on the server (audit C1), so an
			// identical detail_xml is an identical submission whatever sp the client sent.
			if existingXML.Valid && existingXML.String == job.DetailXML {
				// Exact identical replay: clean up cek_login if present and return nil (idempotent no-op)
				_, _ = tx.ExecContext(ctx, "DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ? AND attempt_token = ?", pesertaID, job.TenantID, job.AttemptToken)
				return nil
			}
			return Permanent(fmt.Errorf("hasil_tes for validasi %s is already submitted; overwrite rejected", validasi))
		}
	}

	// Step 2: lookup mapel_id and login_time from the SPECIFIC session that owns the
	// attempt_token, so a peserta with multiple active sessions resolves the right one.
	var mapelID sql.NullInt64
	var loginTime time.Time
	var sessionID sql.NullInt64
	err = tx.QueryRowContext(ctx,
		"SELECT mapel_id, login_time, session_id FROM cek_login WHERE peserta_id = ? AND tenant_id = ? AND attempt_token = ?",
		pesertaID, job.TenantID, job.AttemptToken,
	).Scan(&mapelID, &loginTime, &sessionID)
	if err != nil {
		// Active session was not found in cek_login.
		var sameTenant int
		var samePesertaAnyMapel int
		_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE tenant_id = ?", job.TenantID).Scan(&sameTenant)
		_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE peserta_id = ? AND tenant_id = ?", pesertaID, job.TenantID).Scan(&samePesertaAnyMapel)
		log.Printf("[PROCESSOR] cek_login miss: peserta=%d tenant=%d sessions_for_peserta=%d total_sessions_in_tenant=%d sql_err=%v",
			pesertaID, job.TenantID, samePesertaAnyMapel, sameTenant, err)
		return permanentIfNoRows(fmt.Errorf("active session not found for peserta %d (tenant %d): %w", pesertaID, job.TenantID, err))
	}

	if !mapelID.Valid && sessionID.Valid {
		var fallbackMapelID int
		if fErr := tx.QueryRowContext(ctx, `
			SELECT e.mapel_id
			FROM exam_session es
			JOIN exam e ON es.exam_id = e.id
			WHERE es.id = ? AND es.tenant_id = ?
		`, sessionID.Int64, job.TenantID).Scan(&fallbackMapelID); fErr == nil {
			mapelID = sql.NullInt64{Int64: int64(fallbackMapelID), Valid: true}
		}
	}

	// Resolve validasi if not set initially
	if validasi == "" {
		if sessionID.Valid {
			validasi = fmt.Sprintf("%d_%s_%d", job.TenantID, job.NoID, sessionID.Int64)
		} else {
			validasi = fmt.Sprintf("%d_%s_%d", job.TenantID, job.NoID, mapelID.Int64)
		}
		var existingID int
		var existingStatus string
		var existingXML sql.NullString
		errHT := tx.QueryRowContext(ctx, `
			SELECT id, status, detail_xml
			FROM hasil_tes
			WHERE tenant_id = ? AND validasi = ?
		`, job.TenantID, validasi).Scan(&existingID, &existingStatus, &existingXML)
		if errHT == nil && existingStatus == "submitted" {
			// The stored score is derived from detail_xml on the server (audit C1), so an
			// identical detail_xml is an identical submission whatever sp the client sent.
			if existingXML.Valid && existingXML.String == job.DetailXML {
				_, _ = tx.ExecContext(ctx, "DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ? AND attempt_token = ?", pesertaID, job.TenantID, job.AttemptToken)
				return nil
			}
			return Permanent(fmt.Errorf("hasil_tes for validasi %s is already submitted; overwrite rejected", validasi))
		}
	}

	// Step 3: check grace period (durasi_menit + 5 minutes) against the ORIGINAL submission
	// time, so retries never push a valid submission out of the window (audit H1, D7).
	if err := CheckGrace(ctx, tx, job.TenantID, job.AttemptToken, job.submissionTime()); err != nil {
		return fmt.Errorf("peserta %d (mapel %d): %w", pesertaID, mapelID.Int64, err)
	}

	// Step 4: parse detail_xml - mandatory (P0-1).
	if strings.TrimSpace(job.DetailXML) == "" {
		return Permanent(fmt.Errorf("detail XML is required"))
	}
	detailReport, err := ispringparser.ParseDetailedResults(job.DetailXML)
	if err != nil {
		log.Printf("[PROCESSOR] Invalid iSpring detail XML for job %d: %v", job.ID, err)
		return Permanent(fmt.Errorf("invalid detail XML: %w", err))
	}
	if detailReport == nil {
		return Permanent(fmt.Errorf("detail report is nil"))
	}

	// Step 4b: grade on the server from the package answer key when one exists (audit C1).
	// sp/awardedPoints from the client are then ignored for every gradable question.
	key, err := loadAnswerKey(ctx, tx, job.TenantID, sessionID)
	if err != nil {
		return err
	}
	var graded ispringparser.GradeResult
	if key != nil {
		graded = ispringparser.Grade(key, detailReport)
		if graded.MaxScore <= 0 {
			return fmt.Errorf("answer key max score must be positive, got %.2f", graded.MaxScore)
		}
		if graded.Source == ispringparser.ScoreSourceUnmatched {
			log.Printf("[GRADE] tenant=%d no_id=%s validasi=%s: answer key present but no dr question matched (%d questions)",
				job.TenantID, job.NoID, job.Validasi, len(detailReport.Questions))
		}
	} else {
		// No key (legacy package or session-less attempt): keep the client-derived score,
		// cross-checked against sp, and label it as client-reported (D5).
		derivedScore, derivedMax := ispringparser.DerivedScore(detailReport)
		if derivedMax <= 0 {
			return fmt.Errorf("derived max score must be positive, got %.2f", derivedMax)
		}
		clientScore, parseErr := strconv.ParseFloat(job.Score, 64)
		if parseErr != nil {
			return Permanent(fmt.Errorf("invalid client score %q: %w", job.Score, parseErr))
		}
		const scoreTolerance = 0.05
		if !ispringparser.ScoresConsistent(clientScore, derivedScore, scoreTolerance) {
			log.Printf("[PROCESSOR] score mismatch: client=%.2f derived=%.2f job_id=%d peserta=%d tenant=%d",
				clientScore, derivedScore, job.ID, pesertaID, job.TenantID)
			return Permanent(fmt.Errorf("score mismatch (client=%.2f, derived=%.2f) for peserta %d",
				clientScore, derivedScore, pesertaID))
		}
		graded = ispringparser.GradeResult{
			Score: derivedScore, MaxScore: derivedMax,
			Source: ispringparser.ScoreSourceClient, Questions: detailReport.Questions,
		}
	}

	// Authoritative server-side scores for UPSERT (ignore client-controlled MaxScore)
	job.Score = strconv.FormatFloat(graded.Score, 'f', 2, 64)
	job.MaxScore = strconv.FormatFloat(graded.MaxScore, 'f', 2, 64)

	// Step 6: UPSERT hasil_tes using ON CONFLICT(tenant_id, validasi) DO UPDATE.
	// exam_session_id is set so each result is attributable to its wave (Task 37). The replay
	// guard (Task 38) only bumps waktu_selesai when the prior value is NULL or within a
	// 10-minute (600s) admin-replay window, so a stale replay cannot rewrite the finish time.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, exam_session_id, skor, skor_maks, score_source, detail_xml, status, validasi, waktu_selesai)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'submitted', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, validasi) DO UPDATE SET
			exam_session_id = COALESCE(excluded.exam_session_id, hasil_tes.exam_session_id),
			skor = excluded.skor,
			skor_maks = excluded.skor_maks,
			score_source = excluded.score_source,
			detail_xml = excluded.detail_xml,
			status = 'submitted',
			waktu_selesai = CASE
				WHEN hasil_tes.waktu_selesai IS NULL
					OR (julianday(CURRENT_TIMESTAMP) - julianday(hasil_tes.waktu_selesai)) * 86400 < 600
				THEN CURRENT_TIMESTAMP
				ELSE hasil_tes.waktu_selesai
			END
	`, job.TenantID, pesertaID, mapelID, sessionID, job.Score, job.MaxScore, graded.Source, job.DetailXML, validasi)
	if err != nil {
		return fmt.Errorf("upsert hasil_tes: %w", err)
	}

	// Step 7: get the hasil_tes_id after INSERT or UPSERT update.
	var hasilTesID int
	err = tx.QueryRowContext(ctx,
		"SELECT id FROM hasil_tes WHERE tenant_id = ? AND validasi = ?",
		job.TenantID, validasi,
	).Scan(&hasilTesID)
	if err != nil {
		return fmt.Errorf("select hasil_tes id after upsert: %w", err)
	}

	// Step 8: DELETE existing hasil_tes_detail for that hasil_tes_id (replace strategy, Req 14.3).
	if _, err = tx.ExecContext(ctx,
		"DELETE FROM hasil_tes_detail WHERE hasil_tes_id = ?", hasilTesID,
	); err != nil {
		return fmt.Errorf("delete hasil_tes_detail: %w", err)
	}

	// Step 9: INSERT N new hasil_tes_detail rows.
	if hasilTesID > 0 {
		for _, q := range graded.Questions {
			questionText := q.Text
			if questionText == "" {
				questionText = "Teks soal tidak tersedia"
			}
			if _, err = tx.ExecContext(ctx, `
				INSERT INTO hasil_tes_detail (
					hasil_tes_id, question_id, question_text, question_type,
					status, awarded_points, max_points, user_answer, correct_answer
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, hasilTesID, q.ID, questionText, q.Type, q.Status,
				q.AwardedPoints, q.MaxPoints, q.UserAnswer, q.CorrectAnswer,
			); err != nil {
				return fmt.Errorf("insert hasil_tes_detail (question_id=%s): %w", q.ID, err)
			}
		}
	}

	// Step 10: DELETE only the cek_login row for THIS attempt_token, leaving any other
	// active session for the peserta intact (Requirement 11.3, Task 10.3).
	if _, err = tx.ExecContext(ctx,
		"DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ? AND attempt_token = ?",
		pesertaID, job.TenantID, job.AttemptToken,
	); err != nil {
		return fmt.Errorf("delete cek_login: %w", err)
	}

	return nil
}

// loadAnswerKey returns the answer key of the soal_package behind the attempt's exam session,
// or nil when the attempt has no session, the exam has no package, or the package has no
// stored key (audit C1, D1). A corrupt key is treated as absent so the result is still
// recorded, labelled as client-reported.
func loadAnswerKey(ctx context.Context, tx *sql.Tx, tenantID int, sessionID sql.NullInt64) (*ispringparser.AnswerKey, error) {
	if !sessionID.Valid {
		return nil, nil
	}
	var raw sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT sp.answer_key
		FROM exam_session es
		JOIN exam e ON e.id = es.exam_id AND e.tenant_id = es.tenant_id
		JOIN soal_package sp ON sp.id = e.soal_package_id AND sp.tenant_id = e.tenant_id
		WHERE es.id = ? AND es.tenant_id = ?
	`, sessionID.Int64, tenantID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load answer key (session %d): %w", sessionID.Int64, err)
	}
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var key ispringparser.AnswerKey
	if err := json.Unmarshal([]byte(raw.String), &key); err != nil || len(key.Questions) == 0 {
		log.Printf("[PROCESSOR] unusable answer key for session %d: %v", sessionID.Int64, err)
		return nil, nil
	}
	return &key, nil
}

// Process is a convenience wrapper around ProcessBatch for single-job callers: with a batch of
// one, the per-job error and the transaction-level error collapse into a single error.
func (p *Processor) Process(ctx context.Context, job *SubmissionJob) error {
	jobErrs, txErr := p.ProcessBatch(ctx, []*SubmissionJob{job})
	if txErr != nil {
		return txErr
	}
	if len(jobErrs) > 0 {
		return jobErrs[0]
	}
	return nil
}
