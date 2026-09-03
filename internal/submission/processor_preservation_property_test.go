package submission

// Feature: codebase-bug-sweep, Property 2
//
// **Property 2: Preservation** - Atomisitas batch seluruhnya valid.
//
// Klausa 3.5: batch yang seluruhnya valid tetap ditulis dalam SATU transaksi atomic. Isolasi
// per-job (klausa 2.4) diimplementasikan dengan SAVEPOINT di dalam transaksi yang sama, bukan
// dengan satu transaksi per job, sehingga jalur normal tetap satu BeginTx/Commit — satu fsync
// WAL. Regresi ke satu-transaksi-per-job akan langsung terlihat di sini.
//
// Klausa 3.4: job yang sukses pada percobaan pertama pindah ke done/ tanpa penundaan tambahan.
// Klausa 3.6: job yang gagal permanen tetap berakhir di failed/ setelah maxRetries.
//
// Validates: Requirements 3.4, 3.5, 3.6

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"

	sqlite "modernc.org/sqlite"
)

// --- transaction counting driver -------------------------------------------------------
//
// database/sql gives no visibility into how many transactions a call opened, so the property
// wraps the sqlite driver and counts BeginTx/Commit/Rollback at the driver boundary. This is
// the only way to assert "exactly one commit per batch" rather than merely "the data landed".

type txCounter struct {
	mu        sync.Mutex
	begins    int
	commits   int
	rollbacks int
}

func (c *txCounter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.begins, c.commits, c.rollbacks = 0, 0, 0
}

func (c *txCounter) snapshot() (begins, commits, rollbacks int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.begins, c.commits, c.rollbacks
}

func (c *txCounter) addBegin()    { c.mu.Lock(); c.begins++; c.mu.Unlock() }
func (c *txCounter) addCommit()   { c.mu.Lock(); c.commits++; c.mu.Unlock() }
func (c *txCounter) addRollback() { c.mu.Lock(); c.rollbacks++; c.mu.Unlock() }

var countingTxStats = &txCounter{}

type countingDriver struct{ inner driver.Driver }

func (d countingDriver) Open(dsn string) (driver.Conn, error) {
	conn, err := d.inner.Open(dsn)
	if err != nil {
		return nil, err
	}
	return countingConn{Conn: conn}, nil
}

type countingConn struct{ driver.Conn }

func (c countingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	beginner, ok := c.Conn.(driver.ConnBeginTx)
	if !ok {
		return nil, errors.New("countingConn: wrapped conn does not implement ConnBeginTx")
	}
	tx, err := beginner.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	countingTxStats.addBegin()
	return countingTx{Tx: tx}, nil
}

type countingTx struct{ driver.Tx }

func (t countingTx) Commit() error {
	err := t.Tx.Commit()
	if err == nil {
		countingTxStats.addCommit()
	}
	return err
}

func (t countingTx) Rollback() error {
	err := t.Tx.Rollback()
	if err == nil {
		countingTxStats.addRollback()
	}
	return err
}

func init() {
	sql.Register("sqlite-counting", countingDriver{inner: &sqlite.Driver{}})
}

// fataler is satisfied by both *testing.T and *rapid.T, so the fixtures below serve the
// property test and the unit tests alike.
type fataler interface {
	Fatalf(format string, args ...any)
}

// openCountingDB opens a file-backed database through the counting driver. A single connection
// keeps the transaction counts unambiguous.
func openCountingDB(t fataler, dir string) *sql.DB {
	db, err := sql.Open("sqlite-counting", "file:"+filepath.ToSlash(filepath.Join(dir, "preservation.db")))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
	return db
}

// --- Property 2 -------------------------------------------------------------------------

