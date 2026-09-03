package submission

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

// Worker memproses SubmissionJob dari queue secara batch dengan panic recovery.
// Memenuhi Requirement 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 17.3, 17.4.
// ProcessBatchFunc is the per-job outcome contract between the worker and the processor.
//
// jobErrs is parallel to jobs (nil entry == that job succeeded and was committed); txErr is a
// transaction-level failure that invalidates the whole batch. Without this contract the worker
// is structurally unable to tell a failed job from a healthy one, which is what made a single
// bad submission dead-letter up to batchSize-1 valid exam results (klausa 1.4 / 2.4).
type ProcessBatchFunc func(ctx context.Context, jobs []*SubmissionJob) (jobErrs []error, txErr error)

type Worker struct {
	queue       Queue
	processFunc ProcessBatchFunc
	stopChan    chan struct{}
	stopOnce    sync.Once

	batchSize    int           // default 5
	batchTimeout time.Duration // default 100ms
}

// NewWorker membuat Worker baru dengan processFunc batch.
// batchSize default 5, batchTimeout default 100ms.
func NewWorker(q Queue, processBatch ProcessBatchFunc) *Worker {
	return NewWorkerWithConfig(q, processBatch, 5, 100*time.Millisecond)
}

func NewWorkerWithConfig(q Queue, processBatch ProcessBatchFunc, batchSize int, batchTimeout time.Duration) *Worker {
	if batchSize <= 0 {
		batchSize = 5
	}
	if batchTimeout <= 0 {
		batchTimeout = 100 * time.Millisecond
	}
	return &Worker{
		queue:        q,
		processFunc:  processBatch,
		stopChan:     make(chan struct{}),
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
	}
}

// NewWorkerSingle membuat Worker yang memproses satu job sekaligus (batch-of-1).
// Disediakan untuk backward compatibility dengan test fixture lama yang memakai
// signature func(ctx, *SubmissionJob) error.
//
// Adaptor ini memetakan setiap job ke slot errornya sendiri, sehingga fixture lama pun
// mendapat isolasi per-job: satu job gagal tidak lagi menjatuhkan job lain di batch.
func NewWorkerSingle(q Queue, single func(ctx context.Context, job *SubmissionJob) error) *Worker {
	return NewWorker(q, func(ctx context.Context, jobs []*SubmissionJob) ([]error, error) {
		jobErrs := make([]error, len(jobs))
		for i, job := range jobs {
			jobErrs[i] = single(ctx, job)
		}
		return jobErrs, nil
	})
}

// Run menjalankan loop worker sampai ctx dibatalkan atau Stop dipanggil.
func (w *Worker) Run(ctx context.Context) {
	log.Println("[WORKER] Started")
	for {
		select {
		case <-w.stopChan:
			log.Println("[WORKER] Stopped")
			return
		case <-ctx.Done():
			log.Println("[WORKER] Context cancelled, stopping")
			return
		default:
		}

		batch, err := w.collectBatch(ctx)
		if err != nil {
			log.Printf("[WORKER] dequeue error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		if len(batch) == 0 {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		w.processBatchSafe(ctx, batch)
	}
}

// collectBatch mencoba mengumpulkan hingga batchSize job dengan timeout batchTimeout.
// Mengembalikan (nil, nil) jika queue kosong.
func (w *Worker) collectBatch(ctx context.Context) ([]*SubmissionJob, error) {
	deadline := time.Now().Add(w.batchTimeout)
	var batch []*SubmissionJob
	for len(batch) < w.batchSize {
		if time.Now().After(deadline) && len(batch) > 0 {
			break
		}
		job, err := w.queue.Dequeue(ctx)
		if err != nil {
			return batch, err
		}
		if job == nil {
			if len(batch) > 0 {
				return batch, nil
			}
			return nil, nil
		}
		batch = append(batch, job)
	}
	return batch, nil
}

// processBatchSafe memanggil processFunc dengan defer recover untuk menangkap panic, lalu
// menerapkan outcome PER JOB:
//   - job tanpa error  -> MarkCompleted (pindah ke done/)
//   - job dengan error -> MarkFailed (retry dengan backoff, atau failed/ setelah maxRetries)
//
// Dua kasus tetap berlaku untuk seluruh batch, karena keduanya membuat state transaksi tidak
// dapat dipercaya: error tingkat transaksi (mis. Commit gagal) dan panic.
// Memenuhi Requirement 6.1, 6.2, 6.3, 6.4.
func (w *Worker) processBatchSafe(ctx context.Context, batch []*SubmissionJob) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.Printf("[WORKER] PANIC recovered: %v\n%s", r, stack)
			panicErr := fmt.Errorf("worker panic: %v", r)
			w.markBatchFailed(ctx, batch, panicErr)
		}
	}()

	jobErrs, txErr := w.processFunc(ctx, batch)
	if txErr != nil {
		// Transaction-level failure: nothing in this batch was persisted.
		log.Printf("[WORKER] batch transaction error: %v", txErr)
		w.markBatchFailed(ctx, batch, txErr)
		return
	}

	for i, job := range batch {
		var jobErr error
		if i < len(jobErrs) {
			jobErr = jobErrs[i]
		}
		if jobErr != nil {
			log.Printf("[WORKER] job process error: job=%d no_id=%s: %v", job.ID, job.NoID, jobErr)
			if mErr := w.queue.MarkFailed(ctx, job.ID, jobErr); mErr != nil {
				log.Printf("[WORKER] MarkFailed job=%d: %v", job.ID, mErr)
			}
			continue
		}
		if mErr := w.queue.MarkCompleted(ctx, job.ID); mErr != nil {
			log.Printf("[WORKER] MarkCompleted job=%d: %v", job.ID, mErr)
		}
	}
}

// markBatchFailed applies MarkFailed to every job in the batch. Reserved for failures whose
// blast radius really is the whole batch: a transaction-level error or a panic.
func (w *Worker) markBatchFailed(ctx context.Context, batch []*SubmissionJob, cause error) {
	for _, job := range batch {
		if mErr := w.queue.MarkFailed(ctx, job.ID, cause); mErr != nil {
			log.Printf("[WORKER] MarkFailed job=%d: %v", job.ID, mErr)
		}
	}
}

// Stop menghentikan worker secara graceful. Idempotent via sync.Once (klausa 2.8).
func (w *Worker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopChan)
	})
}
