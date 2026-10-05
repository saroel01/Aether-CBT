package ispring

import (
	"fmt"
	"math"
	"strings"
)

// Score source values stored in hasil_tes.score_source (audit C1, D5).
const (
	ScoreSourceServer = "server" // every question graded from the package answer key
	ScoreSourceMixed  = "mixed"  // some questions fell back to client-reported points
	ScoreSourceClient = "client" // no usable key: the client-reported points were used
	// ScoreSourceUnmatched: a key exists but no dr question matched it by id or text, so the
	// score is 0 and the package most likely does not belong to this report (review b).
	ScoreSourceUnmatched = "unmatched"
)

// GradeResult is the server-side grading of one detail report.
type GradeResult struct {
	Score     float64
	MaxScore  float64
	Source    string
	Questions []Question // report questions with Status/AwardedPoints/MaxPoints/CorrectAnswer from the key
}

// Grade scores a parsed report against the package answer key (audit C1, D2-D5). The client's
// awardedPoints/sp are ignored for every question the key can grade; answers are matched by
// normalised text because iSpring shuffles options. A key question the server cannot grade
// keeps the client's awardedPoints clamped to [0, max] and marks the result "mixed"; a dr
// question absent from the key earns 0. MaxScore comes from the key (D4), randomSelection
// pools count at most RandomCount questions, and Score is clamped to MaxScore. A nil key
// returns the client-derived score with Source "client".
func Grade(key *AnswerKey, r *Report) GradeResult {
	if r == nil {
		r = &Report{}
	}
	if key == nil {
		score, max := DerivedScore(r)
		return GradeResult{Score: score, MaxScore: max, Source: ScoreSourceClient, Questions: r.Questions}
	}

	byID := make(map[string]*KeyQuestion, len(key.Questions))
	textCount := make(map[string]int, len(key.Questions))
	for i := range key.Questions {
		kq := &key.Questions[i]
		byID[kq.ID] = kq
		textCount[NormalizeText(kq.Text)]++
	}
	byText := make(map[string]*KeyQuestion, len(key.Questions))
	for i := range key.Questions {
		kq := &key.Questions[i]
		if t := NormalizeText(kq.Text); t != "" && textCount[t] == 1 {
			byText[t] = kq
		}
	}

	// randomSelection pools draw only RandomCount questions; key.MaxScore counts only that
	// many, so a forged dr answering the whole pool must not score more (review finding).
	poolOf := make(map[string]int)
	poolCap := make(map[int]int)
	for gi, g := range key.Groups {
		if g.Selection == "randomSelection" && g.RandomCount > 0 && g.RandomCount < len(g.QuestionIDs) {
			poolCap[gi] = g.RandomCount
			for _, id := range g.QuestionIDs {
				poolOf[id] = gi
			}
		}
	}
	poolUsed := make(map[int]int)

	res := GradeResult{MaxScore: key.MaxScore}
	seenID := make(map[string]bool, len(r.Questions))
	usedKey := make(map[string]bool, len(r.Questions))
	serverGraded, fallback, matched := 0, 0, 0
	for _, q := range r.Questions {
		// A duplicated question id in dr must not be counted twice.
		if q.ID != "" {
			if seenID[q.ID] {
				continue
			}
			seenID[q.ID] = true
		}
		kq := byID[q.ID]
		if kq == nil {
			kq = byText[NormalizeText(q.Text)]
		}
		if kq != nil {
			if usedKey[kq.ID] {
				continue // second report entry resolving to the same key question
			}
			usedKey[kq.ID] = true
			matched++
			if gi, ok := poolOf[kq.ID]; ok {
				if poolUsed[gi] >= poolCap[gi] {
					continue // beyond the questions this pool actually draws
				}
				poolUsed[gi]++
			}
		}

		if kq == nil {
			// Not in the key: the key is readable in the browser, so a fabricated question
			// must earn nothing and must not raise MaxScore (review finding). The row stays
			// in the details and the result is labelled mixed/client.
			q.AwardedPoints = 0
			q.MaxPoints = math.Max(q.MaxPoints, 0)
			fallback++
		} else if kq.Gradable {
			awarded := gradeQuestion(kq, q)
			q.AwardedPoints = awarded
			q.MaxPoints = kq.Points
			q.Status = gradeStatus(awarded, kq.Points)
			q.CorrectAnswer = keyCorrectAnswer(kq)
			serverGraded++
		} else {
			limit := q.MaxPoints
			if kq.Points > 0 {
				limit = kq.Points
			}
			q.AwardedPoints = round2(math.Min(math.Max(q.AwardedPoints, 0), math.Max(limit, 0)))
			q.MaxPoints = math.Max(limit, 0)
			fallback++
		}
		res.Score += q.AwardedPoints
		res.Questions = append(res.Questions, q)
	}

	res.MaxScore = round2(res.MaxScore)
	res.Score = round2(math.Min(res.Score, res.MaxScore))
	switch {
	case matched == 0 && len(r.Questions) > 0:
		res.Source = ScoreSourceUnmatched
	case fallback == 0:
		res.Source = ScoreSourceServer
	case serverGraded > 0:
		res.Source = ScoreSourceMixed
	default:
		res.Source = ScoreSourceClient
	}
	return res
}

