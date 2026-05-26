package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func runLoginBurst(client *LoadClient, dp *DataPrep, concurrency int, duration time.Duration) {
	scenarioName := fmt.Sprintf("Login Burst (N=%d)", concurrency)
	fmt.Printf("\n>>> Preparing: %s\n", scenarioName)

	examToken := dp.GetExamToken()
	prefix := fmt.Sprintf("LB%d_", concurrency)

	fmt.Printf("  Creating %d test students...\n", concurrency)
	students, err := dp.CreateStudents(concurrency, prefix)
	if err != nil {
		fmt.Printf("  FATAL: Failed to create students: %v\n", err)
		return
	}
	fmt.Printf("  Created %d students. Exam token: %s\n", len(students), examToken)

	defer func() {
		fmt.Println("  Cleaning up...")
		dp.Cleanup(prefix)
	}()

	metrics := NewMetricsCollector()
	progress := NewProgressPrinter(metrics, "login")
	progress.Start(5 * time.Second)
	defer progress.Stop()

	fmt.Printf("  Starting login burst for %v...\n\n", duration)

	var wg sync.WaitGroup
	deadline := time.Now().Add(duration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			student := students[idx%len(students)]

			for time.Now().Before(deadline) {
				start := time.Now()
				_, statusCode, err := client.StudentLogin(student.NoID, student.Password, examToken)
				latency := time.Since(start)

				result := RequestResult{
					Latency:    latency,
					StatusCode: statusCode,
					Endpoint:   "student-login",
					IsError:    err != nil,
				}
				if err != nil {
					result.ErrorMsg = classifyError(err.Error())
				}
				metrics.Record(result)

				jitter := time.Duration(rand.Intn(200)) * time.Millisecond
				time.Sleep(jitter)
			}
		}(i)
	}

	wg.Wait()

	snap := metrics.Snapshot()
	snap.PrintReport(scenarioName, concurrency)
}

func runExamStartBurst(client *LoadClient, dp *DataPrep, concurrency int, duration time.Duration) {
	scenarioName := fmt.Sprintf("Exam Start Burst (N=%d)", concurrency)
	fmt.Printf("\n>>> Preparing: %s\n", scenarioName)

	examToken := dp.GetExamToken()
	prefix := fmt.Sprintf("SB%d_", concurrency)

	fmt.Printf("  Creating %d test students...\n", concurrency)
	students, err := dp.CreateStudents(concurrency, prefix)
	if err != nil {
		fmt.Printf("  FATAL: Failed to create students: %v\n", err)
		return
	}

	fmt.Println("  Logging in all students to get JWT tokens...")
	type loggedIn struct {
		student TestStudent
		jwt     string
	}
	var loggedInStudents []loggedIn
	var loginMu sync.Mutex

	var loginWg sync.WaitGroup
	sem := make(chan struct{}, 20)

	for i := range students {
		loginWg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer loginWg.Done()
			defer func() { <-sem }()
			s := students[idx]
			for attempt := 0; attempt < 3; attempt++ {
				resp, _, err := client.StudentLogin(s.NoID, s.Password, examToken)
				if err == nil {
					loginMu.Lock()
					loggedInStudents = append(loggedInStudents, loggedIn{student: s, jwt: resp.Token})
					loginMu.Unlock()
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}
	loginWg.Wait()

	if len(loggedInStudents) < concurrency/2 {
		fmt.Printf("  FATAL: Only %d/%d students could log in. Aborting.\n", len(loggedInStudents), concurrency)
		dp.Cleanup(prefix)
		return
	}
	fmt.Printf("  %d students logged in successfully.\n", len(loggedInStudents))

	defer func() {
		fmt.Println("  Cleaning up...")
		dp.ClearSessions(prefix)
		dp.Cleanup(prefix)
	}()

	metrics := NewMetricsCollector()
	progress := NewProgressPrinter(metrics, "start")
	progress.Start(5 * time.Second)
	defer progress.Stop()

	fmt.Printf("  Starting exam-start burst for %v...\n\n", duration)

	var wg sync.WaitGroup
	deadline := time.Now().Add(duration)

	for i := 0; i < len(loggedInStudents); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			lg := loggedInStudents[idx%len(loggedInStudents)]

			for time.Now().Before(deadline) {
				start := time.Now()
				_, statusCode, err := client.StartExam(lg.jwt, lg.student.PesertaID, dp.MapelID)
				latency := time.Since(start)

				result := RequestResult{
					Latency:    latency,
					StatusCode: statusCode,
					Endpoint:   "student-start",
					IsError:    err != nil,
				}
				if err != nil {
					result.ErrorMsg = classifyError(err.Error())
				}
				metrics.Record(result)

				jitter := time.Duration(rand.Intn(300)) * time.Millisecond
				time.Sleep(jitter)
			}
		}(i)
	}

	wg.Wait()

	snap := metrics.Snapshot()
	snap.PrintReport(scenarioName, concurrency)
}

