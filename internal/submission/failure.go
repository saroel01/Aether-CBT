package submission

import (
	"database/sql"
	"fmt"
	"log"
)

// RecordFailure writes a dead-lettered job to submission_failure so staff can see it
// (audit H1, D7). It is used as FilesystemQueueConfig.OnDeadLetter; the job file itself
// stays in failed/ for manual replay.
func RecordFailure(db *sql.DB, job *SubmissionJob) error {
	var submittedAt any
	if t := job.submissionTime(); !t.IsZero() {
		submittedAt = t.UTC()
	}
	if _, err := db.Exec(`
		INSERT INTO submission_failure (tenant_id, no_id, validasi, attempt_token, error_message, submitted_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, job.TenantID, job.NoID, job.Validasi, job.AttemptToken, job.LastError, submittedAt); err != nil {
		return fmt.Errorf("record submission failure (no_id=%s, tenant=%d): %w", job.NoID, job.TenantID, err)
	}
	return nil
}

// DeadLetterRecorder adapts RecordFailure to the OnDeadLetter hook, logging write errors
// because the hook has no caller to return them to.
func DeadLetterRecorder(db *sql.DB) func(job *SubmissionJob) {
	return func(job *SubmissionJob) {
		if err := RecordFailure(db, job); err != nil {
			log.Printf("[QUEUE] %v", err)
		}
	}
}
