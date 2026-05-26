package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// verify_e2e.go — Build-tagged separate harness invoked via env switch RUN_E2E=1.
// Runs the queue-based webhook end-to-end: HTTP burst -> wait for drain -> verify
// hasil_tes count -> verify failed_submissions count -> measure drain wall-clock.

var (
	e2eRun         = flag.Bool("e2e", false, "Run end-to-end verification harness")
	e2eScale       = flag.Int("scale", 500, "Single E2E student count. Ignored when --e2e-sizes is set")
	e2eSizes       = flag.String("e2e-sizes", "", "Comma-separated student counts to test")
	e2eMaxDrainSec = flag.Int("e2e-max-drain-sec", 600, "Max seconds to wait for queue drain per scale")
	e2eOutput      = flag.String("e2e-output", "tests/load/E2E_RESULTS.md", "Output markdown path")
	e2eQueueDir    = flag.String("queue-dir", "data/queue", "Filesystem queue root used by the server")
)

type e2eScaleResult struct {
	N                 int
	HTTPDuration      time.Duration
	HTTP200Count      int
	HTTPNon200Count   int
	HTTPAvgLatency    time.Duration
	HTTPP95Latency    time.Duration
	HTTPMaxLatency    time.Duration
	DrainDuration     time.Duration
	HasilTesCount     int
	HasilDetailCount  int
	FailedSubmissions int
	StuckProcessing   int
	StuckPending      int
	WALSizeBytes      int64
	DBSizeBytes       int64
	Notes             string
}

func runE2E(client *LoadClient, dp *DataPrep) {
	sizesValue := *e2eSizes
	if strings.TrimSpace(sizesValue) == "" {
		sizesValue = fmt.Sprintf("%d", *e2eScale)
	}
	sizesRaw := strings.Split(sizesValue, ",")
	scales := make([]int, 0, len(sizesRaw))
	for _, s := range sizesRaw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		var n int
		fmt.Sscanf(s, "%d", &n)
		if n > 0 {
			scales = append(scales, n)
		}
	}
	if len(scales) == 0 {
		fmt.Println("No valid scales given via --e2e-sizes")
		os.Exit(1)
	}

	results := make([]e2eScaleResult, 0, len(scales))

	for _, n := range scales {
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("E2E SCALE: %d students (one-shot burst + drain wait)\n", n)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		r := runE2EScale(client, dp, n)
		results = append(results, r)

		printE2EScaleSummary(r)

		// Cooldown so DB has chance to checkpoint WAL before next scale
		time.Sleep(3 * time.Second)
	}

	writeE2EReport(*e2eOutput, results)
	fmt.Printf("\nReport written to %s\n", *e2eOutput)
}

