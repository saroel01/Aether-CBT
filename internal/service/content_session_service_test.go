package service

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/saroel01/aether-cbt/internal/models"
	"github.com/saroel01/aether-cbt/internal/repository"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

// seedContentBase stands up the shared tenant/class/room/participant/subject graph.
func seedContentBase(t *testing.T, db *sql.DB) {
	t.Helper()
	testutil.SeedTenant(t, db, 1, "default", "Default School")
	testutil.SeedKelas(t, db, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, db, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedPeserta(t, db, 1, 1, 1, 1, "2026001", "Siswa")
	testutil.SeedMapel(t, db, 1, 1, "Kimia", "KIM")
}

// issueContentToken creates an aktif session in [day1 08:00, 10:00] for exam 1 (linked to
// package 10), starts the cek_login row, and stores the given content token. Returns the
// session id.
func issueContentToken(t *testing.T, db *sql.DB, contentToken string) int {
	t.Helper()
	testutil.SeedSoalPackage(t, db, 10, 1, "Pkg", "uuid-10")
	testutil.SeedExam(t, db, 1, 1, 1, intPtr(10))
	sessions := repository.NewExamSessionRepository(db)
	sess, err := sessions.Create(1, repository.SessionInput{
		ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10),
		Token: "TOK", Status: models.SessionStatusAktif,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	cekRepo := repository.NewCekLoginRepository(db)
	if err := cekRepo.Start(1, 1, sess.ID, "att-1"); err != nil {
		t.Fatalf("start cek_login: %v", err)
	}
	if err := cekRepo.SetContentToken(1, 1, sess.ID, contentToken); err != nil {
		t.Fatalf("set content token: %v", err)
	}
	return sess.ID
}

// TestContentSessionService_Authorize_HappyPath: a valid token for an in-window, unlocked
// session whose exam has a linked package resolves to a fully populated ContentContext.
func TestContentSessionService_Authorize_HappyPath(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedContentBase(t, database)
	issueContentToken(t, database, "ct-1")

	svc := NewContentSessionService(database, WithContentClock(clockAt(atTime(1, 9))))
	ctx, err := svc.Authorize("ct-1")
	if err != nil {
		t.Fatalf("Authorize: got err %v, want nil", err)
	}
	if ctx.TenantID != 1 || ctx.PesertaID != 1 || ctx.ExamID != 1 || ctx.PackageID != 10 {
		t.Errorf("identity mismatch: %+v", ctx)
	}
	if ctx.PackageUUID != "uuid-10" {
		t.Errorf("PackageUUID = %q, want uuid-10", ctx.PackageUUID)
	}
	if ctx.EntryPath != "index.html" {
		t.Errorf("EntryPath = %q, want index.html", ctx.EntryPath)
	}
	if ctx.AttemptToken != "att-1" {
		t.Errorf("AttemptToken = %q, want att-1", ctx.AttemptToken)
	}
	if ctx.SID != "2026001" {
		t.Errorf("SID = %q, want 2026001 (peserta.no_id)", ctx.SID)
	}
}

func TestContentSessionService_Authorize_InvalidToken(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedContentBase(t, database)
	svc := NewContentSessionService(database, WithContentClock(clockAt(atTime(1, 9))))
	if _, err := svc.Authorize("does-not-exist"); !errors.Is(err, ErrContentUnauthorized) {
		t.Errorf("invalid token: got %v, want ErrContentUnauthorized", err)
	}
}

func TestContentSessionService_Authorize_Locked(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedContentBase(t, database)
	sid := issueContentToken(t, database, "ct-lock")
	if err := repository.NewCekLoginRepository(database).Lock(1, 1, sid); err != nil {
		t.Fatalf("lock: %v", err)
	}
	svc := NewContentSessionService(database, WithContentClock(clockAt(atTime(1, 9))))
	if _, err := svc.Authorize("ct-lock"); !errors.Is(err, ErrContentLocked) {
		t.Errorf("locked session: got %v, want ErrContentLocked", err)
	}
}

func TestContentSessionService_Authorize_WindowEnded(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedContentBase(t, database)
	issueContentToken(t, database, "ct-end")
	svc := NewContentSessionService(database, WithContentClock(clockAt(atTime(1, 11)))) // after end
	if _, err := svc.Authorize("ct-end"); !errors.Is(err, ErrContentWindowClosed) {
		t.Errorf("ended session: got %v, want ErrContentWindowClosed", err)
	}
}

func TestContentSessionService_Authorize_WindowNotStarted(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedContentBase(t, database)
	issueContentToken(t, database, "ct-early")
	svc := NewContentSessionService(database, WithContentClock(clockAt(atTime(1, 7)))) // before start
	if _, err := svc.Authorize("ct-early"); !errors.Is(err, ErrContentWindowClosed) {
		t.Errorf("not-started session: got %v, want ErrContentWindowClosed", err)
	}
}

func TestContentSessionService_Authorize_NoPackage(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	seedContentBase(t, database)
	testutil.SeedExam(t, database, 1, 1, 1, nil) // exam without a linked package
	sessions := repository.NewExamSessionRepository(database)
	sess, err := sessions.Create(1, repository.SessionInput{
		ExamID: 1, WaktuMulai: atTime(1, 8), WaktuSelesai: atTime(1, 10),
		Token: "TOK", Status: models.SessionStatusAktif,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	cekRepo := repository.NewCekLoginRepository(database)
	if err := cekRepo.Start(1, 1, sess.ID, "att-1"); err != nil {
		t.Fatalf("start cek_login: %v", err)
	}
	if err := cekRepo.SetContentToken(1, 1, sess.ID, "ct-nopkg"); err != nil {
		t.Fatalf("set content token: %v", err)
	}
	svc := NewContentSessionService(database, WithContentClock(clockAt(atTime(1, 9))))
	if _, err := svc.Authorize("ct-nopkg"); !errors.Is(err, ErrContentPackageMissing) {
		t.Errorf("no package: got %v, want ErrContentPackageMissing", err)
	}
}