func runSubmissionBurst(client *LoadClient, dp *DataPrep, concurrency int, duration time.Duration) {
	scenarioName := fmt.Sprintf("Submission Burst (N=%d)", concurrency)
	fmt.Printf("\n>>> Preparing: %s\n", scenarioName)

	prefix := fmt.Sprintf("WB%d_", concurrency)

	fmt.Printf("  Creating %d test students with active sessions...\n", concurrency)
	students, err := dp.CreateStudents(concurrency, prefix)
	if err != nil {
		fmt.Printf("  FATAL: Failed to create students: %v\n", err)
		return
	}

	students, err = dp.CreateSessions(students)
	if err != nil {
		fmt.Printf("  FATAL: Failed to create sessions: %v\n", err)
		dp.Cleanup(prefix)
		return
	}
	fmt.Printf("  %d students with active sessions ready.\n", len(students))

	defer func() {
		fmt.Println("  Cleaning up...")
		dp.ClearSessions(prefix)
		dp.Cleanup(prefix)
	}()

	metrics := NewMetricsCollector()
	progress := NewProgressPrinter(metrics, "webhook")
	progress.Start(3 * time.Second)
	defer progress.Stop()

	fmt.Printf("  Phase A: One-shot burst (all %d students submit simultaneously)...\n", concurrency)
	burstStart := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < len(students); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := students[idx]
			score := fmt.Sprintf("%.2f", 50+rand.Float64()*50)
			xmlData := []byte(buildISpringXML(s.NoID, fmt.Sprintf("Student %d", s.PesertaID)))

			start := time.Now()
			code, err := client.SubmitWebhook(s.NoID, score, "100", xmlData, s.AttemptToken)
			latency := time.Since(start)

			r := RequestResult{Latency: latency, StatusCode: code, Endpoint: "webhook-burst", IsError: err != nil}
			if err != nil {
				r.ErrorMsg = classifyError(err.Error())
			}
			metrics.Record(r)
		}(i)
	}
	wg.Wait()
	burstDuration := time.Since(burstStart)
	fmt.Printf("  Burst completed in %v\n", burstDuration.Round(time.Millisecond))

	burstSnap := metrics.Snapshot()
	fmt.Printf("  Burst results: %d/%d success (%.1f%%), avg=%v, p95=%v\n",
		burstSnap.SuccessCount, burstSnap.TotalRequests,
		burstSnap.SuccessRate(), burstSnap.AvgLatency, burstSnap.P95)

	if duration <= 0 || burstDuration >= duration {
		snap := metrics.Snapshot()
		snap.PrintReport(scenarioName, concurrency)
		return
	}

	fmt.Printf("\n  Phase B: Sustained load for remaining %v...\n", duration-burstDuration)
	remaining := duration - burstDuration
	students, _ = dp.CreateSessions(students)
	if len(students) < concurrency {
		fmt.Printf("  Warning: only %d sessions recreated for phase B\n", len(students))
	}

	sustainedMetrics := NewMetricsCollector()
	sustainedProgress := NewProgressPrinter(sustainedMetrics, "webhook-sustained")
	sustainedProgress.Start(5 * time.Second)
	defer sustainedProgress.Stop()

	deadline := time.Now().Add(remaining)
	var wg2 sync.WaitGroup
	for i := 0; i < len(students); i++ {
		wg2.Add(1)
		go func(idx int) {
			defer wg2.Done()
			s := students[idx%len(students)]
			for time.Now().Before(deadline) {
				s.AttemptToken = randomHex(32)
				dp.DB.Exec(`INSERT OR REPLACE INTO cek_login
					(tenant_id, peserta_id, ruang_id, mapel_id, attempt_token, login_time, last_activity, tab_switch_count, answered_count, total_questions)
					VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'), 0, 0, 40)`,
					dp.TenantID, s.PesertaID, dp.RuangID, dp.MapelID, s.AttemptToken)

				score := fmt.Sprintf("%.2f", 50+rand.Float64()*50)
				xmlData := []byte(buildISpringXML(s.NoID, fmt.Sprintf("Student %d", s.PesertaID)))

				start := time.Now()
				code, err := client.SubmitWebhook(s.NoID, score, "100", xmlData, s.AttemptToken)
				latency := time.Since(start)

				r := RequestResult{Latency: latency, StatusCode: code, Endpoint: "webhook-sustained", IsError: err != nil}
				if err != nil {
					r.ErrorMsg = classifyError(err.Error())
				}
				sustainedMetrics.Record(r)

				jitter := time.Duration(rand.Intn(500)) * time.Millisecond
				time.Sleep(jitter)
			}
		}(i)
	}
	wg2.Wait()

	combined := combineMetrics(metrics, sustainedMetrics)
	combined.Snapshot().PrintReport(scenarioName, concurrency)
}

func combineMetrics(m1, m2 *MetricsCollector) *MetricsCollector {
	combined := NewMetricsCollector()
	combined.startTime = m1.startTime

	m1.mu.Lock()
	combined.results = append(combined.results, m1.results...)
	m1.mu.Unlock()

	m2.mu.Lock()
	combined.results = append(combined.results, m2.results...)
	m2.mu.Unlock()

	snap1 := m1.Snapshot()
	snap2 := m2.Snapshot()
	combined.totalReqs = snap1.TotalRequests + snap2.TotalRequests
	combined.successReqs = snap1.SuccessCount + snap2.SuccessCount
	combined.failReqs = snap1.FailCount + snap2.FailCount

	return combined
}

func classifyError(errMsg string) string {
	if len(errMsg) > 80 {
		return errMsg[:80]
	}
	return errMsg
}
