package service

import (
	"testing"
	"time"

	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

func atTime(day, hour int) time.Time {
	return time.Date(2026, 6, day, hour, 0, 0, 0, time.UTC)
}

func clockAt(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestSchedulingService_ValidateWindow(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	svc := NewSchedulingService(repository.NewExamSessionRepository(database), repository.NewExamRepository(database))
	mulai := atTime(1, 8)

	if err := svc.ValidateWindow(mulai, mulai); err != ErrInvalidWindow {
		t.Errorf("equal start/end: got %v, want ErrInvalidWindow", err)
	}
	if err := svc.ValidateWindow(mulai, mulai.Add(-time.Hour)); err != ErrInvalidWindow {
		t.Errorf("end before start: got %v, want ErrInvalidWindow", err)
	}
	if err := svc.ValidateWindow(mulai, mulai.Add(time.Hour)); err != nil {
		t.Errorf("valid window: got %v, want nil", err)
	}
}

func TestSchedulingService_EffectiveEnterable(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	mulai := atTime(1, 8)
	selesai := atTime(1, 10)
	sess := &models.ExamSession{Status: models.SessionStatusTerjadwal, WaktuMulai: mulai, WaktuSelesai: selesai}

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before window", atTime(1, 7), false},
		{"at start (inclusive)", mulai, true},
		{"inside", atTime(1, 9), true},
		{"at end (inclusive)", selesai, true},
		{"after window", atTime(1, 11), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := NewSchedulingService(repository.NewExamSessionRepository(database), repository.NewExamRepository(database), WithClock(clockAt(c.now)))
			if got := svc.EffectiveEnterable(sess); got != c.want {
				t.Errorf("enterable = %v, want %v", got, c.want)
			}
		})
	}

	// Non-active statuses are never enterable, even inside the window.
	for _, status := range []string{models.SessionStatusDraft, models.SessionStatusSelesai, models.SessionStatusDibatalkan} {
		svc := NewSchedulingService(repository.NewExamSessionRepository(database), repository.NewExamRepository(database), WithClock(clockAt(atTime(1, 9))))
		if svc.EffectiveEnterable(&models.ExamSession{Status: status, WaktuMulai: mulai, WaktuSelesai: selesai}) {
			t.Errorf("status %s must never be enterable", status)
		}
	}
}

func TestSchedulingService_RemainingSeconds(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	sess := &models.ExamSession{WaktuSelesai: atTime(1, 10)}
	exam := &models.Exam{DurasiMenit: 90}

	cases := []struct {
		name string
		now  time.Time
		want int
	}{
		{"full duration when session has plenty of time", atTime(1, 8), 90 * 60},
		{"session-bound when less than duration remains", atTime(1, 9).Add(45 * time.Minute), 15 * 60},
		{"zero past end", atTime(1, 11), 0},
		{"zero at end", atTime(1, 10), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := NewSchedulingService(repository.NewExamSessionRepository(database), repository.NewExamRepository(database), WithClock(clockAt(c.now)))
			if got := svc.RemainingSeconds(sess, exam); got != c.want {
				t.Errorf("remaining = %d, want %d", got, c.want)
			}
		})
	}
}

func TestSchedulingService_ValidateCreateTokenOverlap(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)

	sessions := repository.NewExamSessionRepository(database)
	exams := repository.NewExamRepository(database)
	_, _ = sessions.Create(1, repository.SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK"})

	svc := NewSchedulingService(sessions, exams)

	// Same token, overlapping window -> conflict.
	if err := svc.ValidateCreate(1, repository.SessionInput{ExamID: 1, WaktuMulai: atTime(1, 9), WaktuSelesai: atTime(1, 11), Token: "TOK"}); err != ErrTokenConflict {
		t.Errorf("overlapping token: got %v, want ErrTokenConflict", err)
	}
	// Same token, non-overlapping window -> allowed.
	if err := svc.ValidateCreate(1, repository.SessionInput{ExamID: 1, WaktuMulai: atTime(2, 8), WaktuSelesai: atTime(2, 10), Token: "TOK"}); err != nil {
		t.Errorf("non-overlapping same token: got %v, want nil", err)
	}
	// Invalid window -> ErrInvalidWindow (checked before overlap).
	if err := svc.ValidateCreate(1, repository.SessionInput{ExamID: 1, WaktuMulai: atTime(1, 10), WaktuSelesai: atTime(1, 8), Token: "NEW"}); err != ErrInvalidWindow {
		t.Errorf("invalid window: got %v, want ErrInvalidWindow", err)
	}
}

func TestSchedulingService_ValidateUpdatePackageAndSelfExclusion(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedSoalPackage(t, database, 10, 1, "Pkg", "uuid-10")
	testutil.SeedExam(t, database, 1, 1, 1, nil)        // no package
	testutil.SeedExam(t, database, 2, 1, 1, intPtr(10)) // has package

	sessions := repository.NewExamSessionRepository(database)
	exams := repository.NewExamRepository(database)
	existing, _ := sessions.Create(1, repository.SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK"})

	svc := NewSchedulingService(sessions, exams)

	// Transition to terjadwal on exam WITHOUT package -> rejected.
	if err := svc.ValidateUpdate(1, existing.ID, repository.SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK", Status: models.SessionStatusTerjadwal}); err != ErrPackageRequired {
		t.Errorf("activate without package: got %v, want ErrPackageRequired", err)
	}
	// Same token/window on the session itself (self-excluded) + package present -> ok.
	if err := svc.ValidateUpdate(1, existing.ID, repository.SessionInput{ExamID: 2, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK", Status: models.SessionStatusAktif}); err != nil {
		t.Errorf("update with package, self-excluded token: got %v, want nil", err)
	}
	// Draft status never requires a package.
	if err := svc.ValidateUpdate(1, existing.ID, repository.SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK", Status: models.SessionStatusDraft}); err != nil {
		t.Errorf("draft update: got %v, want nil", err)
	}
}

func TestSchedulingService_IsParticipantEligible(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, database, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)

	sessions := repository.NewExamSessionRepository(database)
	exams := repository.NewExamRepository(database)
	sess, _ := sessions.Create(1, repository.SessionInput{ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10), Token: "TOK"})
	_ = sessions.AttachClasses(1, sess.ID, []int{1})

	svc := NewSchedulingService(sessions, exams)
	if ok, err := svc.IsParticipantEligible(1, 1, sess.ID); err != nil || !ok {
		t.Errorf("peserta 1 should be eligible, got ok=%v err=%v", ok, err)
	}
	if ok, err := svc.IsParticipantEligible(1, 999, sess.ID); err != nil || ok {
		t.Errorf("unknown peserta should not be eligible, got ok=%v err=%v", ok, err)
	}
}

func intPtr(i int) *int { return &i }