// TestPropertyAllValidBatchIsOneTransaction generates all-valid batches of random size and
// asserts exactly ONE BeginTx and ONE Commit per batch, with every job persisted.
//
// Validates: Requirements 3.5
func TestPropertyAllValidBatchIsOneTransaction(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(rt, "batch_size")

		dir := t.TempDir()
		db := openCountingDB(rt, dir)
		defer db.Close()
		seedPreservationSchema(rt, db, n)

		jobs := make([]*SubmissionJob, 0, n)
		for i := 0; i < n; i++ {
			jobs = append(jobs, &SubmissionJob{
				TenantID:     1,
				NoID:         isolationNoID(i),
				Validasi:     fmt.Sprintf("1_%s_7", isolationNoID(i)),
				Score:        "80",
				MaxScore:     "100",
				AttemptToken: isolationToken(i),
			})
		}

		countingTxStats.reset()
		jobErrs, txErr := NewProcessor(db).ProcessBatch(context.Background(), jobs)
		begins, commits, rollbacks := countingTxStats.snapshot()

		if txErr != nil {
			rt.Fatalf("txErr = %v, want nil for an all-valid batch", txErr)
		}
		for i, err := range jobErrs {
			if err != nil {
				rt.Fatalf("jobErrs[%d] = %v, want nil for an all-valid batch", i, err)
			}
		}
		if begins != 1 || commits != 1 {
			rt.Fatalf("batch of %d valid jobs used begins=%d commits=%d rollbacks=%d, "+
				"want exactly 1 begin and 1 commit (per-job savepoints must stay inside ONE transaction)",
				n, begins, commits, rollbacks)
		}

		var persisted int
		if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes`).Scan(&persisted); err != nil {
			rt.Fatalf("count hasil_tes: %v", err)
		}
		if persisted != n {
			rt.Fatalf("hasil_tes rows = %d, want %d", persisted, n)
		}
	})
}

// seedPreservationSchema creates the processor's schema plus n students with active sessions.
func seedPreservationSchema(t fataler, db *sql.DB, n int) {
	schemas := []string{
		`CREATE TABLE peserta (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, no_id TEXT NOT NULL, password TEXT, nama_peserta TEXT, kelas_id INTEGER, ruang_id INTEGER);`,
		`CREATE TABLE mapel (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, nama_mapel TEXT, durasi_menit INTEGER DEFAULT 90);`,
		`CREATE TABLE cek_login (id INTEGER PRIMARY KEY, tenant_id INTEGER NOT NULL, peserta_id INTEGER NOT NULL, mapel_id INTEGER NOT NULL, session_id INTEGER, attempt_token TEXT, login_time DATETIME DEFAULT CURRENT_TIMESTAMP);`,
		`CREATE TABLE hasil_tes (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, peserta_id INTEGER NOT NULL, mapel_id INTEGER NOT NULL, exam_session_id INTEGER, skor REAL, skor_maks REAL, detail_xml TEXT, status TEXT, validasi TEXT NOT NULL, waktu_selesai DATETIME, UNIQUE(tenant_id, validasi));`,
		`CREATE TABLE hasil_tes_detail (id INTEGER PRIMARY KEY AUTOINCREMENT, hasil_tes_id INTEGER NOT NULL, question_id TEXT NOT NULL, question_text TEXT, question_type TEXT, status TEXT, awarded_points REAL, max_points REAL, user_answer TEXT, correct_answer TEXT);`,
	}
	for _, schema := range schemas {
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO mapel (id, tenant_id, nama_mapel, durasi_menit) VALUES (7, 1, 'Matematika', 90)`); err != nil {
		t.Fatalf("seed mapel: %v", err)
	}
	for i := 0; i < n; i++ {
		noID := isolationNoID(i)
		if _, err := db.Exec(`INSERT INTO peserta (tenant_id, no_id) VALUES (1, ?)`, noID); err != nil {
			t.Fatalf("seed peserta: %v", err)
		}
		var pesertaID int
		if err := db.QueryRow(`SELECT id FROM peserta WHERE no_id = ? AND tenant_id = 1`, noID).Scan(&pesertaID); err != nil {
			t.Fatalf("lookup peserta: %v", err)
		}
		if _, err := db.Exec(
			`INSERT INTO cek_login (tenant_id, peserta_id, mapel_id, attempt_token, login_time) VALUES (1, ?, 7, ?, ?)`,
			pesertaID, isolationToken(i), time.Now().UTC().Add(-5*time.Minute),
		); err != nil {
			t.Fatalf("seed cek_login: %v", err)
		}
	}
}

