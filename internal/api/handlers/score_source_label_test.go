package handlers

import "testing"

// TestScoreSourceLabel pins the SUMBER_SKOR labels, including unmatched (review b).
func TestScoreSourceLabel(t *testing.T) {
	for in, want := range map[string]string{
		"server":    "server",
		"mixed":     "campuran",
		"unmatched": "kunci tidak cocok",
		"client":    "dilaporkan klien",
		"":          "dilaporkan klien",
	} {
		if got := scoreSourceLabel(in); got != want {
			t.Errorf("scoreSourceLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
