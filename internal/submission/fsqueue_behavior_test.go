package submission

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testJob(suffix string) *SubmissionJob {
	return &SubmissionJob{
		TenantID:     1,
		NoID:         "student-" + suffix,
		Validasi:     "1_student_" + suffix + "_7",
		Score:        "80",
		MaxScore:     "100",
		AttemptToken: "attempt-" + suffix,
	}
}

func jsonNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	return names
}

func TestFilesystemQueueDequeueFIFOMovesFilesToProcessing(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}

	var expected []time.Time
	for _, suffix := range []string{"a", "b", "c"} {
		job := testJob(suffix)
		if err := q.Enqueue(ctx, job); err != nil {
			t.Fatalf("Enqueue(%s): %v", suffix, err)
		}
		expected = append(expected, job.EnqueuedAt)
		time.Sleep(time.Millisecond)
	}

	var actual []time.Time
	for range expected {
		job, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("Dequeue: %v", err)
		}
		if job == nil {
			t.Fatal("Dequeue returned nil before queue was empty")
		}
		actual = append(actual, job.EnqueuedAt)
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("dequeue order mismatch:\nexpected: %v\nactual:   %v", expected, actual)
	}
	if got := len(jsonNames(t, q.pendingDir)); got != 0 {
		t.Fatalf("pending files = %d, want 0", got)
	}
	if got := len(jsonNames(t, q.processingDir)); got != len(expected) {
		t.Fatalf("processing files = %d, want %d", got, len(expected))
	}
}

func TestFilesystemQueueMarkCompletedMovesFileToDone(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	if err := q.Enqueue(ctx, testJob("complete")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	job, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if job == nil {
		t.Fatal("expected job")
	}
	before, err := os.ReadFile(filepath.Join(q.processingDir, job.fileName))
	if err != nil {
		t.Fatalf("ReadFile processing: %v", err)
	}

	if err := q.MarkCompleted(ctx, job.ID); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	if _, err := os.Stat(filepath.Join(q.processingDir, job.fileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("processing file still exists or stat failed unexpectedly: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(q.doneDir, job.fileName))
	if err != nil {
		t.Fatalf("ReadFile done: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("done file content changed after MarkCompleted")
	}
}

func TestFilesystemQueueMarkFailedRetriesThenDeadLetters(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	q.maxRetries = 2

	if err := q.Enqueue(ctx, testJob("fail")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	job, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if err := q.MarkFailed(ctx, job.ID, errors.New("temporary database lock")); err != nil {
		t.Fatalf("MarkFailed retry: %v", err)
	}

	stats, err := q.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats after retry: %v", err)
	}
	if stats.PendingCount != 1 || stats.FailedCount != 0 || stats.ProcessingCount != 0 {
		t.Fatalf("after first failure stats = %+v, want pending=1 processing=0 failed=0", stats)
	}
	retryJob, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue retry: %v", err)
	}
	if retryJob.RetryCount != 1 || retryJob.LastError != "temporary database lock" {
		t.Fatalf("retry metadata = count %d error %q", retryJob.RetryCount, retryJob.LastError)
	}

	if err := q.MarkFailed(ctx, retryJob.ID, errors.New("permanent database lock")); err != nil {
		t.Fatalf("MarkFailed dead letter: %v", err)
	}
	stats, err = q.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats after dead letter: %v", err)
	}
	if stats.PendingCount != 0 || stats.FailedCount != 1 || stats.ProcessingCount != 0 {
		t.Fatalf("after second failure stats = %+v, want pending=0 processing=0 failed=1", stats)
	}
}

func TestFilesystemQueueGetStatsCountsAllStateDirectories(t *testing.T) {
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	for dir, count := range map[string]int{
		q.pendingDir:    2,
		q.processingDir: 3,
		q.doneDir:       4,
		q.failedDir:     5,
	} {
		for i := 0; i < count; i++ {
			if err := os.WriteFile(filepath.Join(dir, strings.Repeat("x", i+1)+".json"), []byte("{}"), 0644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("not json"), 0644); err != nil {
			t.Fatalf("WriteFile ignore: %v", err)
		}
	}

	stats, err := q.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.PendingCount != 2 || stats.ProcessingCount != 3 || stats.DoneCount != 4 || stats.FailedCount != 5 {
		t.Fatalf("stats = %+v, want pending=2 processing=3 done=4 failed=5", stats)
	}
}

func TestFilesystemQueueDequeueMovesCorruptJSONToFailed(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(q.pendingDir, "0000000000000000001-1-bad-12345678.json"), []byte("{bad json"), 0644); err != nil {
		t.Fatalf("WriteFile corrupt: %v", err)
	}

	job, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue corrupt: %v", err)
	}
	if job != nil {
		t.Fatalf("Dequeue corrupt job = %+v, want nil", job)
	}
	if got := len(jsonNames(t, q.failedDir)); got != 1 {
		t.Fatalf("failed json count = %d, want 1", got)
	}
	errorFiles, err := filepath.Glob(filepath.Join(q.failedDir, "*.error.txt"))
	if err != nil {
		t.Fatalf("Glob error files: %v", err)
	}
	if len(errorFiles) != 1 {
		t.Fatalf("error companion files = %d, want 1", len(errorFiles))
	}
}
