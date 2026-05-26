package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func runFullExamCycle(client *LoadClient, dp *DataPrep, concurrency int, duration time.Duration) {
	scenarioName := fmt.Sprintf("Full Exam Cycle (N=%d)", concurrency)
	fmt.Printf("\n>>> Preparing: %s\n", scenarioName)

	examToken := dp.GetExamToken()
	prefix := fmt.Sprintf("FC%d_", concurrency)

	fmt.Printf("  Creating %d test students...\n", concurrency)
	students, err := dp.CreateStudents(concurrency, prefix)
	if err != nil {
		fmt.Printf("  FATAL: Failed to create students: %v\n", err)
		return
	}
	fmt.Printf("  Created %d students.\n", len(students))

	defer func() {
		fmt.Println("  Cleaning up...")
		dp.ClearSessions(prefix)
		dp.Cleanup(prefix)
	}()

	metrics := NewMetricsCollector()
	progress := NewProgressPrinter(metrics, "full-cycle")
	progress.Start(5 * time.Second)
	defer progress.Stop()

	phaseDuration := duration / 4

	fmt.Printf("\n  === Phase 1: Login (first %v) ===\n", phaseDuration)
	runCycleLogin(client, metrics, students, examToken, dp, phaseDuration)

	fmt.Printf("\n  === Phase 2: Start Exam (next %v) ===\n", phaseDuration)
	jwtMap := runCycleGetJWTs(client, students, examToken, dp)
	runCycleStart(client, metrics, students, jwtMap, dp, phaseDuration)

	fmt.Printf("\n  === Phase 3: During Exam (next %v) ===\n", phaseDuration)
	sessions := dp.refreshSessions(students, prefix)
	runCycleDuring(client, metrics, students, jwtMap, sessions, dp, phaseDuration)

	fmt.Printf("\n  === Phase 4: Submission (final %v) ===\n", phaseDuration)
	runCycleSubmission(client, metrics, students, sessions, dp, phaseDuration)

	snap := metrics.Snapshot()
	snap.PrintReport(scenarioName, concurrency)
	printEndpointBreakdown(metrics)
}

func runCycleLogin(client *LoadClient, metrics *MetricsCollector, students []TestStudent, examToken string, dp *DataPrep, d time.Duration) {
	var wg sync.WaitGroup
	deadline := time.Now().Add(d)
	for i := 0; i < len(students); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := students[idx]
			for time.Now().Before(deadline) {
				start := time.Now()
				_, code, err := client.StudentLogin(s.NoID, s.Password, examToken)
				lat := time.Since(start)
				r := RequestResult{Latency: lat, StatusCode: code, Endpoint: "1-login", IsError: err != nil}
				if err != nil {
					r.ErrorMsg = classifyError(err.Error())
				}
				metrics.Record(r)
				time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
			}
		}(i)
	}
	wg.Wait()
}

func runCycleGetJWTs(client *LoadClient, students []TestStudent, examToken string, dp *DataPrep) map[int]string {
	jwtMap := make(map[int]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20)
	for _, s := range students {
		wg.Add(1)
		sem <- struct{}{}
		go func(student TestStudent) {
			defer wg.Done()
			defer func() { <-sem }()
			for attempt := 0; attempt < 5; attempt++ {
				resp, _, err := client.StudentLogin(student.NoID, student.Password, examToken)
				if err == nil {
					mu.Lock()
					jwtMap[student.PesertaID] = resp.Token
					mu.Unlock()
					return
				}
				time.Sleep(200 * time.Millisecond)
			}
		}(s)
	}
	wg.Wait()
	fmt.Printf("    Got JWTs for %d/%d students\n", len(jwtMap), len(students))
	return jwtMap
}

func runCycleStart(client *LoadClient, metrics *MetricsCollector, students []TestStudent, jwtMap map[int]string, dp *DataPrep, d time.Duration) {
	var wg sync.WaitGroup
	deadline := time.Now().Add(d)
	for _, s := range students {
		jwt, ok := jwtMap[s.PesertaID]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(student TestStudent, token string) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				start := time.Now()
				_, code, err := client.StartExam(token, student.PesertaID, dp.MapelID)
				lat := time.Since(start)
				r := RequestResult{Latency: lat, StatusCode: code, Endpoint: "2-start", IsError: err != nil}
				if err != nil {
					r.ErrorMsg = classifyError(err.Error())
				}
				metrics.Record(r)
				time.Sleep(time.Duration(rand.Intn(400)) * time.Millisecond)
			}
		}(s, jwt)
	}
	wg.Wait()
}

