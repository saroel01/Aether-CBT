package submission

// Feature: codebase-bug-sweep, Property 1
//
// **Property 1: Bug Condition** - C3 jadwal backoff benar-benar berlaku.
//
// Klausa 1.3 (F): MarkFailed menghitung backoff eksponensial min(2^(n-1), 30) detik dan
// menuliskannya HANYA ke job.EnqueuedAt, sementara Dequeue memilih kandidat murni dari
// urutan nama hasil os.ReadDir dan tidak pernah membaca EnqueuedAt. Karena file retry
// mempertahankan prefix unix_nano aslinya, ia adalah nama TERTUA di pending/ sehingga
// selalu terpilih lebih dulu — retry loop ketat yang menghabiskan seluruh maxRetries dalam
// hitungan milidetik dan membuat submission baru kelaparan.
//
// Klausa 2.3 (F'): job tidak dapat di-dequeue sebelum waktu jatuh temponya, DAN job yang
// belum jatuh tempo dilewati sehingga submission baru tetap terproses.
//
// Validates: Requirements 2.3, 3.4

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// testClock is a manually advanced clock injected into FilesystemQueue so the property can
// explore the backoff schedule without sleeping through real 1..30 second waits.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func newTestClock() *testClock {
	return &testClock{t: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// jobIdentity derives the stable identity of a queue file across retries: the part of the
// name written by Enqueue (unix_nano-tenant_id-no_id-8hex), with any scheduling segment
// appended by MarkFailed removed.
//
// This is deliberately re-derived here instead of calling the production parser, so the
// property does not inherit a defect from the very code under test. It accepts BOTH the
// pre-fix shape (<base>.json) and the post-fix shape (<base>-due<unix_nano>.json).
func jobIdentity(fileName string) string {
	stem := strings.TrimSuffix(fileName, ".json")
	if idx := strings.LastIndex(stem, "-due"); idx >= 0 {
		if _, err := strconv.ParseInt(stem[idx+len("-due"):], 10, 64); err == nil {
			stem = stem[:idx]
		}
	}
	return stem
}

// expectedBackoff mirrors the schedule the queue promises: min(2^(retryCount-1), 30) seconds,
// where retryCount is the value AFTER the increment performed by MarkFailed.
func expectedBackoff(retryCount int) time.Duration {
	if retryCount < 1 {
		return 0
	}
	sec := 1 << (retryCount - 1)
	if sec > 30 {
		sec = 30
	}
	return time.Duration(sec) * time.Second
}

// TestPropertyBackoffScheduleIsEnforced generates random interleavings of enqueue, failure
// and time advancement, then asserts:
//
//	(a) no job is ever dequeued before its due time;
//	(b) a freshly enqueued job is still reachable while a not-yet-due retry sits in pending/,
//	    i.e. an un-due job never blocks the queue.
//
// Validates: Requirements 2.3, 3.4
func TestPropertyBackoffScheduleIsEnforced(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		root := t.TempDir()
		clock := newTestClock()
		q, err := NewFilesystemQueueWithConfig(root, FilesystemQueueConfig{
			// High retry ceiling so the exploration exercises the SCHEDULE rather than
			// dead-lettering; clause 3.6 (dead letter after maxRetries) is covered by
			// Property 2 / the preservation unit tests.
			MaxRetries: 12,
			Now:        clock.Now,
		})
		if err != nil {
			rt.Fatalf("NewFilesystemQueueWithConfig: %v", err)
		}
		ctx := context.Background()

		// identity -> earliest time at which the queue promised the job may run again.
		dueAt := make(map[string]time.Time)

		steps := rapid.IntRange(4, 20).Draw(rt, "steps")
		enqueued := 0
		for i := 0; i < steps; i++ {
			op := rapid.SampledFrom([]string{"enqueue", "advance", "dequeue_fail", "dequeue_ok"}).
				Draw(rt, fmt.Sprintf("op_%d", i))

			switch op {
			case "enqueue":
				if err := q.Enqueue(ctx, testJob(fmt.Sprintf("bo-%d-%d", i, enqueued))); err != nil {
					rt.Fatalf("Enqueue: %v", err)
				}
				enqueued++

			case "advance":
				sec := rapid.IntRange(0, 40).Draw(rt, fmt.Sprintf("advance_%d", i))
				clock.Advance(time.Duration(sec) * time.Second)

			case "dequeue_fail", "dequeue_ok":
				job, err := q.Dequeue(ctx)
				if err != nil {
					rt.Fatalf("Dequeue: %v", err)
				}
				if job == nil {
					continue
				}
				id := jobIdentity(job.fileName)

				// (a) The backoff schedule must actually gate the dequeue.
				if due, ok := dueAt[id]; ok && clock.Now().Before(due) {
					rt.Fatalf("job %q dequeued at %s but is not due until %s (retry_count=%d, %s early): "+
						"backoff schedule is not enforced",
						id, clock.Now().Format(time.RFC3339), due.Format(time.RFC3339),
						job.RetryCount, due.Sub(clock.Now()))
				}

				if op == "dequeue_fail" {
					if err := q.MarkFailed(ctx, job.ID, errors.New("synthetic failure")); err != nil {
						rt.Fatalf("MarkFailed: %v", err)
					}
					dueAt[id] = clock.Now().Add(expectedBackoff(job.RetryCount + 1))
				} else {
					if err := q.MarkCompleted(ctx, job.ID); err != nil {
						rt.Fatalf("MarkCompleted: %v", err)
					}
					delete(dueAt, id)
				}
			}
		}

		// (b) Starvation guard: with whatever un-due retries the exploration left behind,
		// a brand new submission must still be reachable WITHOUT advancing the clock.
		fresh := testJob("bo-fresh")
		if err := q.Enqueue(ctx, fresh); err != nil {
			rt.Fatalf("Enqueue fresh: %v", err)
		}
		pendingEntries, err := os.ReadDir(q.pendingDir)
		if err != nil {
			rt.Fatalf("ReadDir pending: %v", err)
		}
		found := false
		for attempt := 0; attempt <= len(pendingEntries); attempt++ {
			job, err := q.Dequeue(ctx)
			if err != nil {
				rt.Fatalf("Dequeue (starvation guard): %v", err)
			}
			if job == nil {
				break
			}
			if strings.Contains(job.fileName, "bo-fresh") {
				found = true
				break
			}
			// Park anything else back out of the way without touching its schedule.
			if err := q.MarkCompleted(ctx, job.ID); err != nil {
				rt.Fatalf("MarkCompleted (starvation guard): %v", err)
			}
		}
		if !found {
			rt.Fatalf("freshly enqueued job was not reachable within %d dequeue attempts: "+
				"an un-due retry is blocking the queue", len(pendingEntries)+1)
		}
	})
}
