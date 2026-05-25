package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"

	"github.com/saroel01/aether-cbt/internal/api/middleware"
	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

func setupStudentAuthFlowDB(t *testing.T) {
	t.Helper()
	var err error
	db.DB, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}

	schemas := []string{
		`CREATE TABLE tenants (id INTEGER PRIMARY KEY, slug TEXT, name TEXT);`,
		`CREATE TABLE settings (tenant_id INTEGER NOT NULL, token TEXT NOT NULL, is_exam_active BOOLEAN DEFAULT TRUE);`,
		`CREATE TABLE peserta (
			id INTEGER PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			no_id TEXT NOT NULL,
			password TEXT NOT NULL,
			nama_peserta TEXT NOT NULL,
			kelas_id INTEGER NOT NULL,
			ruang_id INTEGER NOT NULL,
			jenis_kelamin TEXT,
			deleted_at DATETIME,
			UNIQUE(tenant_id, no_id)
		);`,
		`CREATE TABLE mapel (
			id INTEGER PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			nama_mapel TEXT NOT NULL,
			durasi_menit INTEGER DEFAULT 90,
			deleted_at DATETIME
		);`,
		`CREATE TABLE cek_login (
			id INTEGER PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			peserta_id INTEGER NOT NULL,
			mapel_id INTEGER NOT NULL,
			attempt_token TEXT,
			login_time DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_activity DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(tenant_id, peserta_id, mapel_id)
		);`,
	}
	for _, schema := range schemas {
		if _, err := db.DB.Exec(schema); err != nil {
			t.Fatalf("apply schema: %v\n%s", err, schema)
		}
	}

	_, _ = db.DB.Exec(`INSERT INTO tenants (id, slug, name) VALUES (1, 'default', 'Default')`)
	_, _ = db.DB.Exec(`INSERT INTO settings (tenant_id, token, is_exam_active) VALUES (1, 'ujian2026', TRUE)`)
	_, _ = db.DB.Exec(`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (42, 1, '2026001', 'siswa123', 'Siswa Tes', 1, 1)`)
	_, _ = db.DB.Exec(`INSERT INTO mapel (id, tenant_id, nama_mapel) VALUES (5, 1, 'Matematika')`)
}

func TestStudentLoginReturnsJWTUsableForProtectedExamStart(t *testing.T) {
	setupStudentAuthFlowDB(t)
	defer TeardownTestDB()
	utils.SetJWTSecret("student-auth-flow-test-secret")

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/auth/student-login", StudentLogin)
	app.Post("/student/start", middleware.AuthMiddleware(), StartExamSession)

	loginReq := httptest.NewRequest("POST", "/auth/student-login", bytes.NewBufferString(`{"no_id":"2026001","password":"siswa123","token":"ujian2026"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := app.Test(loginReq)
	if err != nil {
		t.Fatalf("student login request failed: %v", err)
	}
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected login status 200, got %d", loginResp.StatusCode)
	}

	var loginBody struct {
		Data struct {
			Token     string `json:"token"`
			PesertaID int    `json:"peserta_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginBody.Data.Token == "" || loginBody.Data.PesertaID != 42 {
		t.Fatalf("login response should contain token and peserta_id, got %+v", loginBody.Data)
	}

	startReq := httptest.NewRequest("POST", "/student/start", bytes.NewBufferString(`{"peserta_id":42,"mapel_id":5}`))
	startReq.Header.Set("Content-Type", "application/json")
	startReq.Header.Set("Authorization", "Bearer "+loginBody.Data.Token)
	startResp, err := app.Test(startReq)
	if err != nil {
		t.Fatalf("start exam request failed: %v", err)
	}
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("expected start status 200, got %d", startResp.StatusCode)
	}

	var startBody struct {
		Data struct {
			AttemptToken string `json:"attempt_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&startBody); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if startBody.Data.AttemptToken == "" {
		t.Fatalf("start response should include attempt_token")
	}
}

func TestStudentLoginAcceptsBcryptPassword(t *testing.T) {
	setupStudentAuthFlowDB(t)
	defer TeardownTestDB()
	utils.SetJWTSecret("student-auth-flow-test-secret")

	hash, err := utils.HashPassword("siswa123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := db.DB.Exec(`UPDATE peserta SET password = ? WHERE id = 42`, hash); err != nil {
		t.Fatalf("store hashed student password: %v", err)
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/auth/student-login", StudentLogin)

	loginReq := httptest.NewRequest("POST", "/auth/student-login", bytes.NewBufferString(`{"no_id":"2026001","password":"siswa123","token":"ujian2026"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(loginReq, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected hashed student login status 200, got %d", resp.StatusCode)
	}
}

func TestCreateStudentStoresBcryptPassword(t *testing.T) {
	setupStudentAuthFlowDB(t)
	defer TeardownTestDB()

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/students", CreateStudent)

	req := httptest.NewRequest("POST", "/students", bytes.NewBufferString(`{
		"no_id":"2026002",
		"password":"secret123",
		"nama_peserta":"Siswa Baru",
		"kelas_id":1,
		"ruang_id":1,
		"jenis_kelamin":"L"
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("create student request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected create status 200, got %d", resp.StatusCode)
	}

	var storedPassword string
	if err := db.DB.QueryRow(`SELECT password FROM peserta WHERE no_id = '2026002'`).Scan(&storedPassword); err != nil {
		t.Fatalf("read stored password: %v", err)
	}
	if storedPassword == "secret123" {
		t.Fatalf("student password was stored as plaintext")
	}
	if !utils.CheckPasswordHash("secret123", storedPassword) {
		t.Fatalf("stored password hash does not verify")
	}
}

func TestImportStudentsCSVStoresBcryptPassword(t *testing.T) {
	setupStudentAuthFlowDB(t)
	defer TeardownTestDB()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "students.csv")
	if err != nil {
		t.Fatalf("create csv part: %v", err)
	}
	if _, err := part.Write([]byte("no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin,password\n2026003,Siswa CSV,1,1,P,csvsecret\n")); err != nil {
		t.Fatalf("write csv part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		c.Locals("role", "admin")
		return c.Next()
	})
	app.Post("/admin/students/import-csv", ImportStudentsCSV)

	req := httptest.NewRequest("POST", "/admin/students/import-csv", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("import request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected import status 200, got %d", resp.StatusCode)
	}

	var storedPassword string
	if err := db.DB.QueryRow(`SELECT password FROM peserta WHERE no_id = '2026003'`).Scan(&storedPassword); err != nil {
		t.Fatalf("read imported password: %v", err)
	}
	if storedPassword == "csvsecret" {
		t.Fatalf("imported student password was stored as plaintext")
	}
	if !utils.CheckPasswordHash("csvsecret", storedPassword) {
		t.Fatalf("imported password hash does not verify")
	}
}
