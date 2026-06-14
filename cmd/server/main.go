package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/saroel01/aether-cbt/internal/api/handlers"
	"github.com/saroel01/aether-cbt/internal/api/middleware"
	"github.com/saroel01/aether-cbt/internal/config"
	"github.com/saroel01/aether-cbt/internal/db"
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

	cfg := config.Load()
	utils.SetJWTSecret(cfg.JWTSecret) // configure JWT from env/config

	// Configure soal-package upload caps from config (Requirement 3.2, 10.6).
	handlers.SetSoalUploadLimits(cfg.SoalUploadMaxBytes, cfg.SoalPackageMaxFiles)

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
	if err := subQueue.RecoverStartup(ctx, true); err != nil {
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
		// Body limit sized to the largest legitimate payload: soal-package uploads.
		// Quizzes with images commonly run 15-20 MB. Driven from the upload cap so the two
		// stay in sync; the upload handler (task 6.2) may tighten this to a per-route
		// middleware so only /soal-packages/upload accepts the full size (Requirement 3.2).
		BodyLimit: int(cfg.SoalUploadMaxBytes),
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

	// Apply tenant middleware globally
	app.Use(middleware.TenantMiddleware())

	// Health check
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Aether CBT - Multi-Tenant Exam Platform",
			"status":  "running",
		})
	})

	// API routes
	api := app.Group("/api")
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Auth routes (public)
	auth := api.Group("/auth")
	auth.Post("/login", handlers.Login)
	auth.Post("/student-login", handlers.StudentLogin)
	auth.Post("/supervisor-login", handlers.SupervisorLogin)

	// Public QR Code generator
	api.Get("/qrcode", handlers.GetTokenQRCode)

	// iSpring Webhook (public) - registered BEFORE protected group to avoid auth middleware
	webhookMax := 100
	if v := os.Getenv("WEBHOOK_RATE_LIMIT_PER_MIN"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			webhookMax = parsed
		}
	}
	webhookLimiter := limiter.New(limiter.Config{
		Max:        webhookMax,
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).SendString("Too many submissions. Please try again later.")
		},
	})
	api.Post("/ispring/webhook", webhookLimiter, handlers.ISpringWebhook)

	// Protected routes (require login)
	protected := api.Group("", middleware.AuthMiddleware())

	// Room Supervisor routes
	supervisorOnly := middleware.RequireRoles("supervisor", "admin")
	protected.Get("/supervisor/room-status", supervisorOnly, handlers.GetRoomStatus)
	protected.Get("/supervisor/room-status/live", supervisorOnly, handlers.GetRoomStatusSSE)
	protected.Get("/supervisor/settings", supervisorOnly, handlers.GetSupervisorSettings)
	protected.Post("/supervisor/reset", supervisorOnly, handlers.ResetStudentSession)

	// Debug routes
	protected.Get("/debug/queue", supervisorOnly, handlers.GetQueueStatus(subQueue))

	// Student Exam Active session routes
	authenticatedExamUsers := middleware.RequireRoles("student", "admin", "supervisor")
	studentOnly := middleware.RequireRoles("student")
	protected.Get("/student/active-info", authenticatedExamUsers, handlers.GetActiveExamInfo)
	protected.Get("/student/mapels", authenticatedExamUsers, handlers.GetAvailableMapels)
	protected.Post("/student/start", studentOnly, handlers.StartExamSession)
	protected.Post("/student/infraction", studentOnly, handlers.RecordInfraction)
	protected.Post("/student/progress", studentOnly, handlers.UpdateStudentProgress)
	protected.Get("/student/remaining-time", authenticatedExamUsers, handlers.GetRemainingTime)

	// Admin Settings & Mapping routes
	adminOnly := middleware.RequireRoles("admin", "superadmin")
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

	// Serve static frontend from built assets in production
	app.Static("/", "./web/build")

	// SPA Routing support: serve index.html for unmatched client-side routes
	app.Get("/*", func(c *fiber.Ctx) error {
		path := c.Path()
		// Do not handle backend api routes
		if strings.HasPrefix(path, "/api") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "API route not found",
			})
		}
		return c.SendFile("./web/build/index.html")
	})

	log.Printf("Aether CBT starting on port %s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
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
