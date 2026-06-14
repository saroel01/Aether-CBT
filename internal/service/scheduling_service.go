package service

import (
	"time"

	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/repository"
)

// SchedulingService encodes the cross-entity scheduling rules: a session's effective
// (server-time) enterability, window validation, token-overlap enforcement on create and
// update, the package-required rule for status transitions, and server-side participant
// eligibility. It composes the repository layer and is given an injectable clock so the
// time-dependent logic is deterministic in tests (Requirements 2.5, 4.2-4.5, 5.3, 6.1,
// 7.5).
type SchedulingService struct {
	sessions *repository.ExamSessionRepository
	exams    *repository.ExamRepository
	now      func() time.Time
}

// Option configures a SchedulingService.
type Option func(*SchedulingService)

// WithClock overrides the service's clock (defaults to time.Now). Use in tests to make
// the time-dependent rules deterministic.
func WithClock(now func() time.Time) Option {
	return func(s *SchedulingService) { s.now = now }
}

// NewSchedulingService builds a service over the given repositories.
func NewSchedulingService(sessions *repository.ExamSessionRepository, exams *repository.ExamRepository, opts ...Option) *SchedulingService {
	svc := &SchedulingService{
		sessions: sessions,
		exams:    exams,
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// enterable reports whether a session with the given status and window can be entered at
// the given time: status must be terjadwal/aktif and the time must be within
// [mulai, selesai] inclusive (Requirement 4.5, 6.1, Property 5).
func enterable(status string, mulai, selesai, now time.Time) bool {
	if status != models.SessionStatusTerjadwal && status != models.SessionStatusAktif {
		return false
	}
	return !now.Before(mulai) && !now.After(selesai)
}

// remainingSeconds computes the remaining exam seconds as min(duration, sessionEnd - now),
// clamped to 0 so it is never negative (Requirement 7.5, Property 6).
func remainingSeconds(durasiMenit int, selesai, now time.Time) int {
	remaining := selesai.Sub(now)
	if d := time.Duration(durasiMenit) * time.Minute; d < remaining {
		remaining = d
	}
	if remaining < 0 {
		remaining = 0
	}
	return int(remaining.Seconds())
}

// windowsOverlap reports whether two [start, end) windows share any time. Adjacent windows
// (one ending exactly when the other starts) do not overlap. This is the predicate behind
// the token-overlap rule (Requirement 4.4, Property 8).
func windowsOverlap(aMulai, aSelesai, bMulai, bSelesai time.Time) bool {
	return aMulai.Before(bSelesai) && aSelesai.After(bMulai)
}

// EffectiveEnterable reports whether the session can be entered at the service's clock.
func (s *SchedulingService) EffectiveEnterable(sess *models.ExamSession) bool {
	return enterable(sess.Status, sess.WaktuMulai, sess.WaktuSelesai, s.now())
}

// RemainingSeconds returns the remaining exam seconds for an active session given its exam
// duration and the session end (Requirement 7.5).
func (s *SchedulingService) RemainingSeconds(sess *models.ExamSession, exam *models.Exam) int {
	return remainingSeconds(exam.DurasiMenit, sess.WaktuSelesai, s.now())
}

// ValidateWindow rejects a window whose end is not strictly after its start (Requirement 4.2).
func (s *SchedulingService) ValidateWindow(mulai, selesai time.Time) error {
	if !selesai.After(mulai) {
		return ErrInvalidWindow
	}
	return nil
}

// ValidateCreate enforces the rules that must hold before a new session is persisted: a
// valid window and a token that does not overlap another session's window in the tenant
// (Requirement 4.2, 4.4).
func (s *SchedulingService) ValidateCreate(tenantID int, in repository.SessionInput) error {
	if err := s.ValidateWindow(in.WaktuMulai, in.WaktuSelesai); err != nil {
		return err
	}
	overlaps, err := s.sessions.TokenOverlaps(tenantID, in.Token, in.WaktuMulai, in.WaktuSelesai, 0)
	if err != nil {
		return err
	}
	if overlaps {
		return ErrTokenConflict
	}
	return nil
}

// ValidateUpdate enforces the rules that must hold before a session is updated: a valid
// window, a token that does not overlap another (non-self) session's window, and - when
// transitioning to terjadwal/aktif - that the exam has a linked soal package
// (Requirement 4.2, 4.4, 2.5, 4.3).
func (s *SchedulingService) ValidateUpdate(tenantID, id int, in repository.SessionInput) error {
	if err := s.ValidateWindow(in.WaktuMulai, in.WaktuSelesai); err != nil {
		return err
	}
	overlaps, err := s.sessions.TokenOverlaps(tenantID, in.Token, in.WaktuMulai, in.WaktuSelesai, id)
	if err != nil {
		return err
	}
	if overlaps {
		return ErrTokenConflict
	}
	if in.Status == models.SessionStatusTerjadwal || in.Status == models.SessionStatusAktif {
		exam, err := s.exams.GetByID(tenantID, in.ExamID)
		if err != nil {
			return err
		}
		if exam.SoalPackageID == nil {
			return ErrPackageRequired
		}
	}
	return nil
}

// IsParticipantEligible reports whether the participant may enter the session, computed
// entirely server-side from class/room membership (Requirement 5.3, 5.4).
func (s *SchedulingService) IsParticipantEligible(tenantID, pesertaID, sessionID int) (bool, error) {
	return s.sessions.ParticipantEligible(tenantID, pesertaID, sessionID)
}