func runE2EScale(client *LoadClient, dp *DataPrep, n int) e2eScaleResult {
	r := e2eScaleResult{N: n}
	prefix := fmt.Sprintf("E2E%d_", n)

	// Aggressive cleanup before scale starts (in case previous run left garbage)
	dp.Cleanup(prefix)
	cleanupQueuePrefix(*e2eQueueDir, prefix)

	fmt.Printf("  [prep] creating %d students with active sessions...\n", n)
	students, err := dp.CreateStudents(n, prefix)
	if err != nil {
		r.Notes = fmt.Sprintf("CreateStudents failed: %v", err)
		return r
	}
	students, err = dp.CreateSessions(students)
	if err != nil {
		r.Notes = fmt.Sprintf("CreateSessions failed: %v", err)
		dp.Cleanup(prefix)
		return r
	}
	// Verify all sessions are committed and visible BEFORE we let the burst start.
	// This eliminates "race between session insert visibility and worker dequeue".
	var actualSessions int
	_ = dp.DB.QueryRow(`
		SELECT COUNT(*) FROM cek_login
		WHERE tenant_id = ? AND peserta_id IN (SELECT id FROM peserta WHERE no_id LIKE ? AND tenant_id = ?)
	`, dp.TenantID, prefix+"%", dp.TenantID).Scan(&actualSessions)
	if actualSessions != len(students) {
		fmt.Printf("  [prep] WARNING: only %d/%d sessions visible after insert\n", actualSessions, len(students))
	}
	// Force WAL checkpoint so the server reads from main DB rather than WAL window.
	_, _ = dp.DB.Exec("PRAGMA wal_checkpoint(PASSIVE)")
	fmt.Printf("  [prep] %d sessions ready (verified=%d)\n", len(students), actualSessions)

	// Get baseline counts so we measure deltas only
	baselineHasil := countRows(dp.DB, "SELECT COUNT(*) FROM hasil_tes WHERE peserta_id IN (SELECT id FROM peserta WHERE no_id LIKE ? AND tenant_id = ?)", prefix+"%", dp.TenantID)
	baselineFailed := countQueueFilesWithPrefix(*e2eQueueDir, "failed", prefix)

	// One-shot burst
	fmt.Printf("  [burst] firing %d concurrent webhook submissions...\n", n)
	burstStart := time.Now()

	type httpResult struct {
		latency time.Duration
		code    int
		ok      bool
		errMsg  string
	}
	resultsCh := make(chan httpResult, n)

	var wg sync.WaitGroup
	for i := range students {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := students[idx]
			score := fmt.Sprintf("%.2f", 50+rand.Float64()*50)
			xmlData := []byte(buildISpringXML(s.NoID, fmt.Sprintf("E2E Student %d", s.PesertaID)))

			start := time.Now()
			code, herr := client.SubmitWebhook(s.NoID, score, "100", xmlData, s.AttemptToken)
			lat := time.Since(start)
			errMsg := ""
			if herr != nil {
				errMsg = herr.Error()
			}
			resultsCh <- httpResult{latency: lat, code: code, ok: herr == nil && code == 200, errMsg: errMsg}
		}(i)
	}
	wg.Wait()
	close(resultsCh)

	r.HTTPDuration = time.Since(burstStart)

	// Aggregate HTTP stats
	var ok200 atomic.Int64
	var bad atomic.Int64
	var totalLat time.Duration
	var maxLat time.Duration
	var firstErr string
	allLat := make([]time.Duration, 0, n)

	for hr := range resultsCh {
		if hr.ok {
			ok200.Add(1)
		} else {
			bad.Add(1)
			if firstErr == "" {
				firstErr = hr.errMsg
			}
		}
		totalLat += hr.latency
		if hr.latency > maxLat {
			maxLat = hr.latency
		}
		allLat = append(allLat, hr.latency)
	}
	r.HTTP200Count = int(ok200.Load())
	r.HTTPNon200Count = int(bad.Load())
	if firstErr != "" {
		r.Notes = "First webhook error: " + firstErr
	}
	if len(allLat) > 0 {
		r.HTTPAvgLatency = totalLat / time.Duration(len(allLat))
		r.HTTPMaxLatency = maxLat
		// Sort for percentiles
		sortDurations(allLat)
		r.HTTPP95Latency = allLat[percentileIndex(len(allLat), 95)]
	}

	fmt.Printf("  [burst] done in %v: 200=%d, non-200=%d, p95=%v\n",
		r.HTTPDuration.Round(time.Millisecond), r.HTTP200Count, r.HTTPNon200Count, r.HTTPP95Latency.Round(time.Millisecond))

	// Wait for drain
	fmt.Printf("  [drain] waiting for queue to drain (max %ds)...\n", *e2eMaxDrainSec)
	drainStart := time.Now()
	deadline := drainStart.Add(time.Duration(*e2eMaxDrainSec) * time.Second)
	lastReport := drainStart

	for {
		now := time.Now()
		pending, processing := countQueueStateWithPrefix(*e2eQueueDir, prefix)
		if pending+processing == 0 {
			r.DrainDuration = time.Since(drainStart)
			fmt.Printf("  [drain] queue empty after %v\n", r.DrainDuration.Round(time.Millisecond))
			break
		}
		if now.After(deadline) {
			r.DrainDuration = time.Since(drainStart)
			r.StuckPending = pending
			r.StuckProcessing = processing
			r.Notes = fmt.Sprintf("Drain timeout: %d pending + %d processing remaining", r.StuckPending, r.StuckProcessing)
			fmt.Printf("  [drain] TIMEOUT after %v: %s\n", r.DrainDuration.Round(time.Second), r.Notes)
			break
		}
		if now.Sub(lastReport) > 5*time.Second {
			fmt.Printf("    [drain] still queued: pending=%d processing=%d (elapsed %v)\n", pending, processing, time.Since(drainStart).Round(time.Second))
			lastReport = now
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Final verification
	finalHasil := countRows(dp.DB, "SELECT COUNT(*) FROM hasil_tes WHERE peserta_id IN (SELECT id FROM peserta WHERE no_id LIKE ? AND tenant_id = ?)", prefix+"%", dp.TenantID)
	finalFailed := countQueueFilesWithPrefix(*e2eQueueDir, "failed", prefix)
	r.HasilTesCount = finalHasil - baselineHasil
	r.FailedSubmissions = finalFailed - baselineFailed
	r.HasilDetailCount = countRows(dp.DB, `
		SELECT COUNT(*) FROM hasil_tes_detail
		WHERE hasil_tes_id IN (
			SELECT id FROM hasil_tes
			WHERE peserta_id IN (SELECT id FROM peserta WHERE no_id LIKE ? AND tenant_id = ?)
		)
	`, prefix+"%", dp.TenantID)

	// Database file sizes (best-effort)
	if fi, err := os.Stat("data/cbt_aether.db"); err == nil {
		r.DBSizeBytes = fi.Size()
	}
	if fi, err := os.Stat("data/cbt_aether.db-wal"); err == nil {
		r.WALSizeBytes = fi.Size()
	}

	// Cleanup test artifacts
	dp.Cleanup(prefix)
	cleanupQueuePrefix(*e2eQueueDir, prefix)
	// Truncate WAL between scales to avoid balloon
	_, _ = dp.DB.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)")

	return r
}

