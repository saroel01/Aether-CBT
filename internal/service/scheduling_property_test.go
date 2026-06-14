package service

import (
	"testing"
	"time"

	"github.com/saroel01/aether-cbt/internal/models"
	"pgregory.net/rapid"
)

// genTime generates an arbitrary UTC timestamp.
func genTime() *rapid.Generator[time.Time] {
	return rapid.Custom(func(t *rapid.T) time.Time {
		return time.Unix(rapid.Int64Range(0, 3e8).Draw(t, "secs"), 0).UTC()
	})
}

// genTimeAfter generates a timestamp strictly after the given base (>= 1 second later).
func genTimeAfter(base time.Time) *rapid.Generator[time.Time] {
	return rapid.Custom(func(t *rapid.T) time.Time {
		return base.Add(time.Duration(rapid.Int64Range(1, 1e7).Draw(t, "deltaSecs")) * time.Second).UTC()
	})
}

// Property 5: a session is enterable iff its status is terjadwal/aktif AND the clock is
// within [mulai, selesai] inclusive. Verified across random statuses and windows, which
// catches off-by-one errors at the boundaries (Requirement 4.5, 6.1, 6.3, 7.3).
func TestProperty_EnterableMatchesSpec(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		status := rapid.SampledFrom([]string{
			models.SessionStatusDraft, models.SessionStatusTerjadwal, models.SessionStatusAktif,
			models.SessionStatusSelesai, models.SessionStatusDibatalkan,
		}).Draw(rt, "status")
		mulai := genTime().Draw(rt, "mulai")
		selesai := genTimeAfter(mulai).Draw(rt, "selesai")
		now := genTime().Draw(rt, "now")

		got := enterable(status, mulai, selesai, now)
		active := status == models.SessionStatusTerjadwal || status == models.SessionStatusAktif
		inWindow := !now.Before(mulai) && !now.After(selesai)
		if got != (active && inWindow) {
			rt.Fatalf("enterable(%s, now=%v, window=[%v,%v]) = %v, want %v",
				status, now, mulai, selesai, got, active && inWindow)
		}
	})
}

// Property 6: remaining time is always non-negative, never exceeds the exam duration, and
// is zero once the session has ended. When the full duration fits before the session end,
// remaining equals the full duration (Requirement 7.5).
func TestProperty_RemainingBoundedNonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		durasiMin := rapid.IntRange(1, 1000).Draw(rt, "durasiMin")
		mulai := genTime().Draw(rt, "mulai")
		selesai := genTimeAfter(mulai).Draw(rt, "selesai")
		now := genTime().Draw(rt, "now")

		got := remainingSeconds(durasiMin, selesai, now)

		if got < 0 {
			rt.Fatalf("remaining must be non-negative, got %d", got)
		}
		if got > durasiMin*60 {
			rt.Fatalf("remaining %d exceeds duration %d", got, durasiMin*60)
		}
		if now.After(selesai) && got != 0 {
			rt.Fatalf("past session end but remaining = %d", got)
		}
		sessionLeft := selesai.Sub(now)
		if sessionLeft > 0 && time.Duration(durasiMin)*time.Minute < sessionLeft && got != durasiMin*60 {
			rt.Fatalf("full duration should apply (session has room), got %d want %d", got, durasiMin*60)
		}
	})
}

// Property 8: window overlap is symmetric and never reports true for disjoint windows
// (one entirely before or after the other). Backs the token-overlap enforcement
// (Requirement 4.4).
func TestProperty_OverlapSymmetricAndDisjoint(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		aMulai := genTime().Draw(rt, "aMulai")
		aSelesai := genTimeAfter(aMulai).Draw(rt, "aSelesai")
		bMulai := genTime().Draw(rt, "bMulai")
		bSelesai := genTimeAfter(bMulai).Draw(rt, "bSelesai")

		ab := windowsOverlap(aMulai, aSelesai, bMulai, bSelesai)
		ba := windowsOverlap(bMulai, bSelesai, aMulai, aSelesai)
		if ab != ba {
			rt.Fatalf("overlap must be symmetric: ab=%v ba=%v", ab, ba)
		}
		// b entirely before a (bSelesai <= aMulai) -> never overlaps.
		if !bSelesai.After(aMulai) && ab {
			rt.Fatalf("disjoint (b before a) must not overlap: a=[%v,%v] b=[%v,%v]", aMulai, aSelesai, bMulai, bSelesai)
		}
		// b entirely after a (bMulai >= aSelesai) -> never overlaps.
		if !bMulai.Before(aSelesai) && ab {
			rt.Fatalf("disjoint (b after a) must not overlap: a=[%v,%v] b=[%v,%v]", aMulai, aSelesai, bMulai, bSelesai)
		}
	})
}
