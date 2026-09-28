package handlers

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

func postCSVData(t *testing.T, app *fiber.App, path string, csvContent string) (int, []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "test.csv")
	if err != nil {
		t.Fatalf("create multipart form file: %v", err)
	}
	if _, err := part.Write([]byte(csvContent)); err != nil {
		t.Fatalf("write csv part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp.StatusCode, out.Bytes()
}

func TestSanitizeFormulaField(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", "   "},
		{"Normal Name", "Normal Name"},
		{"=CMD|'calc.exe'!A0", "'=CMD|'calc.exe'!A0"},
		{"+12345", "'+12345"},
		{"-54321", "'-54321"},
		{"@SUM(A1:A10)", "'@SUM(A1:A10)"},
		{"\tTabIndent", "'\tTabIndent"},
		{"\rCarriageReturn", "'\rCarriageReturn"},
		{"  =LeadingSpaces", "'=LeadingSpaces"},
		{"Text with = inside is ok", "Text with = inside is ok"},
	}

	for _, tc := range cases {
		got := SanitizeFormulaField(tc.in)
		if got != tc.want {
			t.Errorf("SanitizeFormulaField(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestImportStudentsCSV_PasswordPreservation(t *testing.T) {
	app, database := newStudentApp(t)

	// 1. Pre-insert a student with a custom password hash
	customPass := "CustomSecretPassword123!"
	customHash, err := utils.HashPassword(customPass)
	if err != nil {
		t.Fatalf("hash custom password: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id, jenis_kelamin)
		VALUES (1, 'STU001', ?, 'Original Student', 1, 1, 'L')
	`, customHash)
	if err != nil {
		t.Fatalf("insert existing student: %v", err)
	}

	// 2. Re-import CSV without password column (or blank password)
	// P2-22: must preserve existing custom password, not overwrite with default siswa123
	csvNoPass := "no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\nSTU001,Updated Student Name,1,1,L\n"
	status, body := postCSVData(t, app, "/admin/students/import-csv", csvNoPass)
	if status != fiber.StatusOK {
		t.Fatalf("import without password returned %d: %s", status, string(body))
	}

	var storedHash string
	err = database.QueryRow(`SELECT password FROM peserta WHERE tenant_id = 1 AND no_id = 'STU001'`).Scan(&storedHash)
	if err != nil {
		t.Fatalf("query stored password: %v", err)
	}
	if !utils.CheckPasswordHash(customPass, storedHash) {
		t.Errorf("expected existing custom password to be preserved when CSV omits password, but hash changed")
	}

	// 3. Re-import CSV with explicit default password "siswa123"
	// P2-22: default placeholder in CSV should also not wipe the existing custom password
	csvDefPass := "no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin,password\nSTU001,Updated Student Name 2,1,1,L,siswa123\n"
	status, body = postCSVData(t, app, "/admin/students/import-csv", csvDefPass)
	if status != fiber.StatusOK {
		t.Fatalf("import with default password returned %d: %s", status, string(body))
	}

	err = database.QueryRow(`SELECT password FROM peserta WHERE tenant_id = 1 AND no_id = 'STU001'`).Scan(&storedHash)
	if err != nil {
		t.Fatalf("query stored password: %v", err)
	}
	if !utils.CheckPasswordHash(customPass, storedHash) {
		t.Errorf("expected existing custom password to be preserved when CSV supplies 'siswa123', but hash changed")
	}

	// 4. Re-import CSV with a genuinely new explicit password
	newPass := "NewExplicitPass999!"
	csvNewPass := fmt.Sprintf("no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin,password\nSTU001,Updated Student Name 3,1,1,L,%s\n", newPass)
	status, body = postCSVData(t, app, "/admin/students/import-csv", csvNewPass)
	if status != fiber.StatusOK {
		t.Fatalf("import with new password returned %d: %s", status, string(body))
	}

	err = database.QueryRow(`SELECT password FROM peserta WHERE tenant_id = 1 AND no_id = 'STU001'`).Scan(&storedHash)
	if err != nil {
		t.Fatalf("query stored password: %v", err)
	}
	if !utils.CheckPasswordHash(newPass, storedHash) {
		t.Errorf("expected new explicit password to update the student hash")
	}

	// 5. Brand new student in CSV without password column gets default password
	csvBrandNew := "no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\nSTU_NEW,Brand New Student,1,1,P\n"
	status, body = postCSVData(t, app, "/admin/students/import-csv", csvBrandNew)
	if status != fiber.StatusOK {
		t.Fatalf("import brand new student returned %d: %s", status, string(body))
	}

	var brandNewHash string
	err = database.QueryRow(`SELECT password FROM peserta WHERE tenant_id = 1 AND no_id = 'STU_NEW'`).Scan(&brandNewHash)
	if err != nil {
		t.Fatalf("query brand new student password: %v", err)
	}
	if !utils.CheckPasswordHash("siswa123", brandNewHash) {
		t.Errorf("expected brand new student without explicit password to receive default 'siswa123' hash")
	}
}

func TestImportStudentsCSV_DuplicateNoIDInCSV(t *testing.T) {
	app, _ := newStudentApp(t)

	csvDup := "no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\n" +
		"DUP001,Student First,1,1,L\n" +
		"DUP002,Student Second,1,1,P\n" +
		"DUP001,Student Duplicate,1,1,L\n"

	status, body := postCSVData(t, app, "/admin/students/import-csv", csvDup)
	if status != fiber.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for duplicate no_id in CSV, got %d (body: %s)", status, string(body))
	}

	if !strings.Contains(string(body), "duplikat nomor peserta 'DUP001'") {
		t.Errorf("expected response to mention duplicate no_id 'DUP001', got: %s", string(body))
	}
}

func TestImportStudentsCSV_MaxRowLimit(t *testing.T) {
	app, _ := newStudentApp(t)

	var sb strings.Builder
	sb.WriteString("no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\n")
	// 5001 data rows
	for i := 1; i <= 5001; i++ {
		sb.WriteString(fmt.Sprintf("ROW%05d,Student %d,1,1,L\n", i, i))
	}

	status, body := postCSVData(t, app, "/admin/students/import-csv", sb.String())
	if status != fiber.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when exceeding max rows, got %d", status)
	}

	if !strings.Contains(string(body), "melebihi batas maksimum") {
		t.Errorf("expected error to mention exceeding max row limit, got: %s", string(body))
	}
}

func TestImportStudentsCSV_FormulaSanitization(t *testing.T) {
	app, database := newStudentApp(t)

	csvFormula := "no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\n" +
		"=INJ001,=cmd|' /C calc'!A0,1,1,L\n" +
		"+INJ002,+malicious_name,1,1,P\n" +
		"@INJ003,@SUM(A1:A10),1,1,L\n"

	status, body := postCSVData(t, app, "/admin/students/import-csv", csvFormula)
	if status != fiber.StatusOK {
		t.Fatalf("import with formula characters returned %d: %s", status, string(body))
	}

	// Verify in database that formula prefixes are neutralized with leading single quote
	var name1, name2, name3 string
	err := database.QueryRow(`SELECT nama_peserta FROM peserta WHERE tenant_id = 1 AND no_id = ?`, "'=INJ001").Scan(&name1)
	if err != nil {
		t.Fatalf("query '=INJ001: %v", err)
	}
	if name1 != "'=cmd|' /C calc'!A0" {
		t.Errorf("expected sanitized name \"'=cmd|' /C calc'!A0\", got %q", name1)
	}

	err = database.QueryRow(`SELECT nama_peserta FROM peserta WHERE tenant_id = 1 AND no_id = ?`, "'+INJ002").Scan(&name2)
	if err != nil {
		t.Fatalf("query '+INJ002: %v", err)
	}
	if name2 != "'+malicious_name" {
		t.Errorf("expected sanitized name \"'+malicious_name\", got %q", name2)
	}

	err = database.QueryRow(`SELECT nama_peserta FROM peserta WHERE tenant_id = 1 AND no_id = ?`, "'@INJ003").Scan(&name3)
	if err != nil {
		t.Fatalf("query '@INJ003: %v", err)
	}
	if name3 != "'@SUM(A1:A10)" {
		t.Errorf("expected sanitized name \"'@SUM(A1:A10)\", got %q", name3)
	}
}

// TestImportStudentsCSV_UniquePasswordCap (P2-23) asserts that importing a CSV with more than
// MaxUniquePasswordsPerImport (500) distinct passwords fails with 400 Bad Request to prevent bcrypt DoS.
func TestImportStudentsCSV_UniquePasswordCap(t *testing.T) {
	app, _ := newStudentApp(t)

	var sb strings.Builder
	sb.WriteString("no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin,password\n")
	for i := 1; i <= 501; i++ {
		sb.WriteString(fmt.Sprintf("CAP%04d,Siswa %d,1,1,L,secret_pass_%d\n", i, i, i))
	}

	status, body := postCSVData(t, app, "/admin/students/import-csv", sb.String())
	if status != fiber.StatusBadRequest {
		t.Fatalf("importing >500 unique passwords returned %d (want 400 Bad Request): %s", status, string(body))
	}
	if !strings.Contains(string(body), "password unik") {
		t.Errorf("expected error message to mention unique password cap, got: %s", string(body))
	}
}

// TestStudentLogin_FormulaPrefixedNoID verifies that a student imported with a formula prefix
// (e.g. +628111 or =CMD) can successfully log in using their raw input ID without error.
func TestStudentLogin_FormulaPrefixedNoID(t *testing.T) {
	setupStudentAuthFlowDB(t)
	defer TeardownTestDB()
	utils.SetJWTSecret("test-jwt-secret-student-login-formula-injection")

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("tenant_id", 1)
		return c.Next()
	})
	app.Post("/auth/student-login", StudentLogin)

	// Pre-insert a student with formula-sanitized no_id '+6281110001
	hashedPW, err := utils.HashPassword("mypassword123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	_, err = db.DB.Exec(`
		INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id)
		VALUES (88, 1, "'+6281110001", ?, "Siswa Plus Phone", 1, 1)
	`, hashedPW)
	if err != nil {
		t.Fatalf("insert formula student: %v", err)
	}

	// Student logs in typing raw "+6281110001" (without leading single quote)
	payload := `{"no_id":"+6281110001","password":"mypassword123","token":"ujian2026"}`
	req := httptest.NewRequest("POST", "/auth/student-login", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected login status 200 OK for raw formula no_id, got %d", resp.StatusCode)
	}
}

// TestAuthLogin_RejectsSupervisorUserRole (P1-7) verifies that user accounts with role 'supervisor'
// cannot log in via /api/auth/login, forcing supervisor auth to use room credentials.
func TestAuthLogin_RejectsSupervisorUserRole(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	utils.SetJWTSecret("test-jwt-secret-p1-7-supervisor-rejection")

	hashed, _ := utils.HashPassword("secretpass")
	_, err := database.Exec(`
		INSERT INTO users (tenant_id, username, password_hash, role, full_name, is_active)
		VALUES (1, 'legacy_supervisor', ?, 'supervisor', 'Supervisor Account', TRUE)
	`, hashed)
	if err != nil {
		t.Fatalf("insert supervisor user: %v", err)
	}

	app.Post("/api/auth/login", Login)

	payload := `{"username":"legacy_supervisor","password":"secretpass"}`
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden for supervisor login via /api/auth/login, got %d", resp.StatusCode)
	}
}

// TestExportEssayResults_FormulaSanitization (P2-29) verifies that essay answers containing formula
// triggers (=, +, -, @) are sanitized with a prepended single quote in CSV and XLSX exports.
func TestExportEssayResults_FormulaSanitization(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()

	// Insert test data with formula injection characters in essay answer
	_, err := database.Exec(`
		INSERT INTO kelas (id, tenant_id, nama_kelas) VALUES (1, 1, 'X-IPA-1');
		INSERT INTO mapel (id, tenant_id, nama_mapel, kode_mapel) VALUES (1, 1, 'Informatika', 'INF10');
		INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id) VALUES (1, 1, 'STU001', 'siswa123', 'Budi Santoso', 1);
		INSERT INTO hasil_tes (id, tenant_id, peserta_id, mapel_id, skor, skor_maks, status, validasi)
		VALUES (1, 1, 1, 1, 80, 100, 'submitted', '1_STU001_1');
		INSERT INTO hasil_tes_detail (id, hasil_tes_id, question_id, question_text, question_type, user_answer, awarded_points, max_points)
		VALUES (1, 1, 'Q_ESSAY_1', 'Jelaskan konsep TCP/IP', 'essayQuestion', '=CMD|''calc''!A0', 10, 10);
	`)
	if err != nil {
		t.Fatalf("insert essay test data: %v", err)
	}

	app.Get("/admin/results/essays/export/:format", ExportEssayResults)

	// Test CSV export
	req := httptest.NewRequest("GET", "/admin/results/essays/export/csv", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("csv export request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("csv export expected status 200, got %d", resp.StatusCode)
	}
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	csvBody := buf.String()
	if !strings.Contains(csvBody, "'=CMD|'calc'!A0") {
		t.Errorf("expected escaped formula in CSV, got: %s", csvBody)
	}

	// Test XLSX export
	reqXlsx := httptest.NewRequest("GET", "/admin/results/essays/export/xlsx", nil)
	respXlsx, err := app.Test(reqXlsx, -1)
	if err != nil {
		t.Fatalf("xlsx export request failed: %v", err)
	}
	if respXlsx.StatusCode != http.StatusOK {
		t.Fatalf("xlsx export expected status 200, got %d", respXlsx.StatusCode)
	}
	var bufXlsx bytes.Buffer
	bufXlsx.ReadFrom(respXlsx.Body)
	f, err := excelize.OpenReader(bytes.NewReader(bufXlsx.Bytes()))
	if err != nil {
		t.Fatalf("open excel reader: %v", err)
	}
	defer f.Close()
	val, err := f.GetCellValue("Rekap Esai Siswa", "G2")
	if err != nil {
		t.Fatalf("get cell value G2: %v", err)
	}
	if val != "'=CMD|'calc'!A0" {
		t.Errorf("expected escaped formula in XLSX G2, got %q", val)
	}
}