func printE2EScaleSummary(r e2eScaleResult) {
	fmt.Printf("\n  ┌─ Scale %d Results ─────────────────────────────────────\n", r.N)
	fmt.Printf("  │ HTTP burst       : %v (avg=%v, p95=%v, max=%v)\n",
		r.HTTPDuration.Round(time.Millisecond),
		r.HTTPAvgLatency.Round(time.Millisecond),
		r.HTTPP95Latency.Round(time.Millisecond),
		r.HTTPMaxLatency.Round(time.Millisecond),
	)
	fmt.Printf("  │ HTTP 200         : %d / %d (%.1f%%)\n", r.HTTP200Count, r.N, pct(r.HTTP200Count, r.N))
	fmt.Printf("  │ Drain time       : %v\n", r.DrainDuration.Round(time.Millisecond))
	fmt.Printf("  │ hasil_tes saved  : %d / %d (%.1f%%)  ← END-TO-END SUCCESS\n", r.HasilTesCount, r.N, pct(r.HasilTesCount, r.N))
	fmt.Printf("  │ hasil_tes_detail : %d rows\n", r.HasilDetailCount)
	fmt.Printf("  │ failed_subs      : %d\n", r.FailedSubmissions)
	if r.StuckPending+r.StuckProcessing > 0 {
		fmt.Printf("  │ STUCK            : pending=%d processing=%d\n", r.StuckPending, r.StuckProcessing)
	}
	if r.WALSizeBytes > 0 || r.DBSizeBytes > 0 {
		fmt.Printf("  │ DB / WAL size    : %s / %s\n", humanBytes(r.DBSizeBytes), humanBytes(r.WALSizeBytes))
	}
	if r.Notes != "" {
		fmt.Printf("  │ NOTES            : %s\n", r.Notes)
	}
	fmt.Printf("  └────────────────────────────────────────────────────────\n")
}

