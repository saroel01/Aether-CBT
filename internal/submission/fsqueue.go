package submission

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// sanitizeRegexp matches characters that are NOT alphanumeric, underscore, or hyphen.
var sanitizeRegexp = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// sanitize replaces any character not in [A-Za-z0-9_-] with '_'.
func sanitize(s string) string {
	return sanitizeRegexp.ReplaceAllString(s, "_")
}

// randomHex returns n hex-encoded bytes from crypto/rand (n bytes → 2n hex chars).
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// FilesystemQueue menyimpan setiap SubmissionJob sebagai file JSON di sub-direktori
// pending/, processing/, done/, failed/. Operasi state-transition dilakukan dengan
// os.Rename atomic untuk menghindari job hilang saat crash.
//
// Memenuhi Requirement 1.6, 9.3, 9.4.
type FilesystemQueue struct {
	root          string // QUEUE_DIR, mis. data/queue
	pendingDir    string
	processingDir string
	doneDir       string
	failedDir     string
	tmpDir        string

	maxRetries     int           // default 5
	stuckThreshold time.Duration // default 5 * time.Minute
	doneRetention  time.Duration // default 7 * 24 * time.Hour

	inFlight map[int64]string // jobID -> fileName
	nextID   int64
	mu       sync.Mutex

	enqueueCh chan enqueueRequest
}

type FilesystemQueueConfig struct {
	MaxRetries     int
	StuckThreshold time.Duration
	DoneRetention  time.Duration
}

type enqueueRequest struct {
	ctx  context.Context
	job  *SubmissionJob
	done chan error
}

// NewFilesystemQueue membuat sub-direktori jika belum ada (Requirement 1.6, 9.3).
// Mengembalikan error jika direktori tidak dapat dibuat.
func NewFilesystemQueue(root string) (*FilesystemQueue, error) {
	return NewFilesystemQueueWithConfig(root, FilesystemQueueConfig{})
}

func NewFilesystemQueueWithConfig(root string, cfg FilesystemQueueConfig) (*FilesystemQueue, error) {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 5
	}
	stuckThreshold := cfg.StuckThreshold
	if stuckThreshold <= 0 {
		stuckThreshold = 5 * time.Minute
	}
	doneRetention := cfg.DoneRetention
	if doneRetention <= 0 {
		doneRetention = 7 * 24 * time.Hour
	}

	q := &FilesystemQueue{
		root:           root,
		pendingDir:     filepath.Join(root, "pending"),
		processingDir:  filepath.Join(root, "processing"),
		doneDir:        filepath.Join(root, "done"),
		failedDir:      filepath.Join(root, "failed"),
		tmpDir:         filepath.Join(root, "tmp"),
		maxRetries:     maxRetries,
		stuckThreshold: stuckThreshold,
		doneRetention:  doneRetention,
		inFlight:       make(map[int64]string),
		enqueueCh:      make(chan enqueueRequest, 2048),
	}

	dirs := []string{
		q.pendingDir,
		q.processingDir,
		q.doneDir,
		q.failedDir,
		q.tmpDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	go q.runEnqueueWriter()

	return q, nil
}

func (q *FilesystemQueue) Enqueue(ctx context.Context, job *SubmissionJob) error {
	if q.enqueueCh == nil {
		return q.enqueueDirect(ctx, job)
	}
	req := enqueueRequest{
		ctx:  ctx,
		job:  job,
		done: make(chan error, 1),
	}
	select {
	case q.enqueueCh <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-req.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *FilesystemQueue) runEnqueueWriter() {
	for req := range q.enqueueCh {
		req.done <- q.enqueueDirect(req.ctx, req.job)
	}
}

// enqueueDirect menulis job ke tmp/, lalu rename atomic ke pending/.
// Implementasi memenuhi Requirement 1.1, 1.2, 1.3, 1.4, 1.5.
func (q *FilesystemQueue) enqueueDirect(ctx context.Context, job *SubmissionJob) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Step 1-2: stamp EnqueuedAt
	now := time.Now().UTC()
	job.EnqueuedAt = now

	// Step 3-5: build filename
	suffix, err := randomHex(4) // 4 bytes → 8 hex chars
	if err != nil {
		return fmt.Errorf("enqueue: generate random suffix: %w", err)
	}
	fileName := fmt.Sprintf("%d-%d-%s-%s.json",
		now.UnixNano(),
		job.TenantID,
		sanitize(job.NoID),
		suffix,
	)

	// Step 6: marshal
	data, err := MarshalJob(job)
	if err != nil {
		return fmt.Errorf("enqueue: marshal job: %w", err)
	}

	// Step 7-9: write to tmp/ with O_WRONLY|O_CREATE|O_EXCL
	tmpPath := filepath.Join(q.tmpDir, fileName)
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("enqueue: create tmp file: %w", err)
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("enqueue: write tmp file: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("enqueue: close tmp file: %w", closeErr)
	}

	// Step 10-12: atomic rename tmp/ → pending/
	pendingPath := filepath.Join(q.pendingDir, fileName)
	if err := os.Rename(tmpPath, pendingPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("enqueue: rename to pending: %w", err)
	}

	return nil
}

