package repository

import (
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

func TestExamRepository_CreateWithValidMapel(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")

	repo := NewExamRepository(database)
	exam, err := repo.Create(1, ExamInput{
		MapelID:          1,
		Tingkat:          strPtr("XII"),
		DurasiMenit:      90,
		KKM:              70,
		ShuffleQuestions: true,
		ShuffleAnswers:   false,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if exam.MapelID != 1 || exam.DurasiMenit != 90 || exam.KKM != 70 {
		t.Fatalf("unexpected exam: %+v", exam)
	}
	if !exam.ShuffleQuestions || exam.ShuffleAnswers {
		t.Errorf("shuffle flags roundtrip wrong: q=%v a=%v", exam.ShuffleQuestions, exam.ShuffleAnswers)
	}
	if exam.Tingkat == nil || *exam.Tingkat != "XII" {
		t.Errorf("Tingkat = %v, want XII", exam.Tingkat)
	}
	if exam.SoalPackageID != nil {
		t.Errorf("draft exam should have nil SoalPackageID, got %v", exam.SoalPackageID)
	}
}

func TestExamRepository_CreateRejectsMapelFromOtherTenant(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedMapel(t, database, 1, 2, "Other Mapel", "OTH") // mapel belongs to tenant 2

	repo := NewExamRepository(database)
	_, err := repo.Create(1, ExamInput{MapelID: 1})
	if err != ErrInvalidReference {
		t.Fatalf("expected ErrInvalidReference for mapel not in tenant, got %v", err)
	}
}

func TestExamRepository_CreateValidatesPackageLink(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedSoalPackage(t, database, 10, 1, "Pkg", "uuid-10")

	repo := NewExamRepository(database)

	// Valid package link succeeds.
	exam, err := repo.Create(1, ExamInput{MapelID: 1, SoalPackageID: intPtr(10)})
	if err != nil {
		t.Fatalf("Create with valid package: %v", err)
	}
	if exam.SoalPackageID == nil || *exam.SoalPackageID != 10 {
		t.Fatalf("SoalPackageID = %v, want 10", exam.SoalPackageID)
	}

	// Package from another tenant is rejected.
	seedTenant(t, database, 2, "other", "Other School")
	seedSoalPackage(t, database, 20, 2, "OtherPkg", "uuid-20")
	if _, err := repo.Create(1, ExamInput{MapelID: 1, SoalPackageID: intPtr(20)}); err != ErrInvalidReference {
		t.Fatalf("expected ErrInvalidReference for package not in tenant, got %v", err)
	}

	// Non-existent package is rejected.
	if _, err := repo.Create(1, ExamInput{MapelID: 1, SoalPackageID: intPtr(999)}); err != ErrInvalidReference {
		t.Fatalf("expected ErrInvalidReference for missing package, got %v", err)
	}
}

func TestExamRepository_GetByIDCrossTenantNotFound(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 5, 1, 1, nil)

	repo := NewExamRepository(database)
	if _, err := repo.GetByID(2, 5); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-tenant get, got %v", err)
	}
}

func TestExamRepository_ListTenantScoped(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedMapel(t, database, 2, 2, "Biologi", "BIO")
	seedExam(t, database, 1, 1, 1, nil)
	seedExam(t, database, 2, 1, 1, nil)
	seedExam(t, database, 3, 2, 2, nil)

	repo := NewExamRepository(database)
	got, err := repo.List(1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("tenant 1 should see 2 exams, got %d", len(got))
	}
	for _, e := range got {
		if e.TenantID != 1 {
			t.Errorf("tenant leak: exam %d tenant_id %d", e.ID, e.TenantID)
		}
	}
}

func TestExamRepository_Update(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")

	repo := NewExamRepository(database)
	exam, _ := repo.Create(1, ExamInput{MapelID: 1, DurasiMenit: 60, KKM: 50})

	updated, err := repo.Update(1, exam.ID, ExamInput{MapelID: 1, DurasiMenit: 120, KKM: 75, Nama: strPtr("UAS")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DurasiMenit != 120 || updated.KKM != 75 {
		t.Errorf("update did not persist: dur=%v kkm=%v", updated.DurasiMenit, updated.KKM)
	}
	if updated.Nama == nil || *updated.Nama != "UAS" {
		t.Errorf("Nama = %v, want UAS", updated.Nama)
	}
}

func TestExamRepository_DeleteSoftDeletesWhenNoActiveSession(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")

	repo := NewExamRepository(database)
	exam, _ := repo.Create(1, ExamInput{MapelID: 1})
	if err := repo.Delete(1, exam.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(1, exam.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after soft delete, got %v", err)
	}
}

func TestExamRepository_DeleteRejectsWhenScheduledOrActiveSession(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")

	repo := NewExamRepository(database)
	sid := 1000 // distinct session ids so the seeding loop never collides on the PK
	// Scheduled / active sessions block deletion (Requirement 2.5).
	for _, status := range []string{"terjadwal", "aktif"} {
		exam, _ := repo.Create(1, ExamInput{MapelID: 1})
		seedExamSession(t, database, sid, 1, exam.ID, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK-"+status, status)
		sid++
		if err := repo.Delete(1, exam.ID); err != ErrConflict {
			t.Fatalf("expected ErrConflict for session status %q, got %v", status, err)
		}
	}

	// Draft / finished / cancelled sessions do not block deletion.
	for _, status := range []string{"draft", "selesai", "dibatalkan"} {
		exam, _ := repo.Create(1, ExamInput{MapelID: 1})
		seedExamSession(t, database, sid, 1, exam.ID, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK-"+status, status)
		sid++
		if err := repo.Delete(1, exam.ID); err != nil {
			t.Fatalf("expected delete to succeed with session status %q, got %v", status, err)
		}
	}
}

func TestExamRepository_DeleteNotFoundForMissingExam(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	repo := NewExamRepository(database)
	if err := repo.Delete(1, 99999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
