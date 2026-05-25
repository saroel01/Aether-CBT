package submission

import (
	"context"
	"sync"
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

	// Critical fix: source PendingCount directly from the channel to eliminate
	// the race between channel operations and counter updates. This makes the
	// most important stat (queue depth) reliable for debugging.
	stats.PendingCount = len(q.jobs)
	return stats, nil
}
