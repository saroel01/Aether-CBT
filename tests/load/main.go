package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

var (
	flagScenario    = flag.String("scenario", "all", "Test scenario: login, start, during-exam, submission, full-cycle, all")
	flagConcurrency = flag.Int("concurrency", 100, "Number of concurrent users")
	flagDuration    = flag.Duration("duration", 60*time.Second, "Test duration (e.g. 60s, 2m)")
	flagBaseURL     = flag.String("url", "http://localhost:3000", "Base URL of the Aether CBT server")
	flagDBPath      = flag.String("db", "data/cbt_aether.db", "Path to SQLite database")
	flagTenantID    = flag.Int("tenant", 1, "Tenant ID to use for testing")
	flagMapelID     = flag.Int("mapel", 0, "Mapel ID (0 = auto-detect first available)")
	flagNoCleanup   = flag.Bool("no-cleanup", false, "Skip cleanup of test data after run")
)

func main() {
	flag.Parse()

	printBanner()

	client := NewLoadClient(*flagBaseURL, *flagTenantID)

	fmt.Print("  Checking server health... ")
	if err := client.HealthCheck(); err != nil {
		fmt.Printf("FAILED\n  %v\n\n  Make sure the Aether CBT server is running at %s\n", err, *flagBaseURL)
		os.Exit(1)
	}
	fmt.Println("OK")

	fmt.Printf("  Connecting to database: %s\n", *flagDBPath)
	dp, err := NewDataPrep(*flagDBPath, *flagTenantID, *flagMapelID)
	if err != nil {
		fmt.Printf("  FATAL: Cannot connect to database: %v\n", err)
		os.Exit(1)
	}
	defer dp.Close()
	fmt.Printf("  DB connected. Kelas=%d, Ruang=%d, Mapel=%d\n", dp.KelasID, dp.RuangID, dp.MapelID)

	fmt.Println()

	if *e2eRun {
		runE2E(client, dp)
		return
	}

	scenarios := resolveScenarios(*flagScenario)
	for i, sc := range scenarios {
		if len(scenarios) > 1 {
			fmt.Printf("\n━━━ Scenario %d/%d ━━━\n", i+1, len(scenarios))
		}
		runScenario(sc, client, dp)
	}

	fmt.Println("\n\nAll requested scenarios completed.")
}

func printBanner() {
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║          Aether CBT - Full Load Test Suite               ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Scenario     : %-41s ║\n", *flagScenario)
	fmt.Printf("║  Concurrency  : %-10d                              ║\n", *flagConcurrency)
	fmt.Printf("║  Duration     : %-41v ║\n", *flagDuration)
	fmt.Printf("║  Target URL   : %-41s ║\n", *flagBaseURL)
	fmt.Printf("║  Database     : %-41s ║\n", *flagDBPath)
	fmt.Printf("║  Tenant       : %-10d                              ║\n", *flagTenantID)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
}

func resolveScenarios(name string) []string {
	switch name {
	case "all":
		return []string{"login", "start", "during-exam", "submission", "full-cycle"}
	case "priority":
		return []string{"login", "start", "submission"}
	default:
		return []string{name}
	}
}

func runScenario(name string, client *LoadClient, dp *DataPrep) {
	conc := *flagConcurrency
	dur := *flagDuration

	switch name {
	case "login":
		runLoginBurst(client, dp, conc, dur)
	case "start":
		runExamStartBurst(client, dp, conc, dur)
	case "during-exam":
		runDuringExamLoad(client, dp, conc, dur)
	case "submission":
		runSubmissionBurst(client, dp, conc, dur)
	case "full-cycle":
		runFullExamCycle(client, dp, conc, dur)
	default:
		fmt.Printf("Unknown scenario: %s\n", name)
		fmt.Println("Available: login, start, during-exam, submission, full-cycle, all, priority")
		os.Exit(1)
	}
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
