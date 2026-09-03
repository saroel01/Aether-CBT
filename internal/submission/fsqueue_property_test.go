package submission

// Feature: filesystem-submission-queue, Property 1

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// filenamePattern matches the expected Job_File naming convention as written by Enqueue:
// <unix_nano(19 digits)>-<tenant_id>-<sanitized_no_id>-<8hex>.json
//
// A job that has FAILED at least once carries an extra retry due-time segment appended by
// MarkFailed (`-due<unix_nano>` before the extension, see retryFilenamePattern). Enqueue
// itself never adds that segment, which is how clause 3.4 is preserved structurally: a job
// that never failed is never delayed.
//
// Validates: Requirements 1.1, 1.2, 2.3
var filenamePattern = regexp.MustCompile(`^\d{19}-\d+-[A-Za-z0-9_-]+-[0-9a-f]{8}\.json$`)

// retryFilenamePattern matches the name a job carries while it waits out its backoff in
// pending/: the Enqueue base name plus the due-time segment.
//
// Validates: Requirements 2.3
var retryFilenamePattern = regexp.MustCompile(`^\d{19}-\d+-[A-Za-z0-9_-]+-[0-9a-f]{8}-due\d+\.json$`)

// genValidJobAt generates a valid SubmissionJob suitable for Enqueue.
// All required fields (TenantID, NoID, Validasi) are non-zero/non-empty.
// The idx parameter is used to make draw names unique when called in a loop.
func genValidJobAt(t *rapid.T, idx int) *SubmissionJob {
	tenantID := rapid.IntRange(1, 1000).Draw(t, fmt.Sprintf("tenant_id_%d", idx))
	noID := rapid.StringMatching(`[A-Za-z0-9_-]{1,20}`).Draw(t, fmt.Sprintf("no_id_%d", idx))
	validasi := rapid.StringMatching(`[A-Za-z0-9_-]{1,30}`).Draw(t, fmt.Sprintf("validasi_%d", idx))
	score := rapid.StringMatching(`[0-9]{1,3}`).Draw(t, fmt.Sprintf("score_%d", idx))
	maxScore := rapid.StringMatching(`[0-9]{1,3}`).Draw(t, fmt.Sprintf("max_score_%d", idx))
	attemptToken := rapid.StringMatching(`[0-9a-f]{32}`).Draw(t, fmt.Sprintf("attempt_token_%d", idx))

	return &SubmissionJob{
		TenantID:     tenantID,
		NoID:         noID,
		Validasi:     validasi,
		Score:        score,
		MaxScore:     maxScore,
		AttemptToken: attemptToken,
	}
}

// TestPropertyEnqueueAtomicityAndUniqueness verifies Property 1: Enqueue Atomicity and Unique Naming.
// For N (1..50) valid jobs enqueued to a fresh FilesystemQueue:
// (a) tmp/ is empty after all enqueues
// (b) len(pending/) == N
// (c) all filenames are unique and match the pattern ^\d{19}-\d+-[A-Za-z0-9_-]+-[0-9a-f]{8}\.json$
//
// Validates: Requirements 1.1, 1.2
func TestPropertyEnqueueAtomicityAndUniqueness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate N in range [1, 50]
		n := rapid.IntRange(1, 50).Draw(rt, "n")

		// Create a fresh FilesystemQueue in a temp directory.
		// Use the outer *testing.T for TempDir so cleanup is registered correctly.
		root := t.TempDir()
		q, err := NewFilesystemQueue(root)
		if err != nil {
			rt.Fatalf("NewFilesystemQueue failed: %v", err)
		}

		ctx := context.Background()

		// Enqueue N valid jobs
		for i := 0; i < n; i++ {
			job := genValidJobAt(rt, i)
			if err := q.Enqueue(ctx, job); err != nil {
				rt.Fatalf("Enqueue job %d failed: %v", i, err)
			}
		}

		// (a) Assert tmp/ is empty after all enqueues
		tmpEntries, err := os.ReadDir(filepath.Join(root, "tmp"))
		if err != nil {
			rt.Fatalf("ReadDir tmp/ failed: %v", err)
		}
		if len(tmpEntries) != 0 {
			names := make([]string, len(tmpEntries))
			for i, e := range tmpEntries {
				names[i] = e.Name()
			}
			rt.Fatalf("tmp/ is not empty after enqueue: found %d file(s): %v", len(tmpEntries), names)
		}

		// (b) Assert len(pending/) == N
		pendingEntries, err := os.ReadDir(filepath.Join(root, "pending"))
		if err != nil {
			rt.Fatalf("ReadDir pending/ failed: %v", err)
		}
		var pendingFiles []string
		for _, e := range pendingEntries {
			if strings.HasSuffix(e.Name(), ".json") {
				pendingFiles = append(pendingFiles, e.Name())
			}
		}
		if len(pendingFiles) != n {
			rt.Fatalf("expected %d files in pending/, got %d: %v", n, len(pendingFiles), pendingFiles)
		}

		// (c) All filenames are unique and match the expected pattern
		seen := make(map[string]struct{}, n)
		for _, name := range pendingFiles {
			// Check uniqueness
			if _, exists := seen[name]; exists {
				rt.Fatalf("duplicate filename in pending/: %q", name)
			}
			seen[name] = struct{}{}

			// Check pattern match
			if !filenamePattern.MatchString(name) {
				rt.Fatalf("filename %q does not match expected pattern %s", name, filenamePattern.String())
			}
		}
	})
}

