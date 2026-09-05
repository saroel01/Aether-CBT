package handlers

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

func TestComputeStudentStatus_Inversion(t *testing.T) {
	submitted := "submitted"
	inProgress := "in_progress"

	// 1. Submitted beats locked beats in-progress, even when logged out (after worker deletion)
	if got := computeStudentStatus(false, false, &submitted); got != "submitted" {
		t.Errorf("got %q, want 'submitted' when logged out after submit", got)
	}
	if got := computeStudentStatus(false, true, &submitted); got != "submitted" {
		t.Errorf("got %q, want 'submitted' when locked and logged out after submit", got)
	}
	if got := computeStudentStatus(true, true, &submitted); got != "submitted" {
		t.Errorf("got %q, want 'submitted' when locked and logged in after submit", got)
	}

	// 2. Locked beats in-progress when logged in
	if got := computeStudentStatus(true, true, nil); got != "locked" {
		t.Errorf("got %q, want 'locked'", got)
	}
	if got := computeStudentStatus(true, true, &inProgress); got != "locked" {
		t.Errorf("got %q, want 'locked'", got)
	}

	// 3. In-progress when logged in and not locked
	if got := computeStudentStatus(true, false, nil); got != "in_progress" {
		t.Errorf("got %q, want 'in_progress'", got)
	}
	if got := computeStudentStatus(true, false, &inProgress); got != "in_progress" {
		t.Errorf("got %q, want 'in_progress'", got)
	}

	// 4. Not logged in when logged out and not submitted
	if got := computeStudentStatus(false, false, nil); got != "not_logged_in" {
		t.Errorf("got %q, want 'not_logged_in'", got)
	}
	if got := computeStudentStatus(false, false, &inProgress); got != "not_logged_in" {
		t.Errorf("got %q, want 'not_logged_in'", got)
	}
}

func TestRoomStatus_HistoryScoreLeakagePrevented(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor")
	defer cleanup()
	app.Get("/api/supervisor/room-status", GetRoomStatus)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang 1", "ruang_1")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)

	// Session 1 is yesterday's finished session
	testutil.SeedExamSession(t, database, 1, 1, 1,
		"2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK-1", "selesai")
	// Session 2 is today's active session
	testutil.SeedExamSession(t, database, 2, 1, 1,
		"2026-06-02 08:00:00", "2026-06-02 10:00:00", "TOK-2", "aktif")

	// Peserta 1 sat Session 1 yesterday and has a hasil_tes row with score 95
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa 1")
	_, err := database.Exec(`
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, exam_session_id, skor, skor_maks, status, waktu_selesai)
		VALUES (1, 1, 1, 1, 95.0, 100, 'submitted', '2026-06-01 09:30:00')
	`)
	if err != nil {
		t.Fatalf("seed hasil_tes: %v", err)
	}

	// Peserta 2 is in Session 2 today and submitted with score 80
	testutil.SeedPeserta(t, database, 2, 1, 1, 1, "2026002", "Siswa 2")
	_, err = database.Exec(`
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, exam_session_id, skor, skor_maks, status, waktu_selesai)
		VALUES (1, 2, 1, 2, 80.0, 100, 'submitted', '2026-06-02 09:30:00')
	`)
	if err != nil {
		t.Fatalf("seed hasil_tes: %v", err)
	}

	// Supervisor fetches room status without session_id (sessionID == 0)
	statuses := roomStatusOf(t, app)

	// Peserta 1 has NOT submitted today's session (session 2). Old score from session 1 must NOT appear.
	s1, ok := statuses[1]
	if !ok {
		t.Fatalf("peserta 1 missing from room status")
	}
	if s1.Skor != nil {
		t.Errorf("peserta 1 leaked old score: %v, want nil", *s1.Skor)
	}
	if s1.Status == "submitted" {
		t.Errorf("peserta 1 status is 'submitted', want 'not_logged_in'")
	}

	// Peserta 2 submitted today's active session (session 2). Score should appear.
	s2, ok := statuses[2]
	if !ok {
		t.Fatalf("peserta 2 missing from room status")
	}
	if s2.Skor == nil || *s2.Skor != 80.0 {
		t.Errorf("peserta 2 score = %v, want 80.0", s2.Skor)
	}
	if s2.Status != "submitted" {
		t.Errorf("peserta 2 status = %q, want 'submitted'", s2.Status)
	}
}

