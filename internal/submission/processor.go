package submission

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"

	ispringparser "github.com/saroel01/aether-cbt/internal/ispring"
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
	err := tx.QueryRowContext(ctx,
		"SELECT id FROM peserta WHERE no_id = ? AND tenant_id = ?",
		job.NoID, job.TenantID,
	).Scan(&pesertaID)
	if err != nil {
		return fmt.Errorf("peserta not found (no_id=%s, tenant=%d): %w", job.NoID, job.TenantID, err)
	}

	// Step 2: lookup mapel_id and login_time from the SPECIFIC session that owns the
	// attempt_token, so a peserta with multiple active sessions resolves the right one
	// (Requirement 11.3, Task 10.3). Anti-cheat token validation is done in the handler.
	var mapelID sql.NullInt64
	var loginTime time.Time
	var sessionID sql.NullInt64
	requiresGraceCheck := true
	err = tx.QueryRowContext(ctx,
		"SELECT mapel_id, login_time, session_id FROM cek_login WHERE peserta_id = ? AND tenant_id = ? AND attempt_token = ?",
		pesertaID, job.TenantID, job.AttemptToken,
	).Scan(&mapelID, &loginTime, &sessionID)
	if err != nil {
		if err == sql.ErrNoRows && job.Validasi != "" {
			if existingErr := tx.QueryRowContext(ctx,
				"SELECT mapel_id FROM hasil_tes WHERE tenant_id = ? AND validasi = ?",
				job.TenantID, job.Validasi,
			).Scan(&mapelID); existingErr == nil {
				requiresGraceCheck = false
			} else {
				// Diagnostic: count remaining sessions for this peserta and tenant.
				var sameTenant int
				var samePesertaAnyMapel int
				_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE tenant_id = ?", job.TenantID).Scan(&sameTenant)
				_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE peserta_id = ? AND tenant_id = ?", pesertaID, job.TenantID).Scan(&samePesertaAnyMapel)
				log.Printf("[PROCESSOR] cek_login miss: peserta=%d tenant=%d sessions_for_peserta=%d total_sessions_in_tenant=%d sql_err=%v",
					pesertaID, job.TenantID, samePesertaAnyMapel, sameTenant, err)
				return fmt.Errorf("active session not found for peserta %d (tenant %d): %w", pesertaID, job.TenantID, err)
			}
		} else {
			// Diagnostic: count remaining sessions for this peserta and tenant (Clause 2.25: read via tx).
			var sameTenant int
			var samePesertaAnyMapel int
			_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE tenant_id = ?", job.TenantID).Scan(&sameTenant)
			_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE peserta_id = ? AND tenant_id = ?", pesertaID, job.TenantID).Scan(&samePesertaAnyMapel)
			log.Printf("[PROCESSOR] cek_login miss: peserta=%d tenant=%d sessions_for_peserta=%d total_sessions_in_tenant=%d sql_err=%v",
				pesertaID, job.TenantID, samePesertaAnyMapel, sameTenant, err)
			return fmt.Errorf("active session not found for peserta %d (tenant %d): %w", pesertaID, job.TenantID, err)
		}
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

	// Step 3: check grace period (durasi_menit + 5 minutes). The authoritative duration is
	// the EXAM's (via the session), not the mapel's, falling back to mapel then 90 only if the
	// session→exam chain is missing (legacy rows). This prevents a long mapel default from
	// masking a shorter exam duration (review data finding #21, Task 35).
	var durasiMenit int = 90
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(ex.durasi_menit, m.durasi_menit, 90)
		FROM cek_login cl
		LEFT JOIN exam_session es ON es.id = cl.session_id AND es.tenant_id = cl.tenant_id
		LEFT JOIN exam ex ON ex.id = es.exam_id AND ex.tenant_id = es.tenant_id
		LEFT JOIN mapel m ON m.id = cl.mapel_id AND m.tenant_id = cl.tenant_id
		WHERE cl.peserta_id = ? AND cl.tenant_id = ? AND cl.attempt_token = ?
	`, pesertaID, job.TenantID, job.AttemptToken).Scan(&durasiMenit)

	maxAllowedDuration := time.Duration(durasiMenit)*time.Minute + 5*time.Minute
	submissionTime := job.EnqueuedAt
	if submissionTime.IsZero() {
		submissionTime = time.Now()
	}
	actualDuration := submissionTime.UTC().Sub(loginTime.UTC())
	if requiresGraceCheck && actualDuration > maxAllowedDuration {
		return fmt.Errorf("grace period exceeded for peserta %d (mapel %d)", pesertaID, mapelID.Int64)
	}

	// Step 4: parse detail_xml if non-empty.
	var detailReport *ispringparser.Report
	if job.DetailXML != "" {
		detailReport, err = ispringparser.ParseDetailedResults(job.DetailXML)
		if err != nil {
			log.Printf("[PROCESSOR] Invalid iSpring detail XML for job %d: %v", job.ID, err)
			return fmt.Errorf("invalid detail XML: %w", err)
		}
	}

	// Step 4b: verify the client-supplied score against the server-derived score.
	// The webhook is unauthenticated and attempt_token is in the student's hands,
	// so sp/tp cannot be trusted. We derive the score from the parsed XML and reject
	// jobs whose client score diverges beyond a small rounding tolerance.
	if detailReport != nil {
		derivedScore, _ := ispringparser.DerivedScore(detailReport)
		clientScore, parseErr := strconv.ParseFloat(job.Score, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid client score %q: %w", job.Score, parseErr)
		}
		const scoreTolerance = 0.05
		if !ispringparser.ScoresConsistent(clientScore, derivedScore, scoreTolerance) {
			log.Printf("[PROCESSOR] score mismatch: client=%.2f derived=%.2f job_id=%d peserta=%d tenant=%d",
				clientScore, derivedScore, job.ID, pesertaID, job.TenantID)
			return fmt.Errorf("score mismatch (client=%.2f, derived=%.2f) for peserta %d",
				clientScore, derivedScore, pesertaID)
		}
		// Use the derived (authoritative) value for the UPSERT.
		job.Score = strconv.FormatFloat(derivedScore, 'f', 2, 64)
	}

	// Step 5: use Validasi from job (already set by handler as <tenant_id>_<no_id>_<mapel_id>).
	// Fallback: construct it if not set (backward compat).
	validasi := job.Validasi
	if validasi == "" {
		if sessionID.Valid {
			validasi = fmt.Sprintf("%d_%s_%d", job.TenantID, job.NoID, sessionID.Int64)
		} else {
			validasi = fmt.Sprintf("%d_%s_%d", job.TenantID, job.NoID, mapelID.Int64)
		}
	}

	// Step 6: UPSERT hasil_tes using ON CONFLICT(tenant_id, validasi) DO UPDATE.
	// exam_session_id is set so each result is attributable to its wave (Task 37). The replay
	// guard (Task 38) only bumps waktu_selesai when the prior value is NULL or within a
	// 10-minute (600s) admin-replay window, so a stale replay cannot rewrite the finish time.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, exam_session_id, skor, skor_maks, detail_xml, status, validasi, waktu_selesai)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'submitted', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, validasi) DO UPDATE SET
			exam_session_id = COALESCE(excluded.exam_session_id, hasil_tes.exam_session_id),
			skor = excluded.skor,
			skor_maks = excluded.skor_maks,
			detail_xml = excluded.detail_xml,
			status = 'submitted',
			waktu_selesai = CASE
				WHEN hasil_tes.waktu_selesai IS NULL
					OR (julianday(CURRENT_TIMESTAMP) - julianday(hasil_tes.waktu_selesai)) * 86400 < 600
				THEN CURRENT_TIMESTAMP
				ELSE hasil_tes.waktu_selesai
			END
	`, job.TenantID, pesertaID, mapelID, sessionID, job.Score, job.MaxScore, job.DetailXML, validasi)
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
	if detailReport != nil && hasilTesID > 0 {
		for _, q := range detailReport.Questions {
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