// gradeQuestion applies the D3 formula to one gradable key question.
func gradeQuestion(kq *KeyQuestion, q Question) float64 {
	pt := kq.Points
	switch kq.Type {
	case KeyTypeMultipleChoice, KeyTypeTrueFalse:
		if len(q.UserChoices) == 1 && containsNorm(kq.Correct, q.UserChoices[0]) {
			return pt
		}
		return 0
	case KeyTypeMultipleResponse:
		picked := make(map[string]bool, len(q.UserChoices))
		right, wrong := 0, 0
		for _, c := range q.UserChoices {
			n := NormalizeText(c)
			if n == "" || picked[n] {
				continue
			}
			picked[n] = true
			if containsNorm(kq.Correct, c) {
				right++
			} else {
				wrong++
			}
		}
		return partialScore(kq, right-wrong, len(kq.Correct), right == len(kq.Correct) && wrong == 0)
	case KeyTypeMatching:
		want := make(map[string]string, len(kq.Pairs))
		for _, p := range kq.Pairs {
			want[NormalizeText(p[0])] = NormalizeText(p[1])
		}
		done := make(map[string]bool, len(q.UserPairs))
		right := 0
		for _, p := range q.UserPairs {
			premise := NormalizeText(p[0])
			if done[premise] {
				continue
			}
			done[premise] = true
			if r, ok := want[premise]; ok && r == NormalizeText(p[1]) {
				right++
			}
		}
		return partialScore(kq, right, len(kq.Pairs), right == len(kq.Pairs))
	case KeyTypeSequence:
		right := 0
		for i, item := range kq.Correct {
			if i < len(q.UserOrder) && NormalizeText(q.UserOrder[i]) == NormalizeText(item) {
				right++
			}
		}
		return partialScore(kq, right, len(kq.Correct), right == len(kq.Correct) && len(q.UserOrder) == len(kq.Correct))
	}
	return 0
}

// partialScore: proportional credit when the key allows it (ap=true), otherwise
// all-or-nothing (D3).
func partialScore(kq *KeyQuestion, right, total int, allCorrect bool) float64 {
	if allCorrect {
		return kq.Points
	}
	if !kq.Partial || total <= 0 || right <= 0 {
		return 0
	}
	return round2(kq.Points * float64(right) / float64(total))
}

func gradeStatus(awarded, max float64) string {
	switch {
	case awarded >= max && max > 0:
		return "correct"
	case awarded > 0:
		return "partially"
	default:
		return "incorrect"
	}
}

// keyCorrectAnswer formats the key like the parser's CorrectAnswer display strings.
func keyCorrectAnswer(kq *KeyQuestion) string {
	switch kq.Type {
	case KeyTypeMatching:
		out := make([]string, 0, len(kq.Pairs))
		for _, p := range kq.Pairs {
			out = append(out, p[0]+" - "+p[1])
		}
		return strings.Join(out, "; ")
	case KeyTypeSequence:
		out := make([]string, 0, len(kq.Correct))
		for i, item := range kq.Correct {
			out = append(out, fmt.Sprintf("%d. %s", i+1, item))
		}
		return strings.Join(out, "; ")
	default:
		return strings.Join(kq.Correct, "; ")
	}
}

func containsNorm(values []string, v string) bool {
	n := NormalizeText(v)
	for _, x := range values {
		if NormalizeText(x) == n {
			return true
		}
	}
	return false
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
