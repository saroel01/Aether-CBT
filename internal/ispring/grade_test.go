package ispring

import "testing"

func gradeTestKey() *AnswerKey {
	return &AnswerKey{
		Version:       AnswerKeyVersion,
		Status:        AnswerKeyFull,
		MaxScore:      40,
		MaxScoreKnown: true,
		Questions: []KeyQuestion{
			{ID: "mc", Type: KeyTypeMultipleChoice, Text: "Perangkat input?", Points: 10, Gradable: true,
				Choices: []string{"Monitor", "Keyboard dan mouse", "Printer"}, Correct: []string{"Keyboard dan mouse"}},
			{ID: "mr", Type: KeyTypeMultipleResponse, Text: "Pilih bilangan genap", Points: 10, Partial: true, Gradable: true,
				Choices: []string{"1", "2", "3", "4"}, Correct: []string{"2", "4"}},
			{ID: "mt", Type: KeyTypeMatching, Text: "Pasangkan", Points: 10, Gradable: true,
				Pairs: [][2]string{{"CPU", "Proses"}, {"RAM", "Memori"}}},
			{ID: "sq", Type: KeyTypeSequence, Text: "Urutkan", Points: 10, Partial: true, Gradable: true,
				Correct: []string{"Satu", "Dua", "Tiga", "Empat"}},
		},
	}
}

func TestGradeIgnoresClientPoints(t *testing.T) {
	r := &Report{Questions: []Question{
		// Options are shuffled and the client claims full points for a wrong answer.
		{ID: "mc", Text: "Perangkat input?", UserChoices: []string{"printer"}, AwardedPoints: 10, MaxPoints: 10},
		// Partial MR with ap=true: 2 right - 1 wrong over 2 correct -> 50%.
		{ID: "mr", UserChoices: []string{"2", "4", "1"}, AwardedPoints: 999, MaxPoints: 999},
		{ID: "mt", UserPairs: [][2]string{{"RAM", "memori"}, {"CPU", "Proses"}}},
		// Sequence with 2 of 4 positions right, ap=true -> 50%.
		{ID: "sq", UserOrder: []string{"Satu", "Dua", "Empat", "Tiga"}},
	}}
	got := Grade(gradeTestKey(), r)
	if got.Source != ScoreSourceServer {
		t.Fatalf("source = %q, want server", got.Source)
	}
	want := []float64{0, 5, 10, 5}
	for i, q := range got.Questions {
		if q.AwardedPoints != want[i] {
			t.Errorf("q %s awarded = %v, want %v", q.ID, q.AwardedPoints, want[i])
		}
	}
	if got.Score != 20 || got.MaxScore != 40 {
		t.Fatalf("score = %v/%v, want 20/40", got.Score, got.MaxScore)
	}
	if got.Questions[0].Status != "incorrect" || got.Questions[0].CorrectAnswer != "Keyboard dan mouse" {
		t.Fatalf("mc status/correct = %q/%q", got.Questions[0].Status, got.Questions[0].CorrectAnswer)
	}
}

func TestGradeCorrectAnswersAndTextMatch(t *testing.T) {
	r := &Report{Questions: []Question{
		// Unknown id but unique question text -> matched by text.
		{ID: "other-id", Text: "  perangkat   INPUT? ", UserChoices: []string{"Keyboard  dan mouse"}},
		{ID: "sq", UserOrder: []string{"Satu", "Dua", "Tiga", "Empat"}},
	}}
	got := Grade(gradeTestKey(), r)
	if got.Questions[0].AwardedPoints != 10 || got.Questions[0].Status != "correct" {
		t.Fatalf("mc = %+v", got.Questions[0])
	}
	if got.Questions[1].AwardedPoints != 10 {
		t.Fatalf("sq awarded = %v, want 10", got.Questions[1].AwardedPoints)
	}
	// Dropping mr/mt from dr does not lower the max score.
	if got.Score != 20 || got.MaxScore != 40 || got.Source != ScoreSourceServer {
		t.Fatalf("got %v/%v %s, want 20/40 server", got.Score, got.MaxScore, got.Source)
	}
}

func TestGradeAllOrNothingWithoutPartial(t *testing.T) {
	key := gradeTestKey()
	key.Questions[1].Partial = false
	got := Grade(key, &Report{Questions: []Question{{ID: "mr", UserChoices: []string{"2"}}}})
	if got.Score != 0 {
		t.Fatalf("score = %v, want 0", got.Score)
	}
}

func TestGradeDuplicateIDCountedOnce(t *testing.T) {
	q := Question{ID: "mc", UserChoices: []string{"Keyboard dan mouse"}}
	got := Grade(gradeTestKey(), &Report{Questions: []Question{q, q, q}})
	if got.Score != 10 || len(got.Questions) != 1 {
		t.Fatalf("score = %v, questions = %d, want 10 and 1", got.Score, len(got.Questions))
	}
}

