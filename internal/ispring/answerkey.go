package ispring

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Answer-key status values stored in soal_package.answer_key_status (audit C1, D1).
const (
	AnswerKeyFull    = "full"    // every scored question can be graded by the server
	AnswerKeyPartial = "partial" // some scored questions fall back to the client-reported score
	AnswerKeyNone    = "none"    // no usable key (legacy package or extraction failed)
)

// AnswerKeyVersion is the schema version of the serialized AnswerKey JSON.
const AnswerKeyVersion = 1

// Key question types the server can grade (audit C1). Names match the `tp` field of the
// iSpring player data.
const (
	KeyTypeMultipleChoice   = "MultipleChoice"
	KeyTypeTrueFalse        = "TrueFalse"
	KeyTypeMultipleResponse = "MultipleResponse"
	KeyTypeMatching         = "Matching"
	KeyTypeSequence         = "Sequence"
)

// ErrNoAnswerKeyData is returned when index.html has no `var data = "<base64>"` blob.
var ErrNoAnswerKeyData = errors.New("ispring: index.html has no player data blob")

// AnswerKey is the server-side answer key extracted from an iSpring package's index.html.
// It is never sent to clients (audit C1, D1).
type AnswerKey struct {
	Version       int           `json:"version"`
	Status        string        `json:"status"`
	MaxScore      float64       `json:"max_score"`
	MaxScoreKnown bool          `json:"max_score_known"` // false when some question's points are unknown
	Groups        []KeyGroup    `json:"groups"`
	Questions     []KeyQuestion `json:"questions"`
}

// KeyGroup is a question group (slide pool) of the quiz.
type KeyGroup struct {
	Title       string   `json:"title"`
	Selection   string   `json:"selection"`    // "randomSelection" | "allQuestions" | ...
	RandomCount int      `json:"random_count"` // questions drawn when Selection is randomSelection
	QuestionIDs []string `json:"question_ids"`
}

// KeyQuestion is the key of a single scored question.
//   - MultipleChoice/TrueFalse/MultipleResponse: Choices = all option texts, Correct = correct ones.
//   - Matching: Pairs = premise -> response.
//   - Sequence: Correct = items in the correct order.
type KeyQuestion struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Text     string      `json:"text"`
	Points   float64     `json:"points"`
	Partial  bool        `json:"partial"` // iSpring `ap`: partial credit allowed
	Correct  []string    `json:"correct,omitempty"`
	Choices  []string    `json:"choices,omitempty"`
	Pairs    [][2]string `json:"pairs,omitempty"`
	Gradable bool        `json:"gradable"`
}

var playerDataRe = regexp.MustCompile(`var data = "([A-Za-z0-9+/=]+)"`)

