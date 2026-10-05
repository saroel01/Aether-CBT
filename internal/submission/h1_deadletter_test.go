package submission

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Audit H1: tests for SubmittedAt, permanent errors, OnDeadLetter and RecordFailure.

func enqueueAndClaim(t *testing.T, q *FilesystemQueue) *SubmissionJob {
	t.Helper()
	ctx := context.Background()
	if err := q.Enqueue(ctx, &SubmissionJob{
		TenantID: 1, NoID: "S-1", Validasi: "1_S-1_7", Score: "1", MaxScore: "1", AttemptToken: "tok",
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	job, err := q.Dequeue(ctx)
	if err != nil || job == nil {
		t.Fatalf("Dequeue: job=%v err=%v", job, err)
	}
	return job
}

func TestMarkFailedTransientKeepsSubmittedAt(t *testing.T) {
	clock := newTestClock()
	var deadLetters int
	q, err := NewFilesystemQueueWithConfig(t.TempDir(), FilesystemQueueConfig{
		Now:          clock.Now,
		OnDeadLetter: func(*SubmissionJob) { deadLetters++ },
	})
	if err != nil {
		t.Fatalf("NewFilesystemQueueWithConfig: %v", err)
	}
	defer q.Close()
	ctx := context.Background()

	job := enqueueAndClaim(t, q)
	submittedAt := job.SubmittedAt
	if submittedAt.IsZero() {
		t.Fatal("Enqueue must stamp SubmittedAt")
	}
	if err := q.MarkFailed(ctx, job.ID, errors.New("database is locked")); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	clock.Advance(time.Minute)
	retry, err := q.Dequeue(ctx)
	if err != nil || retry == nil {
		t.Fatalf("transient failure must return to pending/: job=%v err=%v", retry, err)
	}
	if !retry.SubmittedAt.Equal(submittedAt) {
		t.Fatalf("SubmittedAt moved on retry: %v -> %v", submittedAt, retry.SubmittedAt)
	}
	if deadLetters != 0 {
		t.Fatalf("OnDeadLetter called %d times for a transient failure", deadLetters)
	}
}

func TestMarkFailedPermanentDeadLettersImmediately(t *testing.T) {
	root := t.TempDir()
	var got []*SubmissionJob
	q, err := NewFilesystemQueueWithConfig(root, FilesystemQueueConfig{
		OnDeadLetter: func(j *SubmissionJob) { got = append(got, j) },
	})
	if err != nil {
		t.Fatalf("NewFilesystemQueueWithConfig: %v", err)
	}
	defer q.Close()
	ctx := context.Background()

	job := enqueueAndClaim(t, q)
	if err := q.MarkFailed(ctx, job.ID, Permanent(errors.New("invalid detail XML"))); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	stats, _ := q.GetStats(ctx)
	if stats.FailedCount != 1 || stats.PendingCount != 0 || stats.ProcessingCount != 0 {
		t.Fatalf("permanent error must dead-letter on first failure, stats=%+v", stats)
	}
	errTxt, _ := filepath.Glob(filepath.Join(root, "failed", "*.error.txt"))
	if len(errTxt) != 1 {
		t.Fatalf("expected one .error.txt, got %v", errTxt)
	}
	if len(got) != 1 || got[0].LastError != "invalid detail XML" || got[0].NoID != "S-1" {
		t.Fatalf("OnDeadLetter calls = %+v, want exactly one with the job", got)
	}
}

func TestMarkFailedRetriesExhaustedCallsOnDeadLetter(t *testing.T) {
	clock := newTestClock()
	var deadLetters int
	q, err := NewFilesystemQueueWithConfig(t.TempDir(), FilesystemQueueConfig{
		MaxRetries:   2,
		Now:          clock.Now,
		OnDeadLetter: func(*SubmissionJob) { deadLetters++ },
	})
	if err != nil {
		t.Fatalf("NewFilesystemQueueWithConfig: %v", err)
	}
	defer q.Close()
	ctx := context.Background()

	job := enqueueAndClaim(t, q)
	for i := 0; i < 2; i++ {
		if err := q.MarkFailed(ctx, job.ID, errors.New("busy")); err != nil {
			t.Fatalf("MarkFailed %d: %v", i, err)
		}
		clock.Advance(time.Minute)
		if i == 0 {
			if job, err = q.Dequeue(ctx); err != nil || job == nil {
				t.Fatalf("Dequeue retry: job=%v err=%v", job, err)
			}
		}
	}
	if deadLetters != 1 {
		t.Fatalf("OnDeadLetter called %d times, want 1", deadLetters)
	}
}

func TestIsPermanent(t *testing.T) {
	if IsPermanent(errors.New("x")) || IsPermanent(nil) {
		t.Fatal("plain errors are transient")
	}
	if !IsPermanent(Permanent(errors.New("x"))) {
		t.Fatal("Permanent(err) must be permanent")
	}
	if !IsPermanent(errors.Join(errors.New("ctx"), ErrGraceExceeded)) {
		t.Fatal("ErrGraceExceeded must be permanent")
	}
}

func TestProcessorGraceUsesSubmittedAtNotEnqueuedAt(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	// 90-minute exam (95m allowed). Original submission at +80m, retry due at +200m.
	loginTime := time.Now().UTC().Add(-300 * time.Minute)
	if _, err := db.Exec(`UPDATE cek_login SET login_time = ? WHERE tenant_id = 1 AND peserta_id = 42`, loginTime); err != nil {
		t.Fatalf("update login_time: %v", err)
	}
	job := processorJob("10", detailXMLWithQuestions())
	job.SubmittedAt = loginTime.Add(80 * time.Minute)
	job.EnqueuedAt = loginTime.Add(200 * time.Minute)

	if err := NewProcessor(db).Process(context.Background(), job); err != nil {
		t.Fatalf("Process: %v, want success because SubmittedAt is inside the grace window", err)
	}
}

func TestProcessorGraceExceededIsPermanent(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()

	loginTime := time.Now().UTC().Add(-300 * time.Minute)
	if _, err := db.Exec(`UPDATE cek_login SET login_time = ? WHERE tenant_id = 1 AND peserta_id = 42`, loginTime); err != nil {
		t.Fatalf("update login_time: %v", err)
	}
	job := processorJob("10", detailXMLWithQuestions())
	job.SubmittedAt = loginTime.Add(96 * time.Minute)

	err := NewProcessor(db).Process(context.Background(), job)
	if !errors.Is(err, ErrGraceExceeded) || !IsPermanent(err) {
		t.Fatalf("err = %v, want permanent ErrGraceExceeded", err)
	}
}

func TestRecordFailureWritesRow(t *testing.T) {
	db := setupProcessorDB(t)
	defer db.Close()
	migration, err := os.ReadFile(filepath.Join("..", "db", "migrations", "035_create_submission_failure.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	submittedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	job := &SubmissionJob{
		TenantID: 3, NoID: "S-9", Validasi: "3_S-9_1", AttemptToken: "tok-9",
		LastError: "grace period exceeded", SubmittedAt: submittedAt,
	}
	if err := RecordFailure(db, job); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	var tenantID int
	var noID, validasi, token, msg string
	var at time.Time
	if err := db.QueryRow(`SELECT tenant_id, no_id, validasi, attempt_token, error_message, submitted_at FROM submission_failure`).
		Scan(&tenantID, &noID, &validasi, &token, &msg, &at); err != nil {
		t.Fatalf("select: %v", err)
	}
	if tenantID != 3 || noID != "S-9" || validasi != "3_S-9_1" || token != "tok-9" || msg != "grace period exceeded" || !at.Equal(submittedAt) {
		t.Fatalf("row = %d %s %s %s %q %v", tenantID, noID, validasi, token, msg, at)
	}
}
