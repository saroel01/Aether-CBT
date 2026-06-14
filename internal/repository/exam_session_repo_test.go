package repository

import (
	"testing"
	"time"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

// atTime builds a UTC timestamp on 2026-06-DD HH:00:00 for readable window tests.
func atTime(day, hour int) time.Time {
	return time.Date(2026, 6, day, hour, 0, 0, 0, time.UTC)
}

func TestExamSessionRepository_CreateThenGetByID(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)

	repo := NewExamSessionRepository(database)
	s, err := repo.Create(1, SessionInput{
		ExamID:       1,
		Nama:         strPtr("Sesi 1"),
		WaktuMulai:   atTime(1, 8),
		WaktuSelesai: atTime(1, 10),
		Token:        "TOK-A",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.Token != "TOK-A" || s.Status != "draft" {
		t.Fatalf("unexpected session: token=%s status=%s", s.Token, s.Status)
	}
	if !s.WaktuMulai.Equal(atTime(1, 8)) {
		t.Errorf("WaktuMulai = %v, want %v", s.WaktuMulai, atTime(1, 8))
	}

	got, err := repo.GetByID(1, s.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Token != "TOK-A" {
		t.Errorf("GetByID token = %s", got.Token)
	}
}

func TestExamSessionRepository_CreateRejectsExamFromOtherTenant(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil) // exam belongs to tenant 1

	repo := NewExamSessionRepository(database)
	_, err := repo.Create(2, SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK"})
	if err != ErrInvalidReference {
		t.Fatalf("expected ErrInvalidReference for exam not in tenant, got %v", err)
	}
}

func TestExamSessionRepository_ListTenantScoped(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedMapel(t, database, 2, 2, "Biologi", "BIO")
	seedExam(t, database, 1, 1, 1, nil)
	seedExam(t, database, 2, 2, 2, nil)
	seedExamSession(t, database, 1, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "T1", "terjadwal")
	seedExamSession(t, database, 2, 2, 2, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "T2", "terjadwal")

	repo := NewExamSessionRepository(database)
	got, err := repo.List(1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].TenantID != 1 {
		t.Fatalf("tenant 1 should see 1 own session, got %+v", got)
	}
}

func TestExamSessionRepository_Update(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)

	repo := NewExamSessionRepository(database)
	s, _ := repo.Create(1, SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK", Status: "draft"})

	updated, err := repo.Update(1, s.ID, SessionInput{ExamID: 1, WaktuMulai: atTime(1, 9), WaktuSelesai: atTime(1, 12), Token: "TOK2", Status: "terjadwal"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Token != "TOK2" || updated.Status != "terjadwal" {
		t.Errorf("update did not persist: token=%s status=%s", updated.Token, updated.Status)
	}
	if !updated.WaktuSelesai.Equal(atTime(1, 12)) {
		t.Errorf("WaktuSelesai = %v, want 12:00", updated.WaktuSelesai)
	}
}

func TestExamSessionRepository_DeleteSoftDeletes(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)

	repo := NewExamSessionRepository(database)
	s, _ := repo.Create(1, SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK"})
	if err := repo.Delete(1, s.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(1, s.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after soft delete, got %v", err)
	}
}

func TestExamSessionRepository_AttachClassesRejectsCrossTenant(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)
	seedKelas(t, database, 1, 1, "XII IPA 1")
	seedKelas(t, database, 2, 2, "Other class") // belongs to tenant 2

	repo := NewExamSessionRepository(database)
	s, _ := repo.Create(1, SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK"})

	// All-tenant classes succeed.
	if err := repo.AttachClasses(1, s.ID, []int{1}); err != nil {
		t.Fatalf("AttachClasses valid: %v", err)
	}

	// A cross-tenant class id is rejected and the whole op is refused (Requirement 4.7).
	if err := repo.AttachClasses(1, s.ID, []int{1, 2}); err != ErrInvalidReference {
		t.Fatalf("expected ErrInvalidReference for cross-tenant class, got %v", err)
	}
}

func TestExamSessionRepository_AttachRoomsRejectsCrossTenant(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)
	seedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	seedRuang(t, database, 2, 2, "Ruang B", "ruang_b")

	repo := NewExamSessionRepository(database)
	s, _ := repo.Create(1, SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK"})

	if err := repo.AttachRooms(1, s.ID, []int{1}); err != nil {
		t.Fatalf("AttachRooms valid: %v", err)
	}
	if err := repo.AttachRooms(1, s.ID, []int{2}); err != ErrInvalidReference {
		t.Fatalf("expected ErrInvalidReference for cross-tenant room, got %v", err)
	}
}

func TestExamSessionRepository_TokenOverlaps(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)

	repo := NewExamSessionRepository(database)
	existing, _ := repo.Create(1, SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK"})

	cases := []struct {
		name     string
		token    string
		mulai    time.Time
		selesai  time.Time
		exclude  int
		overlaps bool
	}{
		{"same token overlapping window", "TOK", atTime(1, 9), atTime(1, 11), 0, true},
		{"same token fully inside", "TOK", atTime(1, 8), atTime(1, 10), 0, true},
		{"same token adjacent (no overlap)", "TOK", atTime(1, 10), atTime(1, 12), 0, false},
		{"same token disjoint", "TOK", atTime(1, 11), atTime(1, 13), 0, false},
		{"different token overlapping", "OTHER", atTime(1, 9), atTime(1, 11), 0, false},
		{"same token self-excluded", "TOK", atTime(1, 8), atTime(1, 10), existing.ID, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.TokenOverlaps(1, tc.token, tc.mulai, tc.selesai, tc.exclude)
			if err != nil {
				t.Fatalf("TokenOverlaps: %v", err)
			}
			if got != tc.overlaps {
				t.Errorf("TokenOverlaps(%s, %s..%s, exclude=%d) = %v, want %v", tc.token, tc.mulai, tc.selesai, tc.exclude, got, tc.overlaps)
			}
		})
	}
}

func TestExamSessionRepository_FindByToken(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedMapel(t, database, 2, 2, "Biologi", "BIO")
	seedExam(t, database, 1, 1, 1, nil)
	seedExam(t, database, 2, 2, 2, nil)
	// Same token, non-overlapping windows in tenant 1 (allowed); same token in tenant 2.
	repo := NewExamSessionRepository(database)
	_, _ = repo.Create(1, SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "SHARED"})
	_, _ = repo.Create(1, SessionInput{ExamID: 1, WaktuMulai: atTime(2, 8), WaktuSelesai: atTime(2, 10), Token: "SHARED"})
	_, _ = repo.Create(2, SessionInput{ExamID: 2, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "SHARED"})

	got, err := repo.FindByToken(1, "SHARED")
	if err != nil {
		t.Fatalf("FindByToken: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("tenant 1 should find 2 SHARED sessions, got %d", len(got))
	}
	for _, s := range got {
		if s.TenantID != 1 {
			t.Errorf("tenant leak: session %d tenant %d", s.ID, s.TenantID)
		}
	}

	// Non-existent token returns ErrNotFound.
	if _, err := repo.FindByToken(1, "NOPE"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for unknown token, got %v", err)
	}
}
