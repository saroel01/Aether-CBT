package submission

import (
	"context"
	"sync"
)

type Queue interface {
	Enqueue(ctx context.Context, job *SubmissionJob) error
	Dequeue(ctx context.Context) (*SubmissionJob, error)
	MarkCompleted(ctx context.Context, jobID int64) error
	MarkFailed(ctx context.Context, jobID int64, err error) error
	GetStats(ctx context.Context) (QueueStats, error)
}

type QueueStats struct {
	PendingCount    int `json:"pending_count"`
	ProcessingCount int `json:"processing_count"`
	FailedCount     int `json:"failed_count"`
}

// InMemoryQueue adalah implementasi sederhana untuk PoC cepat.
// TIDAK survive restart. Gunakan hanya untuk validasi pola.
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

func (q *InMemoryQueue) MarkCompleted(ctx context.Context, jobID int64) error {
	q.mu.Lock()
	q.stats.ProcessingCount = max(0, q.stats.ProcessingCount-1)
	q.mu.Unlock()
	return nil
}

func (q *InMemoryQueue) MarkFailed(ctx context.Context, jobID int64, err error) error {
	q.mu.Lock()
	q.stats.ProcessingCount = max(0, q.stats.ProcessingCount-1)
	q.stats.FailedCount++
	q.mu.Unlock()
	return nil
}

func (q *InMemoryQueue) GetStats(ctx context.Context) (QueueStats, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.stats, nil
}