// Dequeue memilih file paling lama di pending/ (FIFO via sort filename),
// memindahkannya ke processing/, lalu mengembalikan job yang sudah di-unmarshal.
// Mengembalikan (nil, nil) jika pending kosong.
// Implementasi memenuhi Requirement 2.1, 2.2, 2.3, 2.4.
func (q *FilesystemQueue) Dequeue(ctx context.Context) (*SubmissionJob, error) {
	// os.ReadDir returns entries sorted by name (Go 1.21+).
	// Because filenames are prefixed with unix_nano, sort-by-name == FIFO.
	entries, err := os.ReadDir(q.pendingDir)
	if err != nil {
		return nil, fmt.Errorf("dequeue: read pending dir: %w", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		src := filepath.Join(q.pendingDir, entry.Name())
		dst := filepath.Join(q.processingDir, entry.Name())

		// Atomic claim: rename pending -> processing.
		if err := os.Rename(src, dst); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// Another worker (or concurrent call) already took this file.
				continue
			}
			return nil, fmt.Errorf("dequeue: rename to processing: %w", err)
		}

		// Read the file now that it is safely in processing/.
		data, err := os.ReadFile(dst)
		if err != nil {
			return nil, fmt.Errorf("dequeue: read file: %w", err)
		}

		job, err := UnmarshalJob(data)
		if err != nil {
			// Corrupt file: move to failed/ with a companion .error.txt.
			failedPath := filepath.Join(q.failedDir, entry.Name())
			_ = os.Rename(dst, failedPath)
			errTxtPath := filepath.Join(q.failedDir, strings.TrimSuffix(entry.Name(), ".json")+".error.txt")
			_ = os.WriteFile(errTxtPath, []byte(err.Error()), 0644)
			continue
		}

		job.fileName = entry.Name()

		// Assign an in-memory ID and record the in-flight mapping.
		q.mu.Lock()
		q.nextID++
		job.ID = q.nextID
		q.inFlight[job.ID] = entry.Name()
		q.mu.Unlock()

		return job, nil
	}

	// No pending jobs found.
	return nil, nil
}

// DequeueBatch mengambil hingga maxBatch file dari pending/ secara FIFO.
// Dipakai oleh Worker dalam mode batching (Requirement 17.3).
// Mengembalikan slice kosong jika pending kosong (bukan error).
func (q *FilesystemQueue) DequeueBatch(ctx context.Context, maxBatch int) ([]*SubmissionJob, error) {
	var batch []*SubmissionJob
	for len(batch) < maxBatch {
		job, err := q.Dequeue(ctx)
		if err != nil {
			return batch, err
		}
		if job == nil {
			// Pending queue is empty.
			break
		}
		batch = append(batch, job)
	}
	return batch, nil
}

// MarkCompleted memindahkan file dari processing/ ke done/.
// Implementasi memenuhi Requirement 2.5.
func (q *FilesystemQueue) MarkCompleted(ctx context.Context, jobID int64) error {
	q.mu.Lock()
	fileName, ok := q.inFlight[jobID]
	q.mu.Unlock()

	if !ok {
		return fmt.Errorf("markCompleted: unknown jobID %d", jobID)
	}

	src := filepath.Join(q.processingDir, fileName)
	dst := filepath.Join(q.doneDir, fileName)
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("markCompleted: rename to done: %w", err)
	}

	q.mu.Lock()
	delete(q.inFlight, jobID)
	q.mu.Unlock()

	return nil
}

