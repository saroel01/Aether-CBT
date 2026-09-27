package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/saroel01/aether-cbt/internal/api/handlers"
	"github.com/saroel01/aether-cbt/internal/api/middleware"
	"github.com/saroel01/aether-cbt/internal/config"
	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/service"
	"github.com/saroel01/aether-cbt/internal/submission"
	"github.com/saroel01/aether-cbt/internal/utils"
)

func main() {
	// Load .env file manually if exists (zero-dependency loader for local development)
	if envBytes, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(envBytes), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
			}
		}
	}

	cfg, err := config.LoadWithError()
	if err != nil {
		log.Fatalf("FATAL configuration error: %v", err)
	}
	// Explicit security check for production startup (P0-3)
	if cfg.Environment == "production" || cfg.Environment == "prod" {
		if err := config.ValidateJWTSecret(cfg.JWTSecret, cfg.Environment); err != nil {
			log.Fatalf("FATAL production configuration rejected: %v", err)
		}
	}
	utils.SetJWTSecret(cfg.JWTSecret) // configure JWT from env/config

	// Configure soal-package upload caps from config (Requirement 3.2, 10.6).
	handlers.SetSoalUploadLimits(cfg.SoalUploadMaxBytes, cfg.SoalPackageMaxFiles)
	// Content-session cookie Secure policy: auto / true / false (AD-2, Clause 2.14).
	handlers.SetContentCookieSecureMode(cfg.ContentCookieSecure)
	// Anti-cheat lock threshold from config (Requirement 10.6).
	handlers.SetAntiCheatLockThreshold(cfg.AntiCheatLockThreshold)

	// Connect to database with explicit connection pool tuning (Requirement 13.1)
	if err := db.Connect(cfg.DatabaseURL, db.PoolConfig{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
	}); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations (idempotent)
	if err := db.RunMigrations(db.DB, "internal/db/migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Bootstrap initial admin account if none exists and SETUP_ADMIN_PASSWORD is provided (P0-4)
	var adminCount int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin' AND deleted_at IS NULL`).Scan(&adminCount); err == nil && adminCount == 0 {
		setupPW := strings.TrimSpace(os.Getenv("SETUP_ADMIN_PASSWORD"))
		if setupPW != "" {
			if (cfg.Environment == "production" || cfg.Environment == "prod") && (setupPW == "admin123" || setupPW == "admin" || setupPW == "password" || len(setupPW) < 8) {
				log.Fatalf("FATAL: SETUP_ADMIN_PASSWORD terlalu lemah untuk environment production (panjang minimal 8 karakter dan tidak boleh menggunakan password default).")
			}
			hash, err := utils.HashPassword(setupPW)
			if err != nil {
				log.Fatalf("Failed to hash SETUP_ADMIN_PASSWORD: %v", err)
			}
			_, err = db.DB.Exec(`
				INSERT INTO users (tenant_id, username, password_hash, role, full_name, is_active)
				VALUES (1, 'admin', ?, 'admin', 'System Administrator', TRUE)
			`, hash)
			if err != nil {
				log.Fatalf("Failed to bootstrap initial admin user: %v", err)
			}
			log.Println("✅ Initial admin user bootstrapped successfully from SETUP_ADMIN_PASSWORD (username: admin)")
		} else {
			log.Println("⚠️  SECURITY WARNING: Belum ada akun admin di database. Set SETUP_ADMIN_PASSWORD atau jalankan cmd/createadmin untuk membuat akun admin pertama.")
		}
	}

	// Legacy data migration: for any tenant still on the old global settings.token model
	// (i.e. has a settings row but no exam_session), create one legacy exam + exam_session
	// so the install keeps working after the upgrade (Requirement 14.3, design AD-1).
	// Idempotent: tenants already on the session model are skipped (Requirement 14.4).
	legacyMigrator := service.NewLegacyMigrator(
		repository.NewExamRepository(db.DB),
		repository.NewExamSessionRepository(db.DB),
	)
	if err := legacyMigrator.Migrate(db.DB); err != nil {
		log.Fatalf("Failed to run legacy data migration: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Submission Queue + Worker
	queueDir := getEnvString("QUEUE_DIR", "data/queue")
	queueCfg := submission.FilesystemQueueConfig{
		MaxRetries:     getEnvInt("QUEUE_MAX_RETRIES", 5),
		StuckThreshold: time.Duration(getEnvInt("QUEUE_STUCK_THRESHOLD_MIN", 5)) * time.Minute,
		DoneRetention:  time.Duration(getEnvInt("QUEUE_DONE_RETENTION_DAYS", 7)) * 24 * time.Hour,
	}
	subQueue, err := submission.NewFilesystemQueueWithConfig(queueDir, queueCfg)
	if err != nil {
		log.Fatalf("Failed to initialize filesystem queue at %s: %v", queueDir, err)
	}
	defer subQueue.Close()
	// Recover in-flight jobs on startup. forceAll defaults to false (safer): only jobs
	// stuck longer than QUEUE_STUCK_THRESHOLD_MIN are promoted, so a quick restart does
	// not re-enqueue jobs a live worker may still be processing. Set QUEUE_RECOVER_FORCE_ALL
	// to "true" to promote every in-flight job regardless of age (review finding F4, Task 11).
	forceAll := strings.EqualFold(strings.TrimSpace(getEnvString("QUEUE_RECOVER_FORCE_ALL", "false")), "true")
	if err := subQueue.RecoverStartup(ctx, forceAll); err != nil {
		log.Fatalf("Failed to recover filesystem queue at startup: %v", err)
	}
	if err := subQueue.MigrateLegacyTable(ctx, db.DB); err != nil {
		log.Fatalf("Failed to migrate legacy submission_queue: %v", err)
	}

	processor := submission.NewProcessor(db.DB)
	worker := submission.NewWorkerWithConfig(
		subQueue,
		processor.ProcessBatch,
		getEnvInt("QUEUE_BATCH_SIZE", 5),
		time.Duration(getEnvInt("QUEUE_BATCH_TIMEOUT_MS", 100))*time.Millisecond,
	)
	handlers.SetSubmissionQueue(subQueue)

	go worker.Run(ctx)
	defer worker.Stop()

	app := fiber.New(fiber.Config{
		AppName: "Aether CBT v1.0",
		// Transport-level BodyLimit configured to cfg.SoalUploadMaxBytes (100 MB)
		// to allow legitimate package uploads. A global middleware enforces a strict 2 MB
		// limit on all other endpoints (P1-9).
		BodyLimit:   int(cfg.SoalUploadMaxBytes),
		ReadTimeout: 60 * time.Second,
		IdleTimeout: 120 * time.Second,
	})

	// Panic recovery middleware (P1-13) - prevents unhandled panics from crashing the server
	app.Use(recover.New())

	// Global BodyLimit guard (P1-9): general routes are restricted to 2 MB (1 MB for webhook).
	// Only /api/admin/soal-packages/upload and /api/soal-packages/upload are allowed up to cfg.SoalUploadMaxBytes.
	defaultBodyLimit := 2 * 1024 * 1024 // 2 MB
	app.Use(func(c *fiber.Ctx) error {
		// Non-body methods pass through immediately without buffering checks
		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}

		p := strings.ToLower(c.Path())

		// Upload routes are permitted up to SoalUploadMaxBytes (default 100 MB)
		if strings.HasPrefix(p, "/api/admin/soal-packages/upload") || strings.HasPrefix(p, "/api/soal-packages/upload") {
			maxUpload := int(cfg.SoalUploadMaxBytes)
			if c.Request().Header.ContentLength() > maxUpload || len(c.Body()) > maxUpload {
				return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
					"status":  "error",
					"message": fmt.Sprintf("Ukuran file paket soal melebihi batas maksimum (%d MB)", maxUpload/(1024*1024)),
				})
			}
			return c.Next()
		}

		// Webhook route has a dedicated 1 MB limit to prevent memory exhaustion DoS
		if strings.HasPrefix(p, "/api/ispring/webhook") {
			webhookLimit := 1 * 1024 * 1024 // 1 MB
			if c.Request().Header.ContentLength() > webhookLimit || len(c.Body()) > webhookLimit {
				return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
					"status":  "error",
					"message": "Webhook payload exceeds maximum allowed size (1 MB)",
				})
			}
			return c.Next()
		}

		cl := c.Request().Header.ContentLength()
		if cl > defaultBodyLimit {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"status":  "error",
				"message": "Payload too large. Maximum body size is 2 MB.",
			})
		}
		if len(c.Body()) > defaultBodyLimit {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"status":  "error",
				"message": "Payload too large. Maximum body size is 2 MB.",
			})
		}
		return c.Next()
	})

	// CORS - menggunakan allow-list (bukan wildcard)
	corsOrigins := cfg.CORSAllowedOrigins
	if corsOrigins == "" {
		if cfg.Environment == "development" || cfg.Environment == "dev" {
			corsOrigins = "http://localhost:5173,http://localhost:3000,http://127.0.0.1:5173"
		} else {
			log.Fatal("FATAL: CORS_ALLOWED_ORIGINS wajib diisi di production (contoh: https://app.sekolah.id,https://admin.sekolah.id)")
		}
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Tenant-ID, X-Tenant-Slug",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
	}))

	// Baseline browser-security headers (review security finding #14, Task 41).
	app.Use(middleware.SecurityHeaders())

	// API routes (TenantMiddleware scoped to API routes, exempting public endpoints)
	api := app.Group("/api")
	api.Use(middleware.TenantMiddleware())

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Auth routes (public)
	auth := api.Group("/auth")

	// P1-8: Rate limiter on login endpoints to prevent brute-force and CPU exhaustion DoS.
	loginMax := 10
	if v := os.Getenv("AUTH_RATE_LIMIT_PER_MIN"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			loginMax = parsed
		}
	}
	loginLimiter := limiter.New(limiter.Config{
		Max:        loginMax,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			tenant := ""
			if t := c.Locals("tenant_id"); t != nil {
				tenant = fmt.Sprint(t)
			}
			return c.IP() + "|" + tenant
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"status":  "error",
				"message": "Terlalu banyak percobaan login. Silakan coba beberapa saat lagi.",
			})
		},
	})

	auth.Post("/login", loginLimiter, handlers.Login)
	auth.Post("/student-login", loginLimiter, handlers.StudentLogin)
	auth.Post("/supervisor-login", loginLimiter, handlers.SupervisorLogin)

	// Public QR Code generator
	api.Get("/qrcode", handlers.GetTokenQRCode)

	// iSpring Webhook (public) - registered BEFORE protected group to avoid auth middleware.
	// P1-10: Rate limit keyed per attempt_token so multiple students behind a single NAT/LAN
	// IP do not exhaust a shared quota. Default raised to 1000/min per token.
	webhookMax := 1000
	if v := os.Getenv("WEBHOOK_RATE_LIMIT_PER_MIN"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			webhookMax = parsed
		}
	}
	webhookLimiter := limiter.New(limiter.Config{
		Max:        webhookMax,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			token := c.FormValue("attempt_token")
			if token == "" {
				token = c.FormValue("AETHER_ATTEMPT_TOKEN")
			}
			if token == "" {
				token = c.Query("attempt_token")
			}
			if token != "" {
				return "tok:" + token
			}
			tenant := fmt.Sprint(c.Locals("tenant_id"))
			return "ip:" + c.IP() + ":" + tenant
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).SendString("Too many submissions. Please try again later.")
		},
	})
	api.Post("/ispring/webhook", webhookLimiter, handlers.ISpringWebhook)

	// Exam content serving (Requirement 8, AD-2). Registered on the public group (outside the
	// Bearer AuthMiddleware): the iSpring player loads sub-assets via plain HTML tags with no
	// Authorization header. It is authorized by the content-session cookie instead, and
	// TenantMiddleware exempts this path (the tenant comes from the cookie token).
	api.Get("/exam/content/*", handlers.ServeExamContent)

	// Protected routes (require login)
	protected := api.Group("", middleware.AuthMiddleware())

	// Role middlewares (declared up front so any route below can reference them).
	supervisorOnly := middleware.RequireRoles("supervisor", "admin")
	adminOnly := middleware.RequireRoles("admin", "superadmin")

	// Room Supervisor routes
	protected.Get("/supervisor/room-status", supervisorOnly, handlers.GetRoomStatus)
	protected.Get("/supervisor/room-status/live", supervisorOnly, handlers.GetRoomStatusSSE)
	protected.Get("/supervisor/settings", supervisorOnly, handlers.GetSupervisorSettings)
	protected.Post("/supervisor/reset", supervisorOnly, handlers.ResetStudentSession)
	protected.Post("/supervisor/unlock", supervisorOnly, handlers.UnlockStudentSession)

	// Debug routes — admin-only so internal queue diagnostics never leak to room supervisors
	// (review F13, Task 28).
	protected.Get("/debug/queue", adminOnly, handlers.GetQueueStatus(subQueue))

	// Student Exam Active session routes
	authenticatedExamUsers := middleware.RequireRoles("student", "admin", "supervisor")
	studentOnly := middleware.RequireRoles("student")
	protected.Get("/student/active-info", authenticatedExamUsers, handlers.GetActiveExamInfo)
	protected.Get("/student/mapels", authenticatedExamUsers, handlers.GetAvailableMapels)
	protected.Post("/student/start", studentOnly, handlers.StartExamSession)
	protected.Get("/student/my-sessions", studentOnly, handlers.MySessions)
	protected.Post("/student/infraction", authenticatedExamUsers, handlers.RecordInfraction)
	protected.Post("/student/progress", studentOnly, handlers.UpdateStudentProgress)
	protected.Get("/student/remaining-time", authenticatedExamUsers, handlers.GetRemainingTime)

	// Admin Settings & Mapping routes
	protected.Get("/admin/settings", adminOnly, handlers.GetSettings)
	protected.Post("/admin/settings", adminOnly, handlers.UpdateSettings)
	protected.Post("/admin/curriculum/link", adminOnly, handlers.LinkClassSubject)
	protected.Post("/admin/curriculum/unlink", adminOnly, handlers.UnlinkClassSubject)
	protected.Get("/admin/curriculum/class/:kelas_id", adminOnly, handlers.GetClassSubjects)

	// CSV Utility routes
	protected.Post("/admin/students/import-csv", adminOnly, handlers.ImportStudentsCSV)
	protected.Get("/admin/results/export-csv", supervisorOnly, handlers.ExportResultsCSV)
	protected.Get("/admin/results/export-essay/:format", supervisorOnly, handlers.ExportEssayResults)
	protected.Get("/admin/results/analysis", supervisorOnly, handlers.GetEducationalAnalysis)
	protected.Get("/admin/results/essays", supervisorOnly, handlers.GetEssayAnswers)
	protected.Post("/admin/results/essays/grade", adminOnly, handlers.GradeEssayAnswer)
	protected.Get("/admin/results/item-analysis", supervisorOnly, handlers.GetItemAnalysis)

	// Record Delete routes
	protected.Delete("/students/:id", adminOnly, handlers.DeleteStudent)
	protected.Delete("/classes/:id", adminOnly, handlers.DeleteClass)
	protected.Delete("/mapel/:id", adminOnly, handlers.DeleteMapel)
	protected.Delete("/rooms/:id", adminOnly, handlers.DeleteRoom)

	// Tenant routes (superadmin/admin only)
	superadminOnly := middleware.RequireRoles("superadmin")
	protected.Get("/tenants", superadminOnly, handlers.GetAllTenants)
	protected.Post("/tenants", superadminOnly, handlers.CreateTenant)

	// User routes
	protected.Get("/users", adminOnly, handlers.GetUsers)
	protected.Post("/users", adminOnly, handlers.CreateUser)

	// Current user
	protected.Get("/me", handlers.Me)
	protected.Put("/me", adminOnly, handlers.UpdateMyProfile)

	// Student routes
	protected.Get("/students", adminOnly, handlers.GetStudents)
	protected.Post("/students", adminOnly, handlers.CreateStudent)

	// Class & Subject routes
	protected.Get("/classes", adminOnly, handlers.GetClasses)
	protected.Post("/classes", adminOnly, handlers.CreateClass)
	protected.Get("/mapel", adminOnly, handlers.GetMapel)
	protected.Post("/mapel", adminOnly, handlers.CreateMapel)

	// Room routes
	protected.Get("/rooms", adminOnly, handlers.GetRooms)
	protected.Post("/rooms", adminOnly, handlers.CreateRoom)

	// Exam scheduling & iSpring delivery admin routes (exam-scheduling spec, task 6).
	protected.Put("/classes/:id/tingkat", adminOnly, handlers.SetClassTingkat)
	protected.Get("/admin/soal-packages", adminOnly, handlers.ListSoalPackages)
	protected.Post("/admin/soal-packages/upload", adminOnly, handlers.UploadSoalPackage)
	protected.Delete("/admin/soal-packages/:id", adminOnly, handlers.DeleteSoalPackage)
	protected.Get("/admin/exams", adminOnly, handlers.ListExams)
	protected.Post("/admin/exams", adminOnly, handlers.CreateExam)
	protected.Put("/admin/exams/:id", adminOnly, handlers.UpdateExam)
	protected.Delete("/admin/exams/:id", adminOnly, handlers.DeleteExam)
	protected.Get("/admin/exam-sessions", adminOnly, handlers.ListExamSessions)
	protected.Post("/admin/exam-sessions", adminOnly, handlers.CreateExamSession)
	protected.Put("/admin/exam-sessions/:id", adminOnly, handlers.UpdateExamSession)
	protected.Delete("/admin/exam-sessions/:id", adminOnly, handlers.DeleteExamSession)
	protected.Post("/admin/exam-sessions/:id/classes", adminOnly, handlers.LinkSessionClasses)
	protected.Post("/admin/exam-sessions/:id/rooms", adminOnly, handlers.LinkSessionRooms)

	// Determine web build directory (cwd or relative to binary)
	webBuildDir := "./web/build"
	if _, err := os.Stat(filepath.Join(webBuildDir, "index.html")); os.IsNotExist(err) {
		if exePath, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exePath), "web", "build")
			if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
				webBuildDir = candidate
			}
		}
	}
	if abs, err := filepath.Abs(webBuildDir); err == nil {
		webBuildDir = abs
	}

	// Serve static frontend from built assets in production
	app.Static("/", webBuildDir)

	// SPA Routing support: serve index.html for unmatched client-side routes
	app.Get("/*", func(c *fiber.Ctx) error {
		path := strings.ToLower(c.Path())
		// Do not handle backend api routes (/api or /api/...)
		if path == "/api" || strings.HasPrefix(path, "/api/") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "API route not found",
			})
		}
		indexPath := filepath.Join(webBuildDir, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).SendString("Frontend build not found")
		}
		return c.SendFile(indexPath)
	})

	// Graceful shutdown listener (Clause 2.23)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Aether CBT starting on port %s", cfg.Port)
		if err := app.Listen(":" + cfg.Port); err != nil {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Fatalf("Server failed to start or listen error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal %s, initiating graceful shutdown...", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Printf("Fiber shutdown error: %v", err)
		}
		log.Println("Waiting for submission worker to finish in-flight jobs...")
		worker.Stop()
	}

	log.Println("Aether CBT shutdown complete")
}

func getEnvString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
