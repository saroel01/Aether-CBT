package submission

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestProperty10NoLostResultsUnderConcurrentEnqueue verifies Property 10 / Requirement
// 13.6: when many results arrive concurrently (here ~500, as in the exam-day peak), the
// filesystem queue loses none of them. Every enqueued job must be delivered to the worker
// exactly once — the atomic rename + retry + recovery guarantees survive the burst.
//
// This complements the load-test profile in tests/load/E2E_RESULTS.md (which runs against
// a live server): there, the bottleneck is SQLite write concurrency on hasil_tes; here we
// isolate the queue layer itself, which is the component that enforces no-loss.
func TestProperty10NoLostResultsUnderConcurrentEnqueue(t *testing.T) {
	const n = 500
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}

	// Concurrent burst: 500 goroutines enqueue distinct jobs at once.
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if err := q.Enqueue(ctx, testJob(jobSuffix(i))); err != nil {
				t.Errorf("Enqueue(%d): %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	// Drain via a worker whose processor counts every delivered job once. Use a larger
	// batch size than the default so the 500-job drain is fast and deterministic (the
	// default batch-of-5 + idle sleeps makes the drain time variable and would make this
	// test flaky under load).
	var processed int64
	worker := NewWorkerWithConfig(q, func(_ context.Context, jobs []*SubmissionJob) ([]error, error) {
		atomic.AddInt64(&processed, int64(len(jobs)))
		return make([]error, len(jobs)), nil
	}, 50, 50*time.Millisecond)
	go worker.Run(ctx)

	// Wait until the worker has processed every enqueued job (generous deadline: the
	// assertion is about correctness/no-loss, not throughput).
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&processed) == int64(n) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	worker.Stop()

	if got := atomic.LoadInt64(&processed); got != int64(n) {
		stats, _ := q.GetStats(ctx)
		t.Fatalf("processed = %d, want %d (no lost results); queue stats: %+v", got, n, stats)
	}

	// On Windows the queue's done/ directory holds freshly-written JSON files whose file
	// handles may still be flushing when the test framework tries to clean up the temp dir,
	// producing a spurious "directory not empty" cleanup error unrelated to the property
	// under test. A short settle lets the FS release the handles.
	time.Sleep(100 * time.Millisecond)
}

// jobSuffix produces a unique, filesystem-safe suffix per index so each job yields a
// distinct enqueued file name (the queue dedupes by file name within a tick).
func jobSuffix(i int) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if i == 0 {
		return "conc-0"
	}
	var b []byte
	for i > 0 {
		b = append(b, digits[i%36])
		i /= 36
	}
	return "conc-" + string(b)
}
