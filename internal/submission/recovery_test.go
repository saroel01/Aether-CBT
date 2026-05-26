package submission

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func writeJobFile(t *testing.T, dir, name string, enqueuedAt time.Time) {
	t.Helper()
	data, err := MarshalJob(&SubmissionJob{
		Validasi:     "1_student_7",
		TenantID:     1,
		NoID:         "student",
		Score:        "80",
		MaxScore:     "100",
		AttemptToken: "token",
		EnqueuedAt:   enqueuedAt.UTC(),
	})
	if err != nil {
		t.Fatalf("MarshalJob: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func TestRecoverStartupPromotesOnlyStuckProcessingFiles(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	q.stuckThreshold = 5 * time.Minute

	oldName := "0000000000000000001-1-old-12345678.json"
	freshName := "0000000000000000002-1-fresh-12345678.json"
	writeJobFile(t, q.processingDir, oldName, time.Now().Add(-10*time.Minute))
	writeJobFile(t, q.processingDir, freshName, time.Now())
	oldTime := time.Now().Add(-10 * time.Minute)
	freshTime := time.Now()
	if err := os.Chtimes(filepath.Join(q.processingDir, oldName), oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes old: %v", err)
	}
	if err := os.Chtimes(filepath.Join(q.processingDir, freshName), freshTime, freshTime); err != nil {
		t.Fatalf("Chtimes fresh: %v", err)
	}

	if err := q.RecoverStartup(ctx, false); err != nil {
		t.Fatalf("RecoverStartup: %v", err)
	}

	if _, err := os.Stat(filepath.Join(q.pendingDir, oldName)); err != nil {
		t.Fatalf("old processing file was not promoted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(q.processingDir, freshName)); err != nil {
		t.Fatalf("fresh processing file should remain in processing: %v", err)
	}
}

func TestRecoverStartupForcePromotesAllProcessingAndCleansTmp(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	writeJobFile(t, q.processingDir, "0000000000000000001-1-a-12345678.json", time.Now())
	if err := os.WriteFile(filepath.Join(q.tmpDir, "leftover.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile tmp: %v", err)
	}

	if err := q.RecoverStartup(ctx, true); err != nil {
		t.Fatalf("RecoverStartup force: %v", err)
	}

	stats, err := q.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.PendingCount != 1 || stats.ProcessingCount != 0 {
		t.Fatalf("stats after force recovery = %+v, want pending=1 processing=0", stats)
	}
	entries, err := os.ReadDir(q.tmpDir)
	if err != nil {
		t.Fatalf("ReadDir tmp: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("tmp entries after recovery = %d, want 0", len(entries))
	}
}

func TestMigrateLegacyTableMovesPendingAndProcessingRowsOnly(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE submission_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER NOT NULL,
			no_id TEXT NOT NULL,
			score TEXT,
			max_score TEXT,
			detail_xml TEXT,
			attempt_token TEXT,
			validasi TEXT NOT NULL,
			retry_count INTEGER DEFAULT 0,
			last_error TEXT,
			status TEXT DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO submission_queue (tenant_id, no_id, score, max_score, detail_xml, attempt_token, validasi, retry_count, last_error, status)
		VALUES
			(1, 'pending-student', '80', '100', '<xml/>', 'tok1', '1_pending_7', 1, 'lock', 'pending'),
			(1, 'processing-student', '81', '100', '', 'tok2', '1_processing_7', 2, '', 'processing'),
			(1, 'completed-student', '82', '100', '', 'tok3', '1_completed_7', 0, '', 'completed'),
			(1, 'failed-student', '83', '100', '', 'tok4', '1_failed_7', 0, '', 'failed');
	`)
	if err != nil {
		t.Fatalf("seed legacy table: %v", err)
	}

	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}
	if err := q.MigrateLegacyTable(ctx, db); err != nil {
		t.Fatalf("MigrateLegacyTable: %v", err)
	}

	stats, err := q.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.PendingCount != 2 {
		t.Fatalf("pending files after migration = %d, want 2", stats.PendingCount)
	}
	var remainingMigrated int
	if err := db.QueryRow("SELECT COUNT(*) FROM submission_queue WHERE status IN ('pending','processing')").Scan(&remainingMigrated); err != nil {
		t.Fatalf("count migrated statuses: %v", err)
	}
	if remainingMigrated != 0 {
		t.Fatalf("legacy pending/processing rows remaining = %d, want 0", remainingMigrated)
	}
	var untouched int
	if err := db.QueryRow("SELECT COUNT(*) FROM submission_queue WHERE status IN ('completed','failed')").Scan(&untouched); err != nil {
		t.Fatalf("count untouched statuses: %v", err)
	}
	if untouched != 2 {
		t.Fatalf("legacy completed/failed rows remaining = %d, want 2", untouched)
	}
}

func TestMigrateLegacyTableSkipsWhenTableMissing(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	q, err := NewFilesystemQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFilesystemQueue: %v", err)
	}

	if err := q.MigrateLegacyTable(ctx, db); err != nil {
		t.Fatalf("MigrateLegacyTable without table: %v", err)
	}
}