func (dp *DataPrep) refreshSessions(students []TestStudent, prefix string) map[int]string {
	dp.ClearSessions(prefix)
	sessions := make(map[int]string)
	for _, s := range students {
		token := randomHex(32)
		dp.DB.Exec(`
			INSERT OR REPLACE INTO cek_login
			(tenant_id, peserta_id, ruang_id, mapel_id, attempt_token, login_time, last_activity, tab_switch_count, answered_count, total_questions)
			VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'), 0, 0, 40)
		`, dp.TenantID, s.PesertaID, dp.RuangID, dp.MapelID, token)
		sessions[s.PesertaID] = token
	}
	return sessions
}

func runCycleDuring(client *LoadClient, metrics *MetricsCollector, students []TestStudent, jwtMap map[int]string, sessions map[int]string, dp *DataPrep, d time.Duration) {
	var wg sync.WaitGroup
	deadline := time.Now().Add(d)
	for _, s := range students {
		jwt, ok := jwtMap[s.PesertaID]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(student TestStudent, token string) {
			defer wg.Done()
			answered := rand.Intn(10)
			totalQ := 40

			for time.Now().Before(deadline) {
				roll := rand.Float64()
				var start time.Time
				var code int
				var err error
				var ep string

				if roll < 0.55 {
					ep = "3-remaining"
					start = time.Now()
					code, err = client.GetRemainingTime(token, student.PesertaID, dp.MapelID)
				} else if roll < 0.85 {
					ep = "3-progress"
					if answered < totalQ {
						answered += rand.Intn(3) + 1
						if answered > totalQ {
							answered = totalQ
						}
					}
					start = time.Now()
					code, err = client.UpdateProgress(token, student.PesertaID, dp.MapelID, answered, totalQ)
				} else {
					ep = "3-infraction"
					start = time.Now()
					code, err = client.RecordInfraction(token, student.PesertaID, dp.MapelID)
				}

				lat := time.Since(start)
				r := RequestResult{Latency: lat, StatusCode: code, Endpoint: ep, IsError: err != nil}
				if err != nil {
					r.ErrorMsg = classifyError(err.Error())
				}
				metrics.Record(r)
				time.Sleep(time.Duration(500+rand.Intn(2000)) * time.Millisecond)
			}
		}(s, jwt)
	}
	wg.Wait()
}

func runCycleSubmission(client *LoadClient, metrics *MetricsCollector, students []TestStudent, sessions map[int]string, dp *DataPrep, d time.Duration) {
	var wg sync.WaitGroup
	deadline := time.Now().Add(d)
	for _, s := range students {
		attemptToken, ok := sessions[s.PesertaID]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(student TestStudent, attToken string) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				score := fmt.Sprintf("%.2f", 50+rand.Float64()*50)
				xmlData := []byte(buildISpringXML(student.NoID, fmt.Sprintf("Student %d", student.PesertaID)))

				start := time.Now()
				code, err := client.SubmitWebhook(student.NoID, score, "100", xmlData, attToken)
				lat := time.Since(start)
				r := RequestResult{Latency: lat, StatusCode: code, Endpoint: "4-submit", IsError: err != nil}
				if err != nil {
					r.ErrorMsg = classifyError(err.Error())
				}
				metrics.Record(r)

				if err != nil {
					attToken = randomHex(32)
					dp.DB.Exec(`
						INSERT OR REPLACE INTO cek_login
						(tenant_id, peserta_id, ruang_id, mapel_id, attempt_token, login_time, last_activity)
						VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
					`, dp.TenantID, student.PesertaID, dp.RuangID, dp.MapelID, attToken)
				}

				time.Sleep(time.Duration(rand.Intn(800)) * time.Millisecond)
			}
		}(s, attemptToken)
	}
	wg.Wait()
}
