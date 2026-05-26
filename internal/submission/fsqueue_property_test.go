package submission

// Feature: filesystem-submission-queue, Property 1

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// filenamePattern matches the expected Job_File naming convention:
// <unix_nano(19 digits)>-<tenant_id>-<sanitized_no_id>-<8hex>.json
//
// Validates: Requirements 1.1, 1.2
var filenamePattern = regexp.MustCompile(`^\d{19}-\d+-[A-Za-z0-9_-]+-[0-9a-f]{8}\.json$`)

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
