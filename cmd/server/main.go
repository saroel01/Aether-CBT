package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"github.com/anomalyco/aether-cbt/internal/api/handlers"
	"github.com/anomalyco/aether-cbt/internal/api/middleware"
	"github.com/anomalyco/aether-cbt/internal/config"
	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/utils"
)

func main() {
	cfg := config.Load()
	utils.SetJWTSecret(cfg.JWTSecret) // configure JWT from env/config

	// Connect to database
	if err := db.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations (idempotent)
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	app := fiber.New(fiber.Config{
		AppName: "Aether CBT v1.0",
	})

	// CORS - allow frontend (Vite on 5173) during development
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
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

	// Protected routes (require login)
	protected := api.Group("", middleware.AuthMiddleware())

	// Room Supervisor routes
	protected.Get("/supervisor/room-status", handlers.GetRoomStatus)
	protected.Post("/supervisor/reset", handlers.ResetStudentSession)

	// Student Exam Active session routes
	protected.Get("/student/active-info", handlers.GetActiveExamInfo)
	protected.Get("/student/mapels", handlers.GetAvailableMapels)
	protected.Post("/student/start", handlers.StartExamSession)
	protected.Post("/student/infraction", handlers.RecordInfraction)
	protected.Post("/student/progress", handlers.UpdateStudentProgress)


	// Admin Settings & Mapping routes
	protected.Get("/admin/settings", handlers.GetSettings)
	protected.Post("/admin/settings", handlers.UpdateSettings)
	protected.Post("/admin/curriculum/link", handlers.LinkClassSubject)
	protected.Post("/admin/curriculum/unlink", handlers.UnlinkClassSubject)
	protected.Get("/admin/curriculum/class/:kelas_id", handlers.GetClassSubjects)

	// CSV Utility routes
	protected.Post("/admin/students/import-csv", handlers.ImportStudentsCSV)
	protected.Get("/admin/results/export-csv", handlers.ExportResultsCSV)
	protected.Get("/admin/results/analysis", handlers.GetEducationalAnalysis)

	// Record Delete routes
	protected.Delete("/students/:id", handlers.DeleteStudent)
	protected.Delete("/classes/:id", handlers.DeleteClass)
	protected.Delete("/mapel/:id", handlers.DeleteMapel)
	protected.Delete("/rooms/:id", handlers.DeleteRoom)

	// Tenant routes (superadmin/admin only)
	protected.Get("/tenants", handlers.GetAllTenants)
	protected.Post("/tenants", handlers.CreateTenant)

	// User routes
	protected.Get("/users", handlers.GetUsers)
	protected.Post("/users", handlers.CreateUser)

	// Current user
	protected.Get("/me", handlers.Me)

	// Student routes
	protected.Get("/students", handlers.GetStudents)
	protected.Post("/students", handlers.CreateStudent)

	// Class & Subject routes
	protected.Get("/classes", handlers.GetClasses)
	protected.Post("/classes", handlers.CreateClass)
	protected.Get("/mapel", handlers.GetMapel)
	protected.Post("/mapel", handlers.CreateMapel)

	// Room routes
	protected.Get("/rooms", handlers.GetRooms)
	protected.Post("/rooms", handlers.CreateRoom)

	// iSpring Webhook (public)
	api.Post("/ispring/webhook", handlers.ISpringWebhook)

	log.Printf("Aether CBT starting on port %s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
}
