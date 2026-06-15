package ispring

import "testing"

func TestDerivedScoreSumsAwardedAndMaxPoints(t *testing.T) {
	r := &Report{
		Questions: []Question{
			{AwardedPoints: 3.33, MaxPoints: 5},
			{AwardedPoints: 2, MaxPoints: 2},
			{AwardedPoints: 0, MaxPoints: 1},
		},
	}
	score, max := DerivedScore(r)
	if score != 5.33 {
		t.Fatalf("awarded = %.2f, want 5.33", score)
	}
	if max != 8 {
		t.Fatalf("max = %.2f, want 8", max)
	}
}

func TestDerivedScoreNilReportIsZero(t *testing.T) {
	score, max := DerivedScore(nil)
	if score != 0 || max != 0 {
		t.Fatalf("nil report: (%v,%v), want (0,0)", score, max)
	}
}

func TestScoresMatchWithinTolerance(t *testing.T) {
	cases := []struct {
		client, derived, tol float64
		want                 bool
	}{
		{5.33, 5.33, 0.01, true},
		{5.3, 5.33, 0.05, true},
		{8, 5.33, 0.01, false}, // inflated client score
	}
	for _, c := range cases {
		got := ScoresConsistent(c.client, c.derived, c.tol)
		if got != c.want {
			t.Errorf("ScoresConsistent(client=%.2f,derived=%.2f,tol=%.2f)=%v, want %v",
				c.client, c.derived, c.tol, got, c.want)
		}
	}
}
