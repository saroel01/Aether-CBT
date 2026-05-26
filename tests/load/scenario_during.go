package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func runDuringExamLoad(client *LoadClient, dp *DataPrep, concurrency int, duration time.Duration) {
	scenarioName := fmt.Sprintf("During-Exam Mixed Load (N=%d)", concurrency)
	fmt.Printf("\n>>> Preparing: %s\n", scenarioName)

	examToken := dp.GetExamToken()
	prefix := fmt.Sprintf("DX%d_", concurrency)

	fmt.Printf("  Creating %d test students + sessions...\n", concurrency)
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

	type readyStudent struct {
		student TestStudent
		jwt     string
	}
	var ready []readyStudent
	var readyMu sync.Mutex

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
					readyMu.Lock()
					ready = append(ready, readyStudent{student: s, jwt: resp.Token})
					readyMu.Unlock()
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}
	loginWg.Wait()

	if len(ready) < concurrency/2 {
		fmt.Printf("  FATAL: Only %d/%d students ready. Aborting.\n", len(ready), concurrency)
		dp.Cleanup(prefix)
		return
	}
	fmt.Printf("  %d students ready for mixed load test.\n", len(ready))

	defer func() {
		fmt.Println("  Cleaning up...")
		dp.ClearSessions(prefix)
		dp.Cleanup(prefix)
	}()

	metrics := NewMetricsCollector()
	progress := NewProgressPrinter(metrics, "mixed")
	progress.Start(5 * time.Second)
	defer progress.Stop()

	fmt.Printf("  Starting mixed load for %v (polling + progress + infraction)...\n\n", duration)

	var wg sync.WaitGroup
	deadline := time.Now().Add(duration)

	for i := 0; i < len(ready); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := ready[idx%len(ready)]
			totalQ := 40
			answered := 0

			for time.Now().Before(deadline) {
				roll := rand.Float64()

				var start time.Time
				var statusCode int
				var err error
				var endpoint string

				if roll < 0.60 {
					endpoint = "remaining-time"
					start = time.Now()
					statusCode, err = client.GetRemainingTime(r.jwt, r.student.PesertaID, dp.MapelID)
				} else if roll < 0.90 {
					endpoint = "progress"
					if answered < totalQ {
						answered += rand.Intn(3) + 1
						if answered > totalQ {
							answered = totalQ
						}
					}
					start = time.Now()
					statusCode, err = client.UpdateProgress(r.jwt, r.student.PesertaID, dp.MapelID, answered, totalQ)
				} else {
					endpoint = "infraction"
					start = time.Now()
					statusCode, err = client.RecordInfraction(r.jwt, r.student.PesertaID, dp.MapelID)
				}

				latency := time.Since(start)
				result := RequestResult{
					Latency:    latency,
					StatusCode: statusCode,
					Endpoint:   endpoint,
					IsError:    err != nil,
				}
				if err != nil {
					result.ErrorMsg = classifyError(err.Error())
				}
				metrics.Record(result)

				sleep := time.Duration(500+rand.Intn(2000)) * time.Millisecond
				time.Sleep(sleep)
			}
		}(i)
	}

	wg.Wait()

	snap := metrics.Snapshot()
	snap.PrintReport(scenarioName, concurrency)

	printEndpointBreakdown(metrics)
}

func printEndpointBreakdown(metrics *MetricsCollector) {
	snap := metrics.Snapshot()
	if snap.TotalRequests == 0 {
		return
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	endpointCount := make(map[string]int64)
	endpointErrors := make(map[string]int64)
	for _, r := range metrics.results {
		endpointCount[r.Endpoint]++
		if r.IsError {
			endpointErrors[r.Endpoint]++
		}
	}

	fmt.Println("\n  Endpoint Breakdown:")
	fmt.Printf("    %-20s %8s %8s\n", "Endpoint", "Requests", "Errors")
	fmt.Printf("    %-20s %8s %8s\n", "--------", "--------", "------")
	for ep, count := range endpointCount {
		errs := endpointErrors[ep]
		fmt.Printf("    %-20s %8d %8d\n", ep, count, errs)
	}
}
