package submission

import (
	"context"
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
type QueueStats struct {
	PendingCount    int `json:"pending_count"`
	ProcessingCount int `json:"processing_count"`
	FailedCount     int `json:"failed_count"`
	DoneCount       int `json:"done_count"`
}
