package main

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type RequestResult struct {
	Latency    time.Duration
	StatusCode int
	Endpoint   string
	IsError    bool
	ErrorMsg   string
}

type MetricsCollector struct {
	mu          sync.Mutex
	results     []RequestResult
	startTime   time.Time
	totalReqs   int64
	successReqs int64
	failReqs    int64
	statusCodes map[int]int64
	errors      map[string]int64
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		results:     make([]RequestResult, 0),
		statusCodes: make(map[int]int64),
		errors:      make(map[string]int64),
		startTime:   time.Now(),
	}
}

func (m *MetricsCollector) Record(r RequestResult) {
	m.mu.Lock()
	m.results = append(m.results, r)
	m.mu.Unlock()

	atomic.AddInt64(&m.totalReqs, 1)
	if r.IsError {
		atomic.AddInt64(&m.failReqs, 1)
		m.mu.Lock()
		m.errors[r.ErrorMsg]++
		m.mu.Unlock()
	} else {
		atomic.AddInt64(&m.successReqs, 1)
	}
	m.mu.Lock()
	m.statusCodes[r.StatusCode]++
	m.mu.Unlock()
}

func (m *MetricsCollector) TotalRequests() int64 {
	return atomic.LoadInt64(&m.totalReqs)
}

func (m *MetricsCollector) Snapshot() MetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := MetricsSnapshot{
		TotalRequests: atomic.LoadInt64(&m.totalReqs),
		SuccessCount:  atomic.LoadInt64(&m.successReqs),
		FailCount:     atomic.LoadInt64(&m.failReqs),
		Duration:      time.Since(m.startTime),
		StatusCodes:   make(map[int]int64),
		Errors:        make(map[string]int64),
	}

	for k, v := range m.statusCodes {
		s.StatusCodes[k] = v
	}
	for k, v := range m.errors {
		s.Errors[k] = v
	}

	if len(m.results) == 0 {
		return s
	}

	latencies := make([]time.Duration, len(m.results))
	for i, r := range m.results {
		latencies[i] = r.Latency
	}
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	s.MinLatency = latencies[0]
	s.MaxLatency = latencies[len(latencies)-1]

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	s.AvgLatency = total / time.Duration(len(latencies))

	s.P50 = latencies[percentileIndex(len(latencies), 50)]
	s.P90 = latencies[percentileIndex(len(latencies), 90)]
	s.P95 = latencies[percentileIndex(len(latencies), 95)]
	s.P99 = latencies[percentileIndex(len(latencies), 99)]

	s.Throughput = float64(s.TotalRequests) / s.Duration.Seconds()

	return s
}

func percentileIndex(count, percentile int) int {
	idx := int(float64(count-1) * float64(percentile) / 100.0)
	if idx >= count {
		idx = count - 1
	}
	return idx
}

type MetricsSnapshot struct {
	TotalRequests int64
	SuccessCount  int64
	FailCount     int64
	Duration      time.Duration
	MinLatency    time.Duration
	AvgLatency    time.Duration
	P50           time.Duration
	P90           time.Duration
	P95           time.Duration
	P99           time.Duration
	MaxLatency    time.Duration
	Throughput    float64
	StatusCodes   map[int]int64
	Errors        map[string]int64
}

func (s MetricsSnapshot) SuccessRate() float64 {
	if s.TotalRequests == 0 {
		return 0
	}
	return float64(s.SuccessCount) / float64(s.TotalRequests) * 100
}

func (s MetricsSnapshot) PrintReport(scenarioName string, concurrency int) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Printf("║  LOAD TEST RESULTS: %-40s ║\n", scenarioName)
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Concurrency    : %-10d                              ║\n", concurrency)
	fmt.Printf("║  Total Requests : %-10d                              ║\n", s.TotalRequests)
	fmt.Printf("║  Successful     : %-10d                              ║\n", s.SuccessCount)
	fmt.Printf("║  Failed         : %-10d                              ║\n", s.FailCount)
	fmt.Printf("║  Success Rate   : %-10.2f %%                            ║\n", s.SuccessRate())
	fmt.Printf("║  Duration       : %-10v                              ║\n", s.Duration.Round(time.Millisecond))
	fmt.Printf("║  Throughput     : %-10.2f req/s                       ║\n", s.Throughput)
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  LATENCY DISTRIBUTION                                       ║")
	fmt.Printf("║  Min  : %-10v                                          ║\n", s.MinLatency)
	fmt.Printf("║  Avg  : %-10v                                          ║\n", s.AvgLatency)
	fmt.Printf("║  P50  : %-10v                                          ║\n", s.P50)
	fmt.Printf("║  P90  : %-10v                                          ║\n", s.P90)
	fmt.Printf("║  P95  : %-10v                                          ║\n", s.P95)
	fmt.Printf("║  P99  : %-10v                                          ║\n", s.P99)
	fmt.Printf("║  Max  : %-10v                                          ║\n", s.MaxLatency)
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  STATUS CODES                                               ║")
	for code, count := range s.StatusCodes {
		fmt.Printf("║    HTTP %d : %d %-43s ║\n", code, count, "")
	}
	if len(s.Errors) > 0 {
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║  ERROR BREAKDOWN                                            ║")
		for errMsg, count := range s.Errors {
			truncated := errMsg
			if len(truncated) > 45 {
				truncated = truncated[:45] + "..."
			}
			fmt.Printf("║    %s (x%d) %-18s ║\n", truncated, count, "")
		}
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
}

type ProgressPrinter struct {
	collector *MetricsCollector
	stop      chan struct{}
	scenario  string
}

func NewProgressPrinter(collector *MetricsCollector, scenario string) *ProgressPrinter {
	return &ProgressPrinter{
		collector: collector,
		stop:      make(chan struct{}),
		scenario:  scenario,
	}
}

func (p *ProgressPrinter) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				snap := p.collector.Snapshot()
				fmt.Printf("  [%s] reqs=%d  ok=%d  fail=%d  p95=%v  throughput=%.1f/s\n",
					p.scenario,
					snap.TotalRequests,
					snap.SuccessCount,
					snap.FailCount,
					snap.P95,
					snap.Throughput,
				)
			case <-p.stop:
				return
			}
		}
	}()
}

func (p *ProgressPrinter) Stop() {
	close(p.stop)
}
