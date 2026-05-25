package submission

import "time"

// JobStatus mendefinisikan status pemrosesan job.
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// SubmissionJob merepresentasikan satu permintaan webhook iSpring yang harus diproses.
type SubmissionJob struct {
	ID           int64     `json:"id"`
	TenantID     int       `json:"tenant_id"`
	NoID         string    `json:"no_id"`      // sid / USER_NAME dari iSpring
	Score        string    `json:"score"`      // sp
	MaxScore     string    `json:"max_score"`  // tp
	DetailXML    string    `json:"detail_xml"` // dr
	AttemptToken string    `json:"attempt_token"`
	Validasi     string    `json:"validasi"` // tenant_noID_mapel (untuk idempotency)
	RetryCount   int       `json:"retry_count"`
	LastError    string    `json:"last_error"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	NextRetryAt  time.Time `json:"next_retry_at"`
}

// FailedSubmission adalah job yang sudah gagal melebihi max retry.
type FailedSubmission struct {
	ID            int64     `json:"id"`
	OriginalJobID int64     `json:"original_job_id"`
	TenantID      int       `json:"tenant_id"`
	NoID          string    `json:"no_id"`
	ErrorMessage  string    `json:"error_message"`
	DetailXML     string    `json:"detail_xml"` // simpan untuk investigasi manual
	CreatedAt     time.Time `json:"created_at"`
}
