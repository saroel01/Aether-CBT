package submission

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"time"
)

// Worker memproses SubmissionJob dari queue secara batch dengan panic recovery.
// Memenuhi Requirement 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 17.3, 17.4.
type Worker struct {
	queue       Queue
	processFunc func(ctx context.Context, jobs []*SubmissionJob) error
	stopChan    chan struct{}

	batchSize    int           // default 5
	batchTimeout time.Duration // default 100ms
}

// NewWorker membuat Worker baru dengan processFunc batch.
// batchSize default 5, batchTimeout default 100ms.
func NewWorker(q Queue, processBatch func(ctx context.Context, jobs []*SubmissionJob) error) *Worker {
	return NewWorkerWithConfig(q, processBatch, 5, 100*time.Millisecond)
}

func NewWorkerWithConfig(q Queue, processBatch func(ctx context.Context, jobs []*SubmissionJob) error, batchSize int, batchTimeout time.Duration) *Worker {
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
func NewWorkerSingle(q Queue, single func(ctx context.Context, job *SubmissionJob) error) *Worker {
	return NewWorker(q, func(ctx context.Context, jobs []*SubmissionJob) error {
		for _, job := range jobs {
			if err := single(ctx, job); err != nil {
				return err
			}
		}
		return nil
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

// processBatchSafe memanggil processFunc dengan defer recover untuk menangkap panic.
// Pada panic: log stack trace, lalu MarkFailed semua job di batch.
// Pada error biasa: MarkFailed semua job di batch.
// Pada sukses: MarkCompleted semua job di batch.
// Memenuhi Requirement 6.1, 6.2, 6.3, 6.4.
func (w *Worker) processBatchSafe(ctx context.Context, batch []*SubmissionJob) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.Printf("[WORKER] PANIC recovered: %v\n%s", r, stack)
			panicErr := fmt.Errorf("worker panic: %v", r)
			for _, job := range batch {
				if mErr := w.queue.MarkFailed(ctx, job.ID, panicErr); mErr != nil {
					log.Printf("[WORKER] MarkFailed job=%d: %v", job.ID, mErr)
				}
			}
		}
	}()

	err := w.processFunc(ctx, batch)
	if err == nil {
		for _, job := range batch {
			if mErr := w.queue.MarkCompleted(ctx, job.ID); mErr != nil {
				log.Printf("[WORKER] MarkCompleted job=%d: %v", job.ID, mErr)
			}
		}
		return
	}
	log.Printf("[WORKER] batch process error: %v", err)
	for _, job := range batch {
		if mErr := w.queue.MarkFailed(ctx, job.ID, err); mErr != nil {
			log.Printf("[WORKER] MarkFailed job=%d: %v", job.ID, mErr)
		}
	}
}

// Stop menghentikan worker secara graceful.
func (w *Worker) Stop() {
	close(w.stopChan)
}
