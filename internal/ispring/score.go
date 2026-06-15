package ispring

// DerivedScore sums awarded_points and max_points across all questions in the
// parsed report. This is the server-authoritative score, independent of the
// client-supplied sp/tp form values on the webhook.
func DerivedScore(r *Report) (awarded, max float64) {
	if r == nil {
		return 0, 0
	}
	for _, q := range r.Questions {
		awarded += q.AwardedPoints
		max += q.MaxPoints
	}
	return awarded, max
}

// ScoresConsistent returns true when the client-supplied score is within tol of
// the derived score. Use an absolute tolerance (e.g. 0.01) to absorb rounding
// from iSpring's percent→points conversion.
func ScoresConsistent(clientScore, derivedScore, tol float64) bool {
	diff := clientScore - derivedScore
	if diff < 0 {
		diff = -diff
	}
	return diff <= tol
}