// ExtractAnswerKey decodes the iSpring player data (`var data = "<base64 JSON>"`) embedded in
// an exported index.html and builds the answer key. Question groups live at d.sl.g[], their
// questions at g.S[] (audit C1, plan item 2).
func ExtractAnswerKey(indexHTML []byte) (*AnswerKey, error) {
	m := playerDataRe.FindSubmatch(indexHTML)
	if m == nil {
		return nil, ErrNoAnswerKeyData
	}
	raw, err := base64.StdEncoding.DecodeString(string(m[1]))
	if err != nil {
		return nil, fmt.Errorf("ispring: decode player data: %w", err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("ispring: parse player data: %w", err)
	}
	groups := jArr(jObj(jObj(root, "d"), "sl"), "g")
	if groups == nil {
		return nil, fmt.Errorf("ispring: player data has no question groups (d.sl.g)")
	}

	key := &AnswerKey{Version: AnswerKeyVersion, MaxScoreKnown: true}
	gradable := 0
	for _, g := range groups {
		gm, _ := g.(map[string]any)
		sel := jObj(gm, "s")
		group := KeyGroup{
			Title:       clean(jStr(gm, "T")),
			Selection:   jStr(sel, "st"),
			RandomCount: int(jNum(sel, "rs")),
		}
		var points []float64
		for _, s := range jArr(gm, "S") {
			sm, _ := s.(map[string]any)
			q, scored, known := buildKeyQuestion(sm)
			if !scored {
				continue // info slides / surveys carry no points
			}
			if !known {
				key.MaxScoreKnown = false
			}
			if q.Gradable {
				gradable++
			}
			group.QuestionIDs = append(group.QuestionIDs, q.ID)
			key.Questions = append(key.Questions, q)
			points = append(points, q.Points)
		}
		key.MaxScore += groupMaxScore(group, points)
		key.Groups = append(key.Groups, group)
	}

	switch {
	case gradable == 0:
		key.Status = AnswerKeyNone
	case gradable == len(key.Questions):
		key.Status = AnswerKeyFull
	default:
		key.Status = AnswerKeyPartial
	}
	return key, nil
}

// groupMaxScore implements D4: a randomSelection group drawing rs < len(S) questions is
// worth the sum of its rs smallest point values; any other group is worth all its points.
func groupMaxScore(g KeyGroup, points []float64) float64 {
	total := 0.0
	if g.Selection == "randomSelection" && g.RandomCount > 0 && g.RandomCount < len(points) {
		sorted := append([]float64(nil), points...)
		sort.Float64s(sorted)
		for _, p := range sorted[:g.RandomCount] {
			total += p
		}
		return total
	}
	for _, p := range points {
		total += p
	}
	return total
}

// buildKeyQuestion converts one g.S[] entry. scored is false for entries without an
// evaluation block (s.e); known is false when points are not per-question.
func buildKeyQuestion(sm map[string]any) (q KeyQuestion, scored bool, known bool) {
	settings := jObj(sm, "s")
	eval := jObj(settings, "e")
	if eval == nil {
		return q, false, false
	}
	q = KeyQuestion{
		ID:      strings.TrimSpace(jStr(sm, "i")),
		Type:    jStr(sm, "tp"),
		Text:    richText(jObj(sm, "D")),
		Points:  jNum(eval, "pt"),
		Partial: jBool(settings, "ap"),
	}
	// Only per-question scoring has a known point value (`byAnswer` etc. would not).
	known = jStr(eval, "t") == "byQuestion"
	content := jObj(sm, "C")

	switch q.Type {
	case KeyTypeMultipleChoice, KeyTypeTrueFalse, KeyTypeMultipleResponse:
		for _, c := range jArr(content, "chs") {
			cm, _ := c.(map[string]any)
			text := richText(jObj(cm, "t"))
			q.Choices = append(q.Choices, text)
			if jBool(cm, "c") {
				q.Correct = append(q.Correct, text)
			}
		}
		q.Gradable = len(q.Correct) > 0 && distinctNonEmpty(q.Choices)
	case KeyTypeMatching:
		var premises []string
		for _, p := range jArr(content, "m") {
			pm, _ := p.(map[string]any)
			premise := richText(jObj(jObj(pm, "p"), "t"))
			response := richText(jObj(jObj(pm, "r"), "t"))
			q.Pairs = append(q.Pairs, [2]string{premise, response})
			premises = append(premises, premise)
			if response == "" {
				premises = append(premises, "") // force non-gradable
			}
		}
		q.Gradable = len(q.Pairs) > 0 && distinctNonEmpty(premises)
	case KeyTypeSequence:
		for _, c := range jArr(content, "chs") {
			cm, _ := c.(map[string]any)
			q.Correct = append(q.Correct, richText(jObj(cm, "t")))
		}
		q.Gradable = len(q.Correct) > 0 && distinctNonEmpty(q.Correct)
	}
	if !known || q.ID == "" {
		q.Gradable = false
	}
	return q, true, known
}

// NormalizeText is the comparison form of an answer text: lowercase with collapsed
// whitespace (D2: options are shuffled, so grading matches by text, not index).
func NormalizeText(s string) string {
	return strings.ToLower(clean(s))
}

// distinctNonEmpty reports whether every value is non-empty and unique after
// normalisation; otherwise text matching would be ambiguous.
func distinctNonEmpty(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		n := NormalizeText(v)
		if n == "" || seen[n] {
			return false
		}
		seen[n] = true
	}
	return true
}

// richText joins the plain-text paragraphs (`d`) of an iSpring rich-text node.
func richText(node map[string]any) string {
	var parts []string
	for _, p := range jArr(node, "d") {
		if s, ok := p.(string); ok {
			parts = append(parts, s)
		}
	}
	return clean(strings.Join(parts, " "))
}

func jObj(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	return v
}

func jArr(m map[string]any, k string) []any {
	v, _ := m[k].([]any)
	return v
}

func jStr(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

func jNum(m map[string]any, k string) float64 {
	v, _ := m[k].(float64)
	return v
}

func jBool(m map[string]any, k string) bool {
	v, _ := m[k].(bool)
	return v
}
