package submission

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkerProcessBatchSafeRecoversPanicAndMarksJobsFailed(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	q.maxRetries = 1
	for _, suffix := range []string{"panic-a", "panic-b"} {
		if err := q.Enqueue(ctx, testJob(suffix)); err != nil {
			t.Fatalf("Enqueue(%s): %v", suffix, err)
		}
	}
	batch, err := q.DequeueBatch(ctx, 2)
	if err != nil {
		t.Fatalf("DequeueBatch: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("batch length = %d, want 2", len(batch))
	}

	worker := NewWorker(q, func(context.Context, []*SubmissionJob) ([]error, error) {
		panic("processor exploded")
	})
	worker.processBatchSafe(ctx, batch)

	stats, err := q.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.FailedCount != 2 || stats.ProcessingCount != 0 {
		t.Fatalf("stats after panic = %+v, want failed=2 processing=0", stats)
	}

	for _, name := range jsonNames(t, q.failedDir) {
		data, err := os.ReadFile(filepath.Join(q.failedDir, name))
		if err != nil {
			t.Fatalf("ReadFile failed job: %v", err)
		}
		job, err := UnmarshalJob(data)
		if err != nil {
			t.Fatalf("UnmarshalJob failed job: %v", err)
		}
		if !strings.Contains(job.LastError, "worker panic: processor exploded") {
			t.Fatalf("last_error = %q, want worker panic message", job.LastError)
		}
	}
}

// TestWorkerStopIdempotent verifies clause 2.8: calling Stop multiple times does not panic.
func TestWorkerStopIdempotent(t *testing.T) {
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	defer q.Close()

	w := NewWorker(q, func(ctx context.Context, jobs []*SubmissionJob) ([]error, error) {
		return make([]error, len(jobs)), nil
	})

	// Calling Stop multiple times must not panic
	w.Stop()
	w.Stop()
	w.Stop()
}

func TestWorkerStopWaitsForInFlightBatch(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	defer q.Close()

	if err := q.Enqueue(ctx, testJob("wait-1")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	processingStarted := make(chan struct{})
	var completedBatch bool

	w := NewWorkerWithConfig(q, func(ctx context.Context, jobs []*SubmissionJob) ([]error, error) {
		close(processingStarted)
		time.Sleep(100 * time.Millisecond) // simulate database write
		completedBatch = true
		return make([]error, len(jobs)), nil
	}, 1, 10*time.Millisecond)

	go w.Run(ctx)

	// Wait until processor has started processing the batch
	<-processingStarted

	// Call Stop while batch is in-flight: Stop must block until batch finishes writing
	w.Stop()

	if !completedBatch {
		t.Fatal("Worker.Stop returned before in-flight batch finished processing")
	}
}
