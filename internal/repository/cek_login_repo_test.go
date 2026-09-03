package repository

import (
	"errors"
	"sync"
	"testing"

	"github.com/saroel01/aether-cbt/internal/testutil"
	"pgregory.net/rapid"
)

func TestCekLoginRepository_StartIsIdempotentBySession(t *testing.T) {
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
	// Restarting the same session updates the token without creating a duplicate row.
	if err := repo.Start(1, 1, 1, "attempt-2"); err != nil {
		t.Fatalf("Start (restart): %v", err)
	}

	got, err := repo.GetBySession(1, 1, 1)
	if err != nil {
		t.Fatalf("GetBySession: %v", err)
	}
	if got.AttemptToken == nil || *got.AttemptToken != "attempt-2" {
		t.Errorf("AttemptToken = %v, want attempt-2", got.AttemptToken)
	}
	if got.SessionID == nil || *got.SessionID != 1 {
		t.Errorf("SessionID = %v, want 1", got.SessionID)
	}
	if got.Locked {
		t.Errorf("newly started session should not be locked")
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cek_login WHERE tenant_id=1 AND peserta_id=1 AND session_id=1`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 active session row, got %d", count)
	}
}

func TestCekLoginRepository_LockAndUnlock(t *testing.T) {
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
	_ = repo.Start(1, 1, 1, "tok")

	locked, err := repo.IsLocked(1, 1, 1)
	if err != nil {
		t.Fatalf("IsLocked initial: %v", err)
	}
	if locked {
		t.Fatal("session should start unlocked")
	}

	if err := repo.Lock(1, 1, 1); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if locked, _ := repo.IsLocked(1, 1, 1); !locked {
		t.Error("expected locked after Lock")
	}
	got, _ := repo.GetBySession(1, 1, 1)
	if !got.Locked {
		t.Error("GetBySession should report Locked=true after Lock")
	}

	if err := repo.Unlock(1, 1, 1); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if locked, _ := repo.IsLocked(1, 1, 1); locked {
		t.Error("expected unlocked after Unlock")
	}
}

func TestCekLoginRepository_IncrementInfraction(t *testing.T) {
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
	_ = repo.Start(1, 1, 1, "tok")

	for i, want := range []int{1, 2, 3} {
		got, err := repo.IncrementInfraction(1, 1, 1)
		if err != nil {
			t.Fatalf("IncrementInfraction #%d: %v", i+1, err)
		}
		if got != want {
			t.Errorf("infraction #%d: got %d, want %d", i+1, got, want)
		}
	}
}

func TestCekLoginRepository_UpdateProgress(t *testing.T) {
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
	_ = repo.Start(1, 1, 1, "tok")
	if err := repo.UpdateProgress(1, 1, 1, 12, 40); err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}
	got, _ := repo.GetBySession(1, 1, 1)
	if got.AnsweredCount != 12 || got.TotalQuestions != 40 {
		t.Errorf("progress = answered=%d total=%d, want 12/40", got.AnsweredCount, got.TotalQuestions)
	}
}

func TestCekLoginRepository_ContentTokenLookup(t *testing.T) {
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
	_ = repo.Start(1, 1, 1, "tok")
	if err := repo.SetContentToken(1, 1, 1, "content-secret"); err != nil {
		t.Fatalf("SetContentToken: %v", err)
	}

	got, err := repo.GetByContentToken("content-secret")
	if err != nil {
		t.Fatalf("GetByContentToken: %v", err)
	}
	if got.TenantID != 1 || got.PesertaID != 1 {
		t.Errorf("lookup returned wrong session: %+v", got)
	}
	if got.ContentToken == nil || *got.ContentToken != "content-secret" {
		t.Errorf("ContentToken = %v", got.ContentToken)
	}

	if _, err := repo.GetByContentToken("nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for unknown content token, got %v", err)
	}
}

func TestCekLoginRepository_GetBySessionCrossTenantNotFound(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedTenant(t, database, 2, "other", "Other School")
	seedKelas(t, database, 1, 1, "XII IPA 1")
	seedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	seedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)
	seedExamSession(t, database, 1, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK", "aktif")

	repo := NewCekLoginRepository(database)
	_ = repo.Start(1, 1, 1, "tok")
	if _, err := repo.GetBySession(2, 1, 1); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound cross-tenant, got %v", err)
	}
}

// TestCekLoginRepository_StartRejectsSecondActiveSession verifies the partial unique index
// (tenant_id, peserta_id) WHERE session_id IS NOT NULL: a peserta may not hold two concurrent
// active session-based sessions. Starting a second distinct session must return ErrConflict
// (review H4, Task 14).
func TestCekLoginRepository_StartRejectsSecondActiveSession(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedKelas(t, database, 1, 1, "XII IPA 1")
	seedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	seedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)
	seedExamSession(t, database, 1, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK1", "aktif")
	seedExamSession(t, database, 2, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK2", "aktif")

	repo := NewCekLoginRepository(database)
	// First start (session 1) succeeds.
	if err := repo.Start(1, 1, 1, "attempt-1"); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	// Second start for a DIFFERENT session (2) must be rejected: the peserta already has an
	// active session-based row, and the partial unique index forbids a second.
	err := repo.Start(1, 1, 2, "attempt-2")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("second Start: err = %v, want ErrConflict", err)
	}
}



// TestProperty_AtomicInfractionIncrement tests Clause 2.22 / C22: atomic increment with RETURNING.
func TestProperty_AtomicInfractionIncrement(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedTenant(t, database, 1, "default", "Default School")
	seedKelas(t, database, 1, 1, "XII IPA 1")
	seedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	seedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	seedMapel(t, database, 1, 1, "Kimia", "KIM")
	seedExam(t, database, 1, 1, 1, nil)
	seedExamSession(t, database, 1, 1, 1, "2026-06-01 08:00:00", "2026-06-01 10:00:00", "TOK1", "aktif")

	repo := NewCekLoginRepository(database)
	if err := repo.Start(1, 1, 1, "attempt-1"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rapid.Check(t, func(rt *rapid.T) {
		numIncrements := rapid.IntRange(1, 10).Draw(rt, "numIncrements")
		var wg sync.WaitGroup
		results := make(chan int, numIncrements)

		for i := 0; i < numIncrements; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := repo.IncrementInfraction(1, 1, 1)
				if err == nil {
					results <- c
				}
			}()
		}
		wg.Wait()
		close(results)

		seen := make(map[int]bool)
		for val := range results {
			if seen[val] {
				rt.Fatalf("duplicate count returned under concurrent increments: %d", val)
			}
			seen[val] = true
		}

		var finalCount int
		if err := database.QueryRow(`SELECT tab_switch_count FROM cek_login WHERE tenant_id=1 AND peserta_id=1 AND session_id=1`).Scan(&finalCount); err != nil {
			rt.Fatalf("query final count: %v", err)
		}
		if len(seen) != numIncrements {
			rt.Fatalf("expected %d unique increments, got %d", numIncrements, len(seen))
		}
		if finalCount < numIncrements {
			rt.Fatalf("final count in db = %d, want at least %d", finalCount, numIncrements)
		}
	})
}
