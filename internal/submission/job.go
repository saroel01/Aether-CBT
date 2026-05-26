package submission

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// JobStatus mendefinisikan status pemrosesan job.
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// SubmissionJob merepresentasikan satu permintaan webhook iSpring yang harus diproses.
//
// Field yang ditulis ke Job_File (urutan sesuai Requirement 10.1):
//
//	validasi, tenant_id, no_id, score, max_score, attempt_token,
//	enqueued_at, retry_count, last_error, detail_xml
//
// Field internal (tidak di-serialize ke Job_File, ditandai json:"-"):
//
//	ID, Status, CreatedAt, UpdatedAt, NextRetryAt
type SubmissionJob struct {
	// Field Job_File — urutan tetap sesuai Requirement 10.1
	Validasi     string    `json:"validasi"`      // tenant_noID_mapel (untuk idempotency)
	TenantID     int       `json:"tenant_id"`
	NoID         string    `json:"no_id"`         // sid / USER_NAME dari iSpring
	Score        string    `json:"score"`         // sp
	MaxScore     string    `json:"max_score"`     // tp
	AttemptToken string    `json:"attempt_token"`
	EnqueuedAt   time.Time `json:"enqueued_at"`
	RetryCount   int       `json:"retry_count"`
	LastError    string    `json:"last_error"`
	DetailXML    string    `json:"detail_xml"`    // dr

	// Field internal — tidak di-serialize ke Job_File
	ID          int64     `json:"-"`
	Status      string    `json:"-"`
	CreatedAt   time.Time `json:"-"`
	UpdatedAt   time.Time `json:"-"`
	NextRetryAt time.Time `json:"-"`

	// fileName diisi saat Dequeue; dipakai oleh MarkCompleted/MarkFailed
	// untuk menemukan file di processing/ tanpa scan ulang.
	fileName string
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

// MarshalJob menghasilkan JSON pretty-printed (indent 2 spasi) dengan urutan field
// tetap sesuai Requirement 10.1. Memenuhi Requirement 11.1, 11.5.
func MarshalJob(job *SubmissionJob) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(job); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalJob mem-parse byte JSON menjadi SubmissionJob. Memvalidasi field wajib
// (tenant_id != 0, no_id != "", validasi != "", enqueued_at != zero).
// Memenuhi Requirement 11.2, 11.3.
func UnmarshalJob(data []byte) (*SubmissionJob, error) {
	var job SubmissionJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	if job.TenantID == 0 {
		return nil, errors.New("missing required field: tenant_id")
	}
	if job.NoID == "" {
		return nil, errors.New("missing required field: no_id")
	}
	if job.Validasi == "" {
		return nil, errors.New("missing required field: validasi")
	}
	if job.EnqueuedAt.IsZero() {
		return nil, errors.New("missing required field: enqueued_at")
	}
	return &job, nil
}
