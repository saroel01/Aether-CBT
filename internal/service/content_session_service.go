package service

import (
	"database/sql"
	"errors"
	"time"

	"github.com/saroel01/aether-cbt/internal/repository"
)

// Content-session authorization errors. Handlers map them to HTTP statuses (design Error
// Handling): ErrContentUnauthorized -> 401 (missing/invalid token or broken chain),
// ErrContentLocked/ErrContentWindowClosed -> 403, ErrContentPackageMissing -> 404.
var (
	ErrContentUnauthorized   = errors.New("service: content token is invalid or session not found")
	ErrContentLocked         = errors.New("service: session is locked")
	ErrContentWindowClosed   = errors.New("service: session is not currently active")
	ErrContentPackageMissing = errors.New("service: exam has no linked soal package")
)

// ContentContext is the authorized view of an active content session: everything the serving
// handler needs to locate the package on disk and to populate the iSpring shim context.
type ContentContext struct {
	TenantID     int
	PesertaID    int
	SessionID    int
	ExamID       int
	PackageID    int
	PackageUUID  string
	EntryPath    string
	AttemptToken string
	SID          string // peserta.no_id; the shim appends it so the webhook can rebuild validasi
}

// ContentSessionService authorizes content serving from the content-session cookie token. It
// composes the repository layer and resolves the full token -> session -> exam -> package
// chain, enforcing tenant, window, and lock before returning what to serve (Requirements
// 8.1, 8.5; AD-2). The tenant is derived from the token itself: the iSpring player loads
// sub-assets without a tenant header, so the token is the sole authority. The token is an
// unguessable per-session capability (256-bit crypto/rand) and is constrained 1:1 to a
// cek_login row by the unique partial index idx_cek_login_content_token_unique (migration
// 026); the whole downstream chain is then scoped to that row's tenant, so an attacker
// holding only their own token can never resolve another tenant's data.
type ContentSessionService struct {
	cekLogins *repository.CekLoginRepository
	sessions  *repository.ExamSessionRepository
	exams     *repository.ExamRepository
	packages  *repository.SoalPackageRepository
	db        *sql.DB // peserta.no_id read (no peserta repository exists yet)
	now       func() time.Time
}

// ContentOption configures a ContentSessionService.
type ContentOption func(*ContentSessionService)

// WithContentClock overrides the service clock (defaults to time.Now) for deterministic tests.
func WithContentClock(now func() time.Time) ContentOption {
	return func(s *ContentSessionService) { s.now = now }
}

// NewContentSessionService builds a content-session service over the given DB. Repositories
// are constructed from the DB (cheap), so production wires one global DB and tests inject a
// per-test migrated DB (Requirement 16.7).
func NewContentSessionService(db *sql.DB, opts ...ContentOption) *ContentSessionService {
	s := &ContentSessionService{
		cekLogins: repository.NewCekLoginRepository(db),
		sessions:  repository.NewExamSessionRepository(db),
		exams:     repository.NewExamRepository(db),
		packages:  repository.NewSoalPackageRepository(db),
		db:        db,
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Authorize validates the content token and returns the package context to serve, or a typed
// error explaining why serving is refused.
func (s *ContentSessionService) Authorize(contentToken string) (*ContentContext, error) {
	cek, err := s.cekLogins.GetByContentToken(contentToken)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrContentUnauthorized
		}
		return nil, err
	}
	if cek.SessionID == nil {
		return nil, ErrContentUnauthorized
	}
	// Server lock is authoritative: a locked session is refused before any further work
	// (Requirement 10.3, Property 11).
	if cek.Locked {
		return nil, ErrContentLocked
	}

	session, err := s.sessions.GetByID(cek.TenantID, *cek.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrContentUnauthorized
		}
		return nil, err
	}
	// Effective window computed from server time (Requirement 8.5, Property 5).
	if !enterable(session.Status, session.WaktuMulai, session.WaktuSelesai, s.now()) {
		return nil, ErrContentWindowClosed
	}

	exam, err := s.exams.GetByID(cek.TenantID, session.ExamID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrContentUnauthorized
		}
		return nil, err
	}
	if exam.SoalPackageID == nil {
		return nil, ErrContentPackageMissing
	}
	pkg, err := s.packages.GetByID(cek.TenantID, *exam.SoalPackageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrContentPackageMissing
		}
		return nil, err
	}

	var noID string
	if err := s.db.QueryRow(
		`SELECT no_id FROM peserta WHERE id = ? AND tenant_id = ?`,
		cek.PesertaID, cek.TenantID,
	).Scan(&noID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrContentUnauthorized
		}
		return nil, err
	}

	attemptToken := ""
	if cek.AttemptToken != nil {
		attemptToken = *cek.AttemptToken
	}
	return &ContentContext{
		TenantID:     cek.TenantID,
		PesertaID:    cek.PesertaID,
		SessionID:    *cek.SessionID,
		ExamID:       session.ExamID,
		PackageID:    pkg.ID,
		PackageUUID:  pkg.PackageUUID,
		EntryPath:    pkg.EntryPath,
		AttemptToken: attemptToken,
		SID:          noID,
	}, nil
}
