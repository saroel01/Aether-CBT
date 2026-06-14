package repository

import (
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestSoalPackageRepository_CreateThenGetByID(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")

	repo := NewSoalPackageRepository(database)
	created, err := repo.Create(1, SoalPackageInput{
		Nama:           "Kimia XII UAS",
		PackageUUID:    "uuid-1",
		EntryPath:      "index.html",
		IspringVersion: strPtr("11.9.0.4"),
		TotalSize:      1536000,
		Checksum:       strPtr("sha256:abc"),
		UploadedBy:     intPtr(5),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero id")
	}

	got, err := repo.GetByID(1, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Nama != "Kimia XII UAS" || got.PackageUUID != "uuid-1" || got.EntryPath != "index.html" {
		t.Fatalf("unexpected package: %+v", got)
	}
	if got.TotalSize != 1536000 {
		t.Errorf("TotalSize = %d, want 1536000", got.TotalSize)
	}
	if got.IspringVersion == nil || *got.IspringVersion != "11.9.0.4" {
		t.Errorf("IspringVersion = %v, want 11.9.0.4", got.IspringVersion)
	}
	if got.Checksum == nil || *got.Checksum != "sha256:abc" {
		t.Errorf("Checksum = %v, want sha256:abc", got.Checksum)
	}
	if got.UploadedBy == nil || *got.UploadedBy != 5 {
		t.Errorf("UploadedBy = %v, want 5", got.UploadedBy)
	}
	if got.DeletedAt != nil {
		t.Errorf("DeletedAt = %v, want nil", got.DeletedAt)
	}
}

func TestSoalPackageRepository_CreateWithNullOptionals(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")

	repo := NewSoalPackageRepository(database)
	created, err := repo.Create(1, SoalPackageInput{
		Nama:        "Draft Package",
		PackageUUID: "uuid-2",
		EntryPath:   "", // should default to index.html
		TotalSize:   0,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.EntryPath != "index.html" {
		t.Errorf("EntryPath default = %q, want index.html", created.EntryPath)
	}
	if created.IspringVersion != nil || created.Checksum != nil || created.UploadedBy != nil {
		t.Errorf("nullable fields should be nil, got ver=%v csum=%v by=%v", created.IspringVersion, created.Checksum, created.UploadedBy)
	}
}

func TestSoalPackageRepository_GetByIDCrossTenantNotFound(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedSoalPackage(t, database, 10, 1, "P1", "uuid-1")

	repo := NewSoalPackageRepository(database)
	if _, err := repo.GetByID(2, 10); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-tenant get, got %v", err)
	}
}

func TestSoalPackageRepository_ListTenantScoped(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedSoalPackage(t, database, 1, 1, "P1", "uuid-1")
	seedSoalPackage(t, database, 2, 1, "P2", "uuid-2")
	seedSoalPackage(t, database, 3, 2, "P3", "uuid-3")

	repo := NewSoalPackageRepository(database)
	got, err := repo.List(1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("tenant 1 should see 2 packages, got %d (%+v)", len(got), got)
	}
	for _, p := range got {
		if p.TenantID != 1 {
			t.Errorf("tenant leak: package %d has tenant_id %d", p.ID, p.TenantID)
		}
	}
}

func TestSoalPackageRepository_DeleteUnlinkedSucceeds(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	repo := NewSoalPackageRepository(database)

	created, _ := repo.Create(1, SoalPackageInput{Nama: "Temp", PackageUUID: "uuid-x"})
	if err := repo.Delete(1, created.ID); err != nil {
		t.Fatalf("Delete unlinked: %v", err)
	}
	if _, err := repo.GetByID(1, created.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSoalPackageRepository_DeleteLinkedToExamReturnsConflict(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedSoalPackage(t, database, 10, 1, "Linked", "uuid-10")
	seedExam(t, database, 1, 1, 1, intPtr(10)) // exam links package 10

	repo := NewSoalPackageRepository(database)
	if err := repo.Delete(1, 10); err != ErrConflict {
		t.Fatalf("expected ErrConflict deleting linked package, got %v", err)
	}
	// Package still exists.
	if _, err := repo.GetByID(1, 10); err != nil {
		t.Fatalf("package should still exist after rejected delete, got %v", err)
	}
}

func TestSoalPackageRepository_DeleteCrossTenantNotFound(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedSoalPackage(t, database, 10, 1, "P1", "uuid-1")

	repo := NewSoalPackageRepository(database)
	if err := repo.Delete(2, 10); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-tenant delete, got %v", err)
	}
}
