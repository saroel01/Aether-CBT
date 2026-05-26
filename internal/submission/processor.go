package submission

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	ispringparser "github.com/saroel01/aether-cbt/internal/ispring"
)

type Processor struct {
	db *sql.DB
}

func NewProcessor(db *sql.DB) *Processor {
	return &Processor{db: db}
}

// ProcessBatch writes results for the entire batch in one transaction.
// Memenuhi Requirement 5.1, 5.2, 5.3, 5.4, 5.5, 14.1, 14.3, 17.3, 17.4.
func (p *Processor) ProcessBatch(ctx context.Context, jobs []*SubmissionJob) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, job := range jobs {
		if err := p.processOneInTx(ctx, tx, job); err != nil {
			return err // rollback via defer
		}
	}
	return tx.Commit()
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

	// Step 2: lookup mapel_id and login_time from cek_login using peserta_id and tenant_id.
	// Note: anti-cheat token validation is already done in the handler (Requirement 4).
	// We still need mapel_id for the UPSERT and login_time for grace period check.
	var mapelID int
	var loginTime time.Time
	requiresGraceCheck := true
	err = tx.QueryRowContext(ctx,
		"SELECT mapel_id, login_time FROM cek_login WHERE peserta_id = ? AND tenant_id = ?",
		pesertaID, job.TenantID,
	).Scan(&mapelID, &loginTime)
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
				_ = p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE tenant_id = ?", job.TenantID).Scan(&sameTenant)
				_ = p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE peserta_id = ? AND tenant_id = ?", pesertaID, job.TenantID).Scan(&samePesertaAnyMapel)
				log.Printf("[PROCESSOR] cek_login miss: peserta=%d tenant=%d sessions_for_peserta=%d total_sessions_in_tenant=%d sql_err=%v",
					pesertaID, job.TenantID, samePesertaAnyMapel, sameTenant, err)
				return fmt.Errorf("active session not found for peserta %d (tenant %d): %w", pesertaID, job.TenantID, err)
			}
		} else {
			// Diagnostic: count remaining sessions for this peserta and tenant.
			var sameTenant int
			var samePesertaAnyMapel int
			_ = p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE tenant_id = ?", job.TenantID).Scan(&sameTenant)
			_ = p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cek_login WHERE peserta_id = ? AND tenant_id = ?", pesertaID, job.TenantID).Scan(&samePesertaAnyMapel)
			log.Printf("[PROCESSOR] cek_login miss: peserta=%d tenant=%d sessions_for_peserta=%d total_sessions_in_tenant=%d sql_err=%v",
				pesertaID, job.TenantID, samePesertaAnyMapel, sameTenant, err)
			return fmt.Errorf("active session not found for peserta %d (tenant %d): %w", pesertaID, job.TenantID, err)
		}
	}

	// Step 3: check grace period (durasi_menit + 5 minutes).
	var durasiMenit int = 90
	_ = tx.QueryRowContext(ctx,
		"SELECT COALESCE(durasi_menit, 90) FROM mapel WHERE id = ? AND tenant_id = ?",
		mapelID, job.TenantID,
	).Scan(&durasiMenit)

	maxAllowedDuration := time.Duration(durasiMenit)*time.Minute + 5*time.Minute
	actualDuration := time.Now().UTC().Sub(loginTime.UTC())
	if requiresGraceCheck && actualDuration > maxAllowedDuration {
		return fmt.Errorf("grace period exceeded for peserta %d (mapel %d)", pesertaID, mapelID)
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

	// Step 5: use Validasi from job (already set by handler as <tenant_id>_<no_id>_<mapel_id>).
	// Fallback: construct it if not set (backward compat).
	validasi := job.Validasi
	if validasi == "" {
		validasi = fmt.Sprintf("%d_%s_%d", job.TenantID, job.NoID, mapelID)
	}

	// Step 6: UPSERT hasil_tes using ON CONFLICT(tenant_id, validasi) DO UPDATE.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, skor, skor_maks, detail_xml, status, validasi, waktu_selesai)
		VALUES (?, ?, ?, ?, ?, ?, 'submitted', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, validasi) DO UPDATE SET
			skor = excluded.skor,
			skor_maks = excluded.skor_maks,
			detail_xml = excluded.detail_xml,
			status = 'submitted',
			waktu_selesai = CURRENT_TIMESTAMP
	`, job.TenantID, pesertaID, mapelID, job.Score, job.MaxScore, job.DetailXML, validasi)
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

	// Step 10: DELETE cek_login for that peserta_id and tenant_id.
	if _, err = tx.ExecContext(ctx,
		"DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ?",
		pesertaID, job.TenantID,
	); err != nil {
		return fmt.Errorf("delete cek_login: %w", err)
	}

	return nil
}

// Process is a backward-compatibility wrapper around ProcessBatch for single-job callers.
// Deprecated: use ProcessBatch directly.
func (p *Processor) Process(ctx context.Context, job *SubmissionJob) error {
	return p.ProcessBatch(ctx, []*SubmissionJob{job})
}
