package submission

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Queue is the interface for the submission processing queue (webhook -> worker).
//
// Dequeue contract:
//   - (job, nil)   : job dequeued successfully
//   - (nil, err)   : ctx cancelled / deadline while waiting
//   - (nil, nil)   : no job available right now (non-blocking for worker poll loop)
//
// MarkCompleted and MarkFailed are fire-and-forget. Implementations are not required
// to guarantee that every dequeued job will have a matching Mark* call (e.g. process
// panic, worker restart). The jobID/err parameters may be ignored by some impls.
type Queue interface {
	Enqueue(ctx context.Context, job *SubmissionJob) error
	Dequeue(ctx context.Context) (*SubmissionJob, error)
	MarkCompleted(ctx context.Context, jobID int64) error
	MarkFailed(ctx context.Context, jobID int64, err error) error
	GetStats(ctx context.Context) (QueueStats, error)
}

// QueueStats contains approximate counters for queue monitoring (e.g. /debug/queue).
// For InMemoryQueue these are best-effort only (see type docs).
type QueueStats struct {
	PendingCount    int `json:"pending_count"`
	ProcessingCount int `json:"processing_count"`
	FailedCount     int `json:"failed_count"`
	DoneCount       int `json:"done_count"`
}

// InMemoryQueue is a simple in-memory Queue implementation for Phase 1 PoC / fast validation.
//
// It is intentionally ephemeral (no restart survival) and uses best-effort statistics.
//
// Critical notes for this implementation:
//   - Stats returned by GetStats are approximate. There is a non-zero (but tiny) window
//     between successful channel send/recv and the corresponding counter mutation.
//     PendingCount is corrected at read time via len(jobs) to reduce staleness.
//   - MarkCompleted and MarkFailed are pure heuristics that adjust aggregate counters.
//     They do NOT validate jobID (InMemoryQueue keeps no job registry or map by ID;
//     jobs only exist inside the buffered channel until dequeued). The err param is
//     discarded. These methods are fire-and-forget and always return nil.
//   - This is ONLY for validating the overall queue+worker pattern quickly in Fase 1.
//     Production use must wait for SQLiteQueue (Task 4+).
//
// Use with bufferSize >= expected max concurrent webhook submissions.
type InMemoryQueue struct {
	jobs  chan *SubmissionJob
	stats QueueStats
	mu    sync.RWMutex
}

func NewInMemoryQueue(bufferSize int) *InMemoryQueue {
	return &InMemoryQueue{
		jobs: make(chan *SubmissionJob, bufferSize),
	}
}

