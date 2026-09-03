package submission

// Feature: codebase-bug-sweep, Property 1
//
// **Property 1: Bug Condition** - C4 isolasi kegagalan per job.
//
// Klausa 1.4 (F): ProcessBatch me-rollback SELURUH transaksi pada error pertama dan
// mengembalikan satu error, lalu processBatchSafe memanggil MarkFailed untuk SETIAP job di
// batch. Akibatnya sampai batchSize-1 submission yang sepenuhnya valid ikut di-retry dan
// akhirnya berakhir di failed/ — kehilangan hasil ujian nyata secara silent.
//
// Klausa 2.4 (F'): kegagalan diisolasi pada job yang benar-benar gagal; job valid lain di
// batch yang sama tercommit dan berpindah ke done/.
//
// Validates: Requirements 2.4, 3.6

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"

	_ "modernc.org/sqlite"
)

// setupIsolationDB creates the minimal schema plus n seeded students, each with an active
// session whose attempt_token is derivable from the index.
func setupIsolationDB(t *rapid.T, dir string, n int) *sql.DB {
	db, err := sql.Open("sqlite", "file:"+dir+"/isolation.db")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
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
	return db
}

func isolationNoID(i int) string    { return fmt.Sprintf("OK-%03d", i) }
func isolationToken(i int) string   { return fmt.Sprintf("tok-%03d", i) }
func isolationGhostID(i int) string { return fmt.Sprintf("GHOST-%03d", i) }

// TestPropertyBatchFailureIsolation generates batches of random size with a random subset of
// jobs that must fail (their peserta does not exist), then asserts:
//
//	done/     == exactly the set of valid jobs
//	MarkFailed == exactly the set of failing jobs
//
// Validates: Requirements 2.4, 3.6
func TestPropertyBatchFailureIsolation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(2, 6).Draw(rt, "batch_size")
		failing := rapid.SliceOfN(rapid.Bool(), n, n).Draw(rt, "failing_mask")
		// At least one valid and one failing job, otherwise the property is vacuous.
		failing[rapid.IntRange(0, n-1).Draw(rt, "forced_failure_idx")] = true
		hasValid := false
		for _, f := range failing {
			if !f {
				hasValid = true
			}
		}
		if !hasValid {
			failing[0] = false
		}

		dir := t.TempDir()
		db := setupIsolationDB(rt, dir, n)
		defer db.Close()

		q, err := NewFilesystemQueueWithConfig(dir+"/queue", FilesystemQueueConfig{MaxRetries: 5})
		if err != nil {
			rt.Fatalf("NewFilesystemQueueWithConfig: %v", err)
		}
		ctx := context.Background()

		wantDone := make(map[string]bool)
		wantFailed := make(map[string]bool)
		for i := 0; i < n; i++ {
			job := &SubmissionJob{
				TenantID:     1,
				Score:        "80",
				MaxScore:     "100",
				AttemptToken: isolationToken(i),
			}
			if failing[i] {
				// peserta not found -> a genuine per-job failure.
				job.NoID = isolationGhostID(i)
				job.Validasi = fmt.Sprintf("1_%s_7", job.NoID)
				wantFailed[job.NoID] = true
			} else {
				job.NoID = isolationNoID(i)
				job.Validasi = fmt.Sprintf("1_%s_7", job.NoID)
				wantDone[job.NoID] = true
			}
			if err := q.Enqueue(ctx, job); err != nil {
				rt.Fatalf("Enqueue: %v", err)
			}
		}

		batch, err := q.DequeueBatch(ctx, n)
		if err != nil {
			rt.Fatalf("DequeueBatch: %v", err)
		}
		if len(batch) != n {
			rt.Fatalf("DequeueBatch returned %d jobs, want %d", len(batch), n)
		}

		worker := NewWorker(q, NewProcessor(db).ProcessBatch)
		worker.processBatchSafe(ctx, batch)

		gotDone := namesByNoID(rt, q.doneDir)
		// A failed job with retry_count=1 < maxRetries goes back to pending/.
		gotRetrying := namesByNoID(rt, q.pendingDir)

		if !sameSet(gotDone, wantDone) {
			rt.Fatalf("done/ = %v, want exactly the valid jobs %v (failure was not isolated: "+
				"valid submissions in the same batch were rolled back)", keys(gotDone), keys(wantDone))
		}
		if !sameSet(gotRetrying, wantFailed) {
			rt.Fatalf("pending/ (retrying) = %v, want exactly the failing jobs %v "+
				"(MarkFailed was applied to jobs that did not fail)", keys(gotRetrying), keys(wantFailed))
		}

		var persisted int
		if err := db.QueryRow(`SELECT COUNT(*) FROM hasil_tes`).Scan(&persisted); err != nil {
			rt.Fatalf("count hasil_tes: %v", err)
		}
		if persisted != len(wantDone) {
			rt.Fatalf("hasil_tes rows = %d, want %d (one per valid job)", persisted, len(wantDone))
		}
	})
}

// namesByNoID maps the no_id segment of every *.json file in dir to true.
func namesByNoID(t *rapid.T, dir string) map[string]bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	out := make(map[string]bool)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		// <unix_nano>-<tenant_id>-<no_id>-<8hex>[-due<unix_nano>].json
		parts := strings.Split(strings.TrimSuffix(name, ".json"), "-")
		if len(parts) < 4 {
			t.Fatalf("unexpected queue file name: %q", name)
		}
		out[parts[2]+"-"+parts[3]] = true
	}
	return out
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
