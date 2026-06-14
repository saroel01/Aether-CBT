package repository

import (
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

func TestGradeRepository_ListTingkatEmptyBeforeAnySet(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedKelas(t, database, 1, 1, "XII IPA 1")

	repo := NewGradeRepository(database)
	tingkat, err := repo.ListTingkat(1)
	if err != nil {
		t.Fatalf("ListTingkat: %v", err)
	}
	if len(tingkat) != 0 {
		t.Fatalf("expected no tingkat before any is set, got %v", tingkat)
	}
}

func TestGradeRepository_SetTingkatThenListReturnsIt(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedKelas(t, database, 1, 1, "XII IPA 1")
	seedKelas(t, database, 2, 1, "XI IPS 2")

	repo := NewGradeRepository(database)
	if err := repo.SetTingkat(1, 1, "XII"); err != nil {
		t.Fatalf("SetTingkat class 1: %v", err)
	}
	if err := repo.SetTingkat(1, 2, "XI"); err != nil {
		t.Fatalf("SetTingkat class 2: %v", err)
	}

	tingkat, err := repo.ListTingkat(1)
	if err != nil {
		t.Fatalf("ListTingkat: %v", err)
	}
	// DISTINCT, ordered: ["XI", "XII"]
	want := []string{"XI", "XII"}
	if len(tingkat) != len(want) {
		t.Fatalf("expected %v, got %v", want, tingkat)
	}
	for i := range want {
		if tingkat[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, tingkat)
		}
	}
}

func TestGradeRepository_SetTingkatRejectsClassFromOtherTenant(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedKelas(t, database, 1, 1, "XII IPA 1") // belongs to tenant 1

	repo := NewGradeRepository(database)
	// Tenant 2 must not be able to set tingkat on tenant 1's class.
	err := repo.SetTingkat(2, 1, "XII")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-tenant class, got %v", err)
	}

	// Tenant 1 still has no tingkat set (the update was a no-op).
	tingkat, _ := repo.ListTingkat(1)
	if len(tingkat) != 0 {
		t.Fatalf("cross-tenant SetTingkat must not mutate tenant 1, got %v", tingkat)
	}
}

func TestGradeRepository_SetTingkatNotFoundForMissingClass(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")

	repo := NewGradeRepository(database)
	if err := repo.SetTingkat(1, 99999, "X"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing class, got %v", err)
	}
}

func TestGradeRepository_ListTingkatIsolatedPerTenant(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedKelas(t, database, 1, 1, "A")
	seedKelas(t, database, 2, 2, "B")

	repo := NewGradeRepository(database)
	_ = repo.SetTingkat(1, 1, "XII")
	_ = repo.SetTingkat(2, 2, "X")

	if tingkat, _ := repo.ListTingkat(1); len(tingkat) != 1 || tingkat[0] != "XII" {
		t.Fatalf("tenant 1 should see only [XII], got %v", tingkat)
	}
	if tingkat, _ := repo.ListTingkat(2); len(tingkat) != 1 || tingkat[0] != "X" {
		t.Fatalf("tenant 2 should see only [X], got %v", tingkat)
	}
}