// TestPropertyMarkFailedEncodesDueTimeInFilename replaces the former EnqueuedAt-based backoff
// assertion. The schedule is now authoritative in the FILE NAME, because the name is the only
// thing Dequeue reads when picking a candidate; EnqueuedAt is kept purely as payload
// information. For a random number of consecutive failures the test asserts:
//
// (a) the pending file matches the retry name shape;
// (b) its base name (the Enqueue-written prefix, including tenant_id and no_id) is unchanged,
//     so admins can still correlate the job across directories by prefix;
// (c) the encoded due time equals now + min(2^(retry_count-1), 30) seconds.
//
// Validates: Requirements 2.3, 3.4
func TestPropertyMarkFailedEncodesDueTimeInFilename(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		failures := rapid.IntRange(1, 7).Draw(rt, "failures")

		root := t.TempDir()
		clock := newTestClock()
		q, err := NewFilesystemQueueWithConfig(root, FilesystemQueueConfig{
			MaxRetries: 8, // > failures so every attempt returns to pending/
			Now:        clock.Now,
		})
		if err != nil {
			rt.Fatalf("NewFilesystemQueueWithConfig: %v", err)
		}
		ctx := context.Background()

		if err := q.Enqueue(ctx, testJob("due-seg")); err != nil {
			rt.Fatalf("Enqueue: %v", err)
		}
		names, err := os.ReadDir(filepath.Join(root, "pending"))
		if err != nil || len(names) != 1 {
			rt.Fatalf("ReadDir pending after enqueue: %v (entries=%d)", err, len(names))
		}
		enqueuedName := names[0].Name()
		if !filenamePattern.MatchString(enqueuedName) {
			rt.Fatalf("enqueued name %q does not match %s", enqueuedName, filenamePattern)
		}

		for attempt := 1; attempt <= failures; attempt++ {
			job, err := q.Dequeue(ctx)
			if err != nil {
				rt.Fatalf("Dequeue attempt %d: %v", attempt, err)
			}
			if job == nil {
				rt.Fatalf("Dequeue attempt %d returned nil; job should be due at %s",
					attempt, clock.Now())
			}
			failedAt := clock.Now()
			if err := q.MarkFailed(ctx, job.ID, errors.New("synthetic")); err != nil {
				rt.Fatalf("MarkFailed attempt %d: %v", attempt, err)
			}

			pending, err := os.ReadDir(filepath.Join(root, "pending"))
			if err != nil {
				rt.Fatalf("ReadDir pending: %v", err)
			}
			var jsonNames []string
			for _, e := range pending {
				if strings.HasSuffix(e.Name(), ".json") {
					jsonNames = append(jsonNames, e.Name())
				}
			}
			if len(jsonNames) != 1 {
				rt.Fatalf("pending after attempt %d = %v, want exactly one file", attempt, jsonNames)
			}
			retryName := jsonNames[0]

			// (a) name shape
			if !retryFilenamePattern.MatchString(retryName) {
				rt.Fatalf("retry name %q does not match %s", retryName, retryFilenamePattern)
			}
			// (b) prefix preserved for admin correlation
			base, due, hasDue := splitDueTime(retryName)
			if !hasDue {
				rt.Fatalf("retry name %q carries no due segment", retryName)
			}
			if base != enqueuedName {
				rt.Fatalf("retry base = %q, want the Enqueue name %q (prefix correlation broken)",
					base, enqueuedName)
			}
			// (c) schedule
			wantDue := failedAt.Add(expectedBackoff(attempt))
			if !due.Equal(wantDue) {
				rt.Fatalf("attempt %d due = %s, want %s (backoff min(2^(n-1),30)s)",
					attempt, due, wantDue)
			}
			// The un-due job must not be dequeuable yet, and must become dequeuable exactly
			// when its due time arrives.
			if job, err := q.Dequeue(ctx); err != nil || job != nil {
				rt.Fatalf("dequeued before due time (err=%v, job=%v)", err, job)
			}
			clock.Advance(expectedBackoff(attempt))
		}
	})
}
