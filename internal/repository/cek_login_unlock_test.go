package repository

import (
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
)

// TestCekLoginUnlockResetsInfractionCounter (audit L1): after an unlock the counter starts
// over, so one more infraction yields 1 instead of immediately re-reaching the threshold.
func TestCekLoginUnlockResetsInfractionCounter(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedKelas(t, database, 1, 1, "XII IPA 1")
	seedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	seedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)
	seedExamSession(t, database, 1, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK", "aktif")

	repo := NewCekLoginRepository(database)
	if err := repo.Start(1, 1, 1, "attempt-1"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := repo.IncrementInfraction(1, 1, 1); err != nil {
			t.Fatalf("IncrementInfraction: %v", err)
		}
	}
	if err := repo.Lock(1, 1, 1); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := repo.Unlock(1, 1, 1); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	n, err := repo.IncrementInfraction(1, 1, 1)
	if err != nil {
		t.Fatalf("IncrementInfraction after unlock: %v", err)
	}
	if n != 1 {
		t.Errorf("tab_switch_count after unlock + 1 infraction = %d, want 1", n)
	}
	if locked, _ := repo.IsLocked(1, 1, 1); locked {
		t.Error("session must stay unlocked after a single post-unlock infraction")
	}
}