// MarkFailed menambah retry_count, mengisi last_error, lalu:
//   - jika retry_count < maxRetries: rewrite file via tmp/, rename ke pending/.
//   - jika retry_count >= maxRetries: rewrite + rename ke failed/.
//
// Backoff exponential: EnqueuedAt = now + min(2^(retryCount-1), 30) detik.
// Filename tetap sama agar admin dapat mengkorelasi file antar direktori.
// Implementasi memenuhi Requirement 3.1, 3.2, 3.3, 3.4, 3.5.
func (q *FilesystemQueue) MarkFailed(ctx context.Context, jobID int64, processErr error) error {
	q.mu.Lock()
	fileName, ok := q.inFlight[jobID]
	q.mu.Unlock()

	if !ok {
		return fmt.Errorf("markFailed: unknown jobID %d", jobID)
	}

	// Step 1-3: read and unmarshal the job from processing/
	processingPath := filepath.Join(q.processingDir, fileName)
	data, err := os.ReadFile(processingPath)
	if err != nil {
		return fmt.Errorf("markFailed: read processing file: %w", err)
	}

	job, err := UnmarshalJob(data)
	if err != nil {
		return fmt.Errorf("markFailed: unmarshal job: %w", err)
	}

	// Step 4-6: update retry metadata
	job.RetryCount++
	job.LastError = processErr.Error()

	// Exponential backoff: min(2^(retryCount-1), 30) seconds
	// retryCount is already incremented, so use the new value
	backoffSec := 1 << (job.RetryCount - 1) // 2^(retryCount-1)
	if backoffSec > 30 {
		backoffSec = 30
	}
	job.EnqueuedAt = time.Now().UTC().Add(time.Duration(backoffSec) * time.Second)

	// Step 7-9: marshal and write to tmp/
	newData, err := MarshalJob(job)
	if err != nil {
		return fmt.Errorf("markFailed: marshal job: %w", err)
	}

	tmpPath := filepath.Join(q.tmpDir, fileName)
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("markFailed: create tmp file: %w", err)
	}
	_, writeErr := f.Write(newData)
	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("markFailed: write tmp file: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("markFailed: close tmp file: %w", closeErr)
	}

	// Step 10-13: determine destination and rename
	var dstDir string
	if job.RetryCount >= q.maxRetries {
		dstDir = q.failedDir
	} else {
		dstDir = q.pendingDir
	}

	dstPath := filepath.Join(dstDir, fileName)
	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("markFailed: rename to %s: %w", dstDir, err)
	}

	// Step 14-15: remove old processing file and clean up inFlight
	_ = os.Remove(processingPath)

	q.mu.Lock()
	delete(q.inFlight, jobID)
	q.mu.Unlock()

	return nil
}

// countJSONFiles counts the number of *.json files in the given directory.
// Returns an error if the directory cannot be read.
func countJSONFiles(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read dir %s: %w", dir, err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	return count, nil
}

// GetStats menghitung jumlah file *.json di tiap direktori.
// Implementasi memenuhi Requirement 12.1, 12.2.
// Return error jika ada direktori yang tidak terbaca (Requirement 12.5).
func (q *FilesystemQueue) GetStats(ctx context.Context) (QueueStats, error) {
	pending, err := countJSONFiles(q.pendingDir)
	if err != nil {
		return QueueStats{}, err
	}

	processing, err := countJSONFiles(q.processingDir)
	if err != nil {
		return QueueStats{}, err
	}

	done, err := countJSONFiles(q.doneDir)
	if err != nil {
		return QueueStats{}, err
	}

	failed, err := countJSONFiles(q.failedDir)
	if err != nil {
		return QueueStats{}, err
	}

	return QueueStats{
		PendingCount:    pending,
		ProcessingCount: processing,
		DoneCount:       done,
		FailedCount:     failed,
	}, nil
}