func writeE2EReport(path string, results []e2eScaleResult) {
	var b strings.Builder
	b.WriteString("# Aether CBT — End-to-End Queue Verification Report\n\n")
	b.WriteString(fmt.Sprintf("**Generated:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("**Methodology:** One-shot burst N concurrent webhook POSTs → wait for queue.drain → count `hasil_tes` rows + `failed_submissions` rows for that scale's prefix.\n\n")
	b.WriteString("This measures TRUE end-to-end success (job actually persisted), not just HTTP 200 acceptance.\n\n")

	b.WriteString("## Results Matrix\n\n")
	b.WriteString("| N | HTTP 200 % | HTTP P95 | Drain Time | hasil_tes Saved | E2E Success % | Failed | Notes |\n")
	b.WriteString("|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("| %d | %.1f%% | %v | %v | %d/%d | **%.1f%%** | %d | %s |\n",
			r.N,
			pct(r.HTTP200Count, r.N),
			r.HTTPP95Latency.Round(time.Millisecond),
			r.DrainDuration.Round(time.Millisecond),
			r.HasilTesCount, r.N,
			pct(r.HasilTesCount, r.N),
			r.FailedSubmissions,
			summaryNote(r),
		))
	}

	b.WriteString("\n## Per-Scale Detail\n\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("### Scale: %d students\n\n", r.N))
		b.WriteString(fmt.Sprintf("- **HTTP burst window**: %v\n", r.HTTPDuration.Round(time.Millisecond)))
		b.WriteString(fmt.Sprintf("- **HTTP latency**: avg %v / p95 %v / max %v\n",
			r.HTTPAvgLatency.Round(time.Millisecond),
			r.HTTPP95Latency.Round(time.Millisecond),
			r.HTTPMaxLatency.Round(time.Millisecond)))
		b.WriteString(fmt.Sprintf("- **HTTP 200 acceptance**: %d / %d (%.1f%%)\n",
			r.HTTP200Count, r.N, pct(r.HTTP200Count, r.N)))
		b.WriteString(fmt.Sprintf("- **Worker drain duration**: %v\n", r.DrainDuration.Round(time.Millisecond)))
		b.WriteString(fmt.Sprintf("- **hasil_tes rows persisted**: %d / %d (**%.1f%% end-to-end success**)\n",
			r.HasilTesCount, r.N, pct(r.HasilTesCount, r.N)))
		b.WriteString(fmt.Sprintf("- **hasil_tes_detail rows**: %d\n", r.HasilDetailCount))
		b.WriteString(fmt.Sprintf("- **failed_submissions delta**: %d\n", r.FailedSubmissions))
		if r.StuckPending+r.StuckProcessing > 0 {
			b.WriteString(fmt.Sprintf("- **Queue stuck**: pending=%d, processing=%d\n", r.StuckPending, r.StuckProcessing))
		}
		if r.DBSizeBytes > 0 || r.WALSizeBytes > 0 {
			b.WriteString(fmt.Sprintf("- **DB / WAL after run**: %s / %s\n", humanBytes(r.DBSizeBytes), humanBytes(r.WALSizeBytes)))
		}
		if r.Notes != "" {
			b.WriteString(fmt.Sprintf("- **Notes**: %s\n", r.Notes))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Interpretasi Cepat\n\n")
	b.WriteString("- **HTTP 200 %** = berapa % submission diterima oleh handler (acceptance/queue insertion).\n")
	b.WriteString("- **E2E Success %** = berapa % yang BENAR-BENAR tersimpan ke `hasil_tes` setelah worker selesai.\n")
	b.WriteString("- Selisih HTTP200 - E2E = job yang gagal di worker (bisa karena bug, lock contention internal, atau pindah ke `failed_submissions`).\n")
	b.WriteString("- **Drain time** = berapa lama worker menyelesaikan semua job. Ini menentukan UX pasca-ujian (kapan admin bisa lihat hasil lengkap).\n")
	b.WriteString("- **Failed** > 0 = submission yang permanently lost (perlu intervensi manual).\n")

	_ = os.WriteFile(path, []byte(b.String()), 0o644)
}

// helpers

func countRows(db *sql.DB, query string, args ...any) int {
	var n int
	err := db.QueryRow(query, args...).Scan(&n)
	if err != nil {
		return -1
	}
	return n
}

func countQueueStateWithPrefix(root, prefix string) (pending, processing int) {
	return countQueueFilesWithPrefix(root, "pending", prefix), countQueueFilesWithPrefix(root, "processing", prefix)
}

func countQueueFilesWithPrefix(root, state, prefix string) int {
	entries, err := os.ReadDir(filepath.Join(root, state))
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".json") && strings.Contains(name, prefix) {
			count++
		}
	}
	return count
}

func cleanupQueuePrefix(root, prefix string) {
	for _, state := range []string{"pending", "processing", "done", "failed", "tmp"} {
		dir := filepath.Join(root, state)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.Contains(name, prefix) {
				_ = os.Remove(filepath.Join(dir, name))
			}
		}
	}
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func sortDurations(d []time.Duration) {
	// Simple insertion sort — n is small (<= 500)
	for i := 1; i < len(d); i++ {
		x := d[i]
		j := i - 1
		for j >= 0 && d[j] > x {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = x
	}
}

func humanBytes(n int64) string {
	if n <= 0 {
		return "—"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func summaryNote(r e2eScaleResult) string {
	if r.Notes != "" {
		return r.Notes
	}
	if r.HasilTesCount == r.N {
		return "all good"
	}
	return fmt.Sprintf("missing %d", r.N-r.HasilTesCount)
}

// suppress unused warning
var _ = url.Values{}