func TestUnlockStudentSession_ClearsLockWithoutDeletingSession(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor")
	defer cleanup()
	app.Post("/api/supervisor/unlock", UnlockStudentSession)

	sid := seedContentGraph(t, database, defaultTenant1Seed("ct-unlock"))
	cekRepo := repository.NewCekLoginRepository(database)

	// Lock the student session
	if err := cekRepo.Lock(1, 1, sid); err != nil {
		t.Fatalf("lock: %v", err)
	}

	locked, err := cekRepo.IsLocked(1, 1, sid)
	if err != nil || !locked {
		t.Fatalf("session should be locked")
	}

	// Supervisor unlocks the student
	body := strings.NewReader(fmt.Sprintf(`{"peserta_id":1,"session_id":%d}`, sid))
	resp := doJSON(t, app, "POST", "/api/supervisor/unlock", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Sesi tetap ada (tidak dihapus) dan status locked kembali false
	cek, err := cekRepo.GetBySession(1, 1, sid)
	if err != nil {
		t.Fatalf("session should not be deleted, got err: %v", err)
	}
	if cek.Locked {
		t.Errorf("session should be unlocked (locked == false)")
	}
}

func TestExportResultsCSV_HasUTF8BOM(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "supervisor")
	defer cleanup()
	app.Get("/api/admin/results/export-csv", ExportResultsCSV)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang 1", "ruang_1")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Ahmad")
	testutil.SeedMapel(t, database, 1, 1, "Matematika", "MTK")
	_, err := database.Exec(`
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, skor, skor_maks, status, waktu_selesai)
		VALUES (1, 1, 1, 85.0, 100, 'submitted', '2026-06-01 09:30:00')
	`)
	if err != nil {
		t.Fatalf("seed hasil_tes: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/results/export-csv", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	csvBytes := buf.Bytes()

	if len(csvBytes) < 3 || !bytes.Equal(csvBytes[:3], []byte("\xEF\xBB\xBF")) {
		t.Errorf("CSV output is missing UTF-8 BOM prefix (want \\xEF\\xBB\\xBF)")
	}
}

func TestValidation_DeletedAtForRuang(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/students", CreateStudent)
	app.Post("/admin/students/import-csv", ImportStudentsCSV)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang SoftDeleted", "ruang_del")

	// Soft delete the ruang
	_, err := database.Exec(`UPDATE ruang SET deleted_at = CURRENT_TIMESTAMP WHERE id = 1`)
	if err != nil {
		t.Fatalf("soft delete ruang: %v", err)
	}

	// 1. CreateStudent with deleted ruang must return 400
	createBody := strings.NewReader(`{
		"no_id":"2026099",
		"password":"password123",
		"nama_peserta":"Siswa Test",
		"kelas_id":1,
		"ruang_id":1,
		"jenis_kelamin":"L"
	}`)
	resp := doJSON(t, app, "POST", "/students", createBody)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("CreateStudent with deleted ruang: status = %d, want 400", resp.StatusCode)
	}

	// 2. ImportStudentsCSV with deleted ruang must return 400
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "students.csv")
	_, _ = part.Write([]byte("no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\n2026098,Siswa CSV,1,1,L\n"))
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/admin/students/import-csv", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("import request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("ImportStudentsCSV with deleted ruang: status = %d, want 400", resp.StatusCode)
	}
}

func TestImportStudentsCSV_BulkPerformance(t *testing.T) {
	app, _, database, cleanup := newAdminTestApp(t, "admin")
	defer cleanup()
	app.Post("/admin/students/import-csv", ImportStudentsCSV)

	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang 1", "ruang_1")

	// Generate 50 students with default password
	var csvData strings.Builder
	csvData.WriteString("no_id,nama_peserta,kelas_id,ruang_id,jenis_kelamin\n")
	for i := 1; i <= 50; i++ {
		csvData.WriteString(fmt.Sprintf("STUDENT_%03d,Student Name %d,1,1,L\n", i, i))
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "bulk_students.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte(csvData.String()))
	_ = writer.Close()

	start := time.Now()
	req := httptest.NewRequest("POST", "/admin/students/import-csv", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req, -1)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("import request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	t.Logf("Imported 50 students in %v", duration)
	if duration > 2*time.Second {
		t.Errorf("bulk import took %v, want < 2s (without caching at cost 14 it would take > 15s)", duration)
	}
}