// RecoverStartup cleans leftover tmp files, promotes stuck processing jobs back to
// pending, and applies done/ retention cleanup before the HTTP server starts.
// If forceAll is true, every processing job is promoted regardless of mtime.
func (q *FilesystemQueue) RecoverStartup(ctx context.Context, forceAll bool) error {
	tmpEntries, err := os.ReadDir(q.tmpDir)
	if err != nil {
		return fmt.Errorf("recover startup: read tmp dir: %w", err)
	}
	removedTmp := 0
	for _, entry := range tmpEntries {
		if entry.IsDir() {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		path := filepath.Join(q.tmpDir, entry.Name())
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("recover startup: remove tmp file %s: %w", path, err)
		}
		removedTmp++
	}
	if removedTmp > 0 {
		log.Printf("[QUEUE] recovery removed %d tmp file(s)", removedTmp)
	}

	now := time.Now()
	processingEntries, err := os.ReadDir(q.processingDir)
	if err != nil {
		return fmt.Errorf("recover startup: read processing dir: %w", err)
	}
	promoted := 0
	for _, entry := range processingEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		src := filepath.Join(q.processingDir, entry.Name())
		info, err := os.Stat(src)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("recover startup: stat processing file %s: %w", src, err)
		}
		if !forceAll && now.Sub(info.ModTime()) <= q.stuckThreshold {
			continue
		}
		dst := filepath.Join(q.pendingDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("recover startup: promote %s to pending: %w", src, err)
		}
		promoted++
		log.Printf("[QUEUE] recovery promoted processing job to pending: %s", dst)
	}
	if promoted > 0 {
		log.Printf("[QUEUE] recovery promoted %d processing file(s)", promoted)
	}

	doneEntries, err := os.ReadDir(q.doneDir)
	if err != nil {
		return fmt.Errorf("recover startup: read done dir: %w", err)
	}
	removedDone := 0
	for _, entry := range doneEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(q.doneDir, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("recover startup: stat done file %s: %w", path, err)
		}
		if now.Sub(info.ModTime()) <= q.doneRetention {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("recover startup: remove expired done file %s: %w", path, err)
		}
		removedDone++
	}
	if removedDone > 0 {
		log.Printf("[QUEUE] recovery removed %d expired done file(s)", removedDone)
	}

	return nil
}

// MigrateLegacyTable converts legacy pending/processing rows from submission_queue
// into pending filesystem jobs, then deletes only the migrated rows.
func (q *FilesystemQueue) MigrateLegacyTable(ctx context.Context, db *sql.DB) error {
	var tableName string
	err := db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'submission_queue'
	`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("[QUEUE] legacy submission_queue table not found; skipping migration")
		return nil
	}
	if err != nil {
		return fmt.Errorf("legacy migration: check submission_queue table: %w", err)
	}

	var skipped int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM submission_queue WHERE status NOT IN ('pending', 'processing')
	`).Scan(&skipped); err != nil {
		return fmt.Errorf("legacy migration: count skipped rows: %w", err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, tenant_id, no_id, COALESCE(score, ''), COALESCE(max_score, ''),
		       COALESCE(detail_xml, ''), COALESCE(attempt_token, ''), retry_count,
		       COALESCE(last_error, ''), validasi
		  FROM submission_queue
		 WHERE status IN ('pending', 'processing')
		 ORDER BY id ASC
	`)
	if err != nil {
		return fmt.Errorf("legacy migration: select rows: %w", err)
	}
	defer rows.Close()

	var migratedIDs []int64
	for rows.Next() {
		var job SubmissionJob
		var id int64
		if err := rows.Scan(&id, &job.TenantID, &job.NoID, &job.Score, &job.MaxScore,
			&job.DetailXML, &job.AttemptToken, &job.RetryCount, &job.LastError, &job.Validasi); err != nil {
			return fmt.Errorf("legacy migration: scan row: %w", err)
		}
		if err := q.Enqueue(ctx, &job); err != nil {
			return fmt.Errorf("legacy migration: enqueue row id %d: %w", id, err)
		}
		migratedIDs = append(migratedIDs, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("legacy migration: iterate rows: %w", err)
	}
	if len(migratedIDs) == 0 {
		log.Printf("[QUEUE] legacy migration completed: migrated=0 skipped=%d", skipped)
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("legacy migration: begin delete tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	placeholders := make([]string, len(migratedIDs))
	args := make([]any, len(migratedIDs))
	for i, id := range migratedIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM submission_queue WHERE id IN (%s)", strings.Join(placeholders, ","))
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("legacy migration: delete migrated rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("legacy migration: commit delete tx: %w", err)
	}

	log.Printf("[QUEUE] legacy migration completed: migrated=%d skipped=%d", len(migratedIDs), skipped)
	return nil
}
