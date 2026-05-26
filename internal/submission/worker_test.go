package submission

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	worker := NewWorker(q, func(context.Context, []*SubmissionJob) error {
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