// --- preservation unit tests ------------------------------------------------------------

// TestFirstAttemptSuccessMovesToDoneWithoutDelay covers clause 3.4: a job that succeeds on its
// first attempt is dequeued immediately (no due-time gate) and lands in done/ under its plain
// name — the backoff segment is added only by MarkFailed, so an unfailed job is never delayed.
func TestFirstAttemptSuccessMovesToDoneWithoutDelay(t *testing.T) {
	ctx := context.Background()
	clock := newTestClock()
	q, err := NewFilesystemQueueWithConfig(t.TempDir(), FilesystemQueueConfig{Now: clock.Now})
	if err != nil {
		t.Fatalf("NewFilesystemQueueWithConfig: %v", err)
	}

	if err := q.Enqueue(ctx, testJob("no-delay")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// No clock advance at all: a first attempt must be immediately dequeuable.
	job, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if job == nil {
		t.Fatal("Dequeue returned nil for a first attempt; a never-failed job must not be delayed")
	}
	if _, _, hasDue := splitDueTime(job.fileName); hasDue {
		t.Fatalf("first-attempt file %q carries a due-time segment; only MarkFailed may add one", job.fileName)
	}
	if err := q.MarkCompleted(ctx, job.ID); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	doneNames := jsonNames(t, q.doneDir)
	if len(doneNames) != 1 {
		t.Fatalf("done/ = %v, want exactly one file", doneNames)
	}
	if doneNames[0] != job.fileName {
		t.Fatalf("done file = %q, want the enqueued name %q", doneNames[0], job.fileName)
	}
	if strings.Contains(doneNames[0], dueSegmentPrefix) {
		t.Fatalf("done file %q carries a due-time segment", doneNames[0])
	}
}

// TestPermanentFailureReachesFailedAfterMaxRetries covers clause 3.6: a job that keeps failing
// still ends up in failed/ once maxRetries is exhausted, now paced by the backoff schedule.
func TestPermanentFailureReachesFailedAfterMaxRetries(t *testing.T) {
	ctx := context.Background()
	clock := newTestClock()
	const maxRetries = 4
	q, err := NewFilesystemQueueWithConfig(t.TempDir(), FilesystemQueueConfig{
		MaxRetries: maxRetries,
		Now:        clock.Now,
	})
	if err != nil {
		t.Fatalf("NewFilesystemQueueWithConfig: %v", err)
	}

	if err := q.Enqueue(ctx, testJob("doomed")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		job, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("Dequeue attempt %d: %v", attempt, err)
		}
		if job == nil {
			t.Fatalf("Dequeue attempt %d returned nil; the job should be due by now", attempt)
		}
		if err := q.MarkFailed(ctx, job.ID, errors.New("permanent failure")); err != nil {
			t.Fatalf("MarkFailed attempt %d: %v", attempt, err)
		}
		clock.Advance(expectedBackoff(attempt))
	}

	stats, err := q.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.FailedCount != 1 || stats.PendingCount != 0 || stats.ProcessingCount != 0 {
		t.Fatalf("stats = %+v, want failed=1 pending=0 processing=0 after %d retries", stats, maxRetries)
	}
	failedNames := jsonNames(t, q.failedDir)
	if len(failedNames) != 1 {
		t.Fatalf("failed/ = %v, want exactly one file", failedNames)
	}
	if strings.Contains(failedNames[0], dueSegmentPrefix) {
		t.Fatalf("dead-lettered file %q carries a due-time segment; scheduling has no meaning in failed/", failedNames[0])
	}
	data, err := os.ReadFile(filepath.Join(q.failedDir, failedNames[0]))
	if err != nil {
		t.Fatalf("ReadFile failed job: %v", err)
	}
	deadJob, err := UnmarshalJob(data)
	if err != nil {
		t.Fatalf("UnmarshalJob: %v", err)
	}
	if deadJob.RetryCount != maxRetries {
		t.Fatalf("retry_count = %d, want %d", deadJob.RetryCount, maxRetries)
	}
	if deadJob.LastError != "permanent failure" {
		t.Fatalf("last_error = %q, want %q", deadJob.LastError, "permanent failure")
	}
}