// Enqueue submits a job to the queue (non-blocking send if buffer has space).
// On success the pending counter is incremented (best-effort).
func (q *InMemoryQueue) Enqueue(ctx context.Context, job *SubmissionJob) error {
	select {
	case q.jobs <- job:
		q.mu.Lock()
		q.stats.PendingCount++
		q.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dequeue retrieves the next pending job for processing.
//
// It never blocks on an empty queue: the default case returns (nil, nil) immediately.
// This design lets the Worker loop do a cheap non-blocking check, then sleep briefly,
// while still respecting ctx cancellation for graceful shutdown.
//
// See Queue interface godoc for the full (nil, nil) contract.
func (q *InMemoryQueue) Dequeue(ctx context.Context) (*SubmissionJob, error) {
	select {
	case job := <-q.jobs:
		q.mu.Lock()
		q.stats.PendingCount = max(0, q.stats.PendingCount-1)
		q.stats.ProcessingCount++
		q.mu.Unlock()
		return job, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, nil
	}
}

// MarkCompleted is a fire-and-forget signal that processing of a job succeeded.
//
// InMemoryQueue implementation: blindly decrements ProcessingCount (clamped at 0).
// The jobID parameter is accepted for interface compatibility but is ignored —
// this implementation does not track which specific jobs are in-flight.
// It is safe to call with any int64 (or even with IDs never enqueued).
func (q *InMemoryQueue) MarkCompleted(ctx context.Context, jobID int64) error {
	_ = ctx // accepted for signature parity with durable impls; unused here
	_ = jobID
	q.mu.Lock()
	q.stats.ProcessingCount = max(0, q.stats.ProcessingCount-1)
	q.mu.Unlock()
	return nil
}

// MarkFailed is a fire-and-forget signal that a job has failed permanently.
//
// InMemoryQueue: decrements ProcessingCount and increments the lifetime FailedCount.
// Both jobID and err are ignored (no per-job state or error persistence in the PoC).
// ctx is accepted but the operation is synchronous and non-blocking.
func (q *InMemoryQueue) MarkFailed(ctx context.Context, jobID int64, err error) error {
	_ = ctx
	_ = jobID
	_ = err
	q.mu.Lock()
	q.stats.ProcessingCount = max(0, q.stats.ProcessingCount-1)
	q.stats.FailedCount++
	q.mu.Unlock()
	return nil
}

// GetStats returns a point-in-time snapshot of queue counters.
//
// For InMemoryQueue:
//   - PendingCount is always set to the current len(jobs) channel (accurate, no
//     staleness from the send/recv vs counter timing window).
//   - ProcessingCount and FailedCount come from the protected counter fields and
//     are best-effort (may be off by a few under heavy concurrent Mark/Dequeue).
//
// This is sufficient for the /debug/queue admin endpoint during Fase 1 validation.
// The implementation deliberately avoids heavier synchronization to keep the
// hot path (Enqueue/Dequeue) as fast and simple as possible.
func (q *InMemoryQueue) GetStats(ctx context.Context) (QueueStats, error) {
	q.mu.RLock()
	stats := q.stats
	q.mu.RUnlock()

	stats.PendingCount = len(q.jobs)
	return stats, nil
}

const maxRetries = 5

type SQLiteQueue struct {
	db *sql.DB
}

func NewSQLiteQueue(db *sql.DB) *SQLiteQueue {
	return &SQLiteQueue{db: db}
}

func (q *SQLiteQueue) Enqueue(ctx context.Context, job *SubmissionJob) error {
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO submission_queue
		(tenant_id, no_id, score, max_score, detail_xml, attempt_token, validasi, status, next_retry_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP)
	`, job.TenantID, job.NoID, job.Score, job.MaxScore, job.DetailXML, job.AttemptToken, job.Validasi)
	return err
}

func (q *SQLiteQueue) Dequeue(ctx context.Context) (*SubmissionJob, error) {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("dequeue begin tx: %w", err)
	}
	defer tx.Rollback()

	var job SubmissionJob
	var lastError sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, no_id, score, max_score, detail_xml, attempt_token,
		       validasi, retry_count, last_error, status, created_at, updated_at, next_retry_at
		FROM submission_queue
		WHERE status = 'pending' AND next_retry_at <= CURRENT_TIMESTAMP
		ORDER BY created_at ASC
		LIMIT 1
	`).Scan(&job.ID, &job.TenantID, &job.NoID, &job.Score, &job.MaxScore, &job.DetailXML,
		&job.AttemptToken, &job.Validasi, &job.RetryCount, &lastError, &job.Status,
		&job.CreatedAt, &job.UpdatedAt, &job.NextRetryAt)
	if lastError.Valid {
		job.LastError = lastError.String
	}

	if err == sql.ErrNoRows {
		_ = tx.Commit()
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dequeue select: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE submission_queue SET status = 'processing', updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, job.ID)
	if err != nil {
		return nil, fmt.Errorf("dequeue update processing: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("dequeue commit: %w", err)
	}
	return &job, nil
}

func (q *SQLiteQueue) MarkCompleted(ctx context.Context, jobID int64) error {
	_, err := q.db.ExecContext(ctx, `
		UPDATE submission_queue SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, jobID)
	return err
}

func (q *SQLiteQueue) MarkFailed(ctx context.Context, jobID int64, processErr error) error {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mark_failed begin tx: %w", err)
	}
	defer tx.Rollback()

	var retryCount int
	var tenantID int
	var noID string
	var detailXML string
	err = tx.QueryRowContext(ctx, `
		SELECT retry_count, tenant_id, no_id, COALESCE(detail_xml, '') FROM submission_queue WHERE id = ?
	`, jobID).Scan(&retryCount, &tenantID, &noID, &detailXML)
	if err != nil {
		return fmt.Errorf("mark_failed select: %w", err)
	}

	retryCount++
	errMsg := ""
	if processErr != nil {
		errMsg = processErr.Error()
	}

	if retryCount >= maxRetries {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO failed_submissions (original_job_id, tenant_id, no_id, error_message, detail_xml)
			VALUES (?, ?, ?, ?, ?)
		`, jobID, tenantID, noID, errMsg, detailXML)
		if err != nil {
			return fmt.Errorf("mark_failed insert dead letter: %w", err)
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM submission_queue WHERE id = ?`, jobID)
		if err != nil {
			return fmt.Errorf("mark_failed delete job: %w", err)
		}
	} else {
		backoff := time.Duration(1<<uint(retryCount)) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE submission_queue
			SET status = 'pending', retry_count = ?, last_error = ?,
			    next_retry_at = datetime('now', '+' || ? || ' seconds'),
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, retryCount, errMsg, int(backoff.Seconds()), jobID)
		if err != nil {
			return fmt.Errorf("mark_failed update retry: %w", err)
		}
	}

	return tx.Commit()
}

func (q *SQLiteQueue) GetStats(ctx context.Context) (QueueStats, error) {
	var stats QueueStats
	err := q.db.QueryRowContext(ctx, `
		SELECT
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END)
		FROM submission_queue
	`).Scan(&stats.PendingCount, &stats.ProcessingCount, &stats.FailedCount)
	if err != nil {
		return stats, fmt.Errorf("get stats: %w", err)
	}
	return stats, nil
}

// Deprecated: gunakan FilesystemQueue. Dipertahankan untuk fixture/test legacy.
type BufferedSQLiteQueue struct {
	inner     *SQLiteQueue
	buf       chan *SubmissionJob
	stats     QueueStats
	statsMu   sync.RWMutex
	drainDone chan struct{}
}

// Deprecated: gunakan NewFilesystemQueue atau NewFilesystemQueueWithConfig.
func NewBufferedSQLiteQueue(db *sql.DB, bufferSize int) *BufferedSQLiteQueue {
	bq := &BufferedSQLiteQueue{
		inner:     NewSQLiteQueue(db),
		buf:       make(chan *SubmissionJob, bufferSize),
		drainDone: make(chan struct{}),
	}
	return bq
}

func (bq *BufferedSQLiteQueue) StartDrain(ctx context.Context) {
	go func() {
		defer close(bq.drainDone)
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-bq.buf:
				for attempt := 0; attempt < 3; attempt++ {
					err := bq.inner.Enqueue(ctx, job)
					if err == nil {
						break
					}
					time.Sleep(50 * time.Millisecond)
				}
			}
		}
	}()
}

func (bq *BufferedSQLiteQueue) WaitDrain() {
	for {
		if len(bq.buf) == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (bq *BufferedSQLiteQueue) Enqueue(ctx context.Context, job *SubmissionJob) error {
	select {
	case bq.buf <- job:
		bq.statsMu.Lock()
		bq.stats.PendingCount++
		bq.statsMu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (bq *BufferedSQLiteQueue) Dequeue(ctx context.Context) (*SubmissionJob, error) {
	return bq.inner.Dequeue(ctx)
}

func (bq *BufferedSQLiteQueue) MarkCompleted(ctx context.Context, jobID int64) error {
	return bq.inner.MarkCompleted(ctx, jobID)
}

func (bq *BufferedSQLiteQueue) MarkFailed(ctx context.Context, jobID int64, err error) error {
	return bq.inner.MarkFailed(ctx, jobID, err)
}

func (bq *BufferedSQLiteQueue) GetStats(ctx context.Context) (QueueStats, error) {
	bq.statsMu.RLock()
	bufPending := bq.stats.PendingCount
	bq.statsMu.RUnlock()

	dbStats, err := bq.inner.GetStats(ctx)
	if err != nil {
		return dbStats, err
	}
	dbStats.PendingCount += bufPending
	return dbStats, nil
}