func TestGradeUnknownQuestionIsMixedAndClamped(t *testing.T) {
	key := gradeTestKey()
	key.Questions[3].Gradable = false // e.g. ambiguous key entry
	r := &Report{Questions: []Question{
		{ID: "mc", UserChoices: []string{"Keyboard dan mouse"}},
		{ID: "sq", AwardedPoints: 500, MaxPoints: 500},                  // clamped to key points 10
		{ID: "essay", Text: "Jelaskan", AwardedPoints: 7, MaxPoints: 5}, // not in key: earns 0
	}}
	got := Grade(key, r)
	if got.Source != ScoreSourceMixed {
		t.Fatalf("source = %q, want mixed", got.Source)
	}
	if got.Score != 20 || got.MaxScore != 40 {
		t.Fatalf("score = %v/%v, want 20/40", got.Score, got.MaxScore)
	}
	if len(got.Questions) != 3 {
		t.Fatalf("questions = %d, want 3 (unknown row kept)", len(got.Questions))
	}
}

func TestGradeFabricatedQuestionEarnsNothing(t *testing.T) {
	r := &Report{Questions: []Question{
		{ID: "mc", UserChoices: []string{"Keyboard dan mouse"}},
		{ID: "fake", Text: "Soal palsu", AwardedPoints: 10000, MaxPoints: 10000, Status: "correct"},
	}}
	got := Grade(gradeTestKey(), r)
	if got.Score != 10 || got.MaxScore != 40 || got.Source != ScoreSourceMixed {
		t.Fatalf("got %v/%v %q, want 10/40 mixed", got.Score, got.MaxScore, got.Source)
	}
	if got.Questions[1].AwardedPoints != 0 {
		t.Fatalf("fake question awarded %v, want 0", got.Questions[1].AwardedPoints)
	}
}

// TestGradeUnmatchedWhenNoQuestionMatches (review b): a key that matches none of the dr
// questions is labelled unmatched instead of silently scoring 0 as "client".
func TestGradeUnmatchedWhenNoQuestionMatches(t *testing.T) {
	r := &Report{Questions: []Question{
		{ID: "other-1", Text: "Soal lain satu", AwardedPoints: 10, MaxPoints: 10},
		{ID: "other-2", Text: "Soal lain dua", AwardedPoints: 10, MaxPoints: 10},
	}}
	got := Grade(gradeTestKey(), r)
	if got.Source != ScoreSourceUnmatched || got.Score != 0 {
		t.Fatalf("got score %v source %q, want 0 unmatched", got.Score, got.Source)
	}
	// One real match keeps the previous labelling (mixed because the fabricated one falls back).
	r.Questions = append(r.Questions, Question{ID: "mc", UserChoices: []string{"Keyboard dan mouse"}})
	if got := Grade(gradeTestKey(), r); got.Source != ScoreSourceMixed {
		t.Fatalf("with one match: source %q, want mixed", got.Source)
	}
	// An empty report is not "unmatched".
	if got := Grade(gradeTestKey(), &Report{}); got.Source == ScoreSourceUnmatched {
		t.Fatal("empty report must not be labelled unmatched")
	}
}

func TestGradeRandomPoolCountsAtMostRandomCount(t *testing.T) {
	key := gradeTestKey()
	// Pool of mc/mr/mt drawing 1 question (worth 10) + sq: MaxScore 20.
	key.Groups = []KeyGroup{
		{Selection: "randomSelection", RandomCount: 1, QuestionIDs: []string{"mc", "mr", "mt"}},
		{Selection: "allQuestions", QuestionIDs: []string{"sq"}},
	}
	key.MaxScore = 20
	r := &Report{Questions: []Question{
		{ID: "mc", UserChoices: []string{"Keyboard dan mouse"}},
		{ID: "mr", UserChoices: []string{"2", "4"}},
		{ID: "mt", UserPairs: [][2]string{{"CPU", "Proses"}, {"RAM", "Memori"}}},
		{ID: "sq", UserOrder: []string{"Satu", "Dua", "Tiga", "Empat"}},
	}}
	got := Grade(key, r)
	if got.Score != 20 || got.MaxScore != 20 {
		t.Fatalf("score = %v/%v, want 20/20", got.Score, got.MaxScore)
	}
	if got.Score > got.MaxScore {
		t.Fatalf("score %v exceeds max %v", got.Score, got.MaxScore)
	}
}

func TestGradeNilKeyIsClient(t *testing.T) {
	got := Grade(nil, &Report{Questions: []Question{{ID: "x", AwardedPoints: 3, MaxPoints: 5}}})
	if got.Source != ScoreSourceClient || got.Score != 3 || got.MaxScore != 5 {
		t.Fatalf("got %+v", got)
	}
}
