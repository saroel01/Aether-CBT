package ispring

import (
	"encoding/base64"
	"errors"
	"os"
	"testing"
)

const samplePackageIndex = "../../contoh_soal/INFORMATIKA X (W) (Published)/index.html"

func TestExtractAnswerKeySamplePackage(t *testing.T) {
	html, err := os.ReadFile(samplePackageIndex)
	if err != nil {
		t.Skipf("sample package not available: %v", err)
	}
	key, err := ExtractAnswerKey(html)
	if err != nil {
		t.Fatalf("ExtractAnswerKey: %v", err)
	}
	if len(key.Questions) != 60 {
		t.Fatalf("questions = %d, want 60", len(key.Questions))
	}
	if key.MaxScore != 444 || !key.MaxScoreKnown {
		t.Fatalf("max score = %v (known=%v), want 444", key.MaxScore, key.MaxScoreKnown)
	}
	if key.Status != AnswerKeyFull {
		t.Fatalf("status = %q, want full", key.Status)
	}
	if len(key.Groups) != 15 {
		t.Fatalf("groups = %d, want 15", len(key.Groups))
	}

	counts := map[string]int{}
	for _, q := range key.Questions {
		counts[q.Type]++
	}
	want := map[string]int{KeyTypeMultipleChoice: 15, KeyTypeMultipleResponse: 13, KeyTypeTrueFalse: 12, KeyTypeMatching: 10, KeyTypeSequence: 10}
	for tp, n := range want {
		if counts[tp] != n {
			t.Errorf("type %s count = %d, want %d", tp, counts[tp], n)
		}
	}

	first := key.Questions[0]
	if first.ID != "4mqyagrpm2r8-kejs8gd1glcc" || first.Type != KeyTypeMultipleChoice {
		t.Fatalf("first question = %s/%s", first.ID, first.Type)
	}
	if len(first.Correct) != 1 || first.Correct[0] != "Keyboard dan mouse" {
		t.Fatalf("first question correct = %v", first.Correct)
	}
	if first.Points != 5 || len(first.Choices) != 5 {
		t.Fatalf("first question points=%v choices=%v", first.Points, first.Choices)
	}
	for _, q := range key.Questions {
		switch q.Type {
		case KeyTypeMatching:
			if len(q.Pairs) == 0 {
				t.Errorf("matching %s has no pairs", q.ID)
			}
		case KeyTypeSequence:
			if len(q.Correct) < 2 {
				t.Errorf("sequence %s has %d items", q.ID, len(q.Correct))
			}
		}
	}
}

func TestExtractAnswerKeyNoData(t *testing.T) {
	if _, err := ExtractAnswerKey([]byte("<html><body>no quiz</body></html>")); !errors.Is(err, ErrNoAnswerKeyData) {
		t.Fatalf("err = %v, want ErrNoAnswerKeyData", err)
	}
}

func encodePlayerData(json string) []byte {
	return []byte(`<script>var data = "` + base64.StdEncoding.EncodeToString([]byte(json)) + `";</script>`)
}

func TestExtractAnswerKeyRandomSelectionAndPartial(t *testing.T) {
	// Group 1 draws 1 of 2 questions (points 5 and 8) -> worth 5 (D4). Group 2 has a
	// gradable MC (10) and an unsupported type -> status partial.
	html := encodePlayerData(`{"d":{"sl":{"g":[
	 {"T":"G1","s":{"st":"randomSelection","rs":1},"S":[
	  {"i":"q1","tp":"TrueFalse","D":{"d":["Q1"]},"s":{"e":{"t":"byQuestion","pt":5}},"C":{"chs":[{"c":true,"t":{"d":["True"]}},{"c":false,"t":{"d":["False"]}}]}},
	  {"i":"q2","tp":"TrueFalse","D":{"d":["Q2"]},"s":{"e":{"t":"byQuestion","pt":8}},"C":{"chs":[{"c":false,"t":{"d":["True"]}},{"c":true,"t":{"d":["False"]}}]}}]},
	 {"T":"G2","s":{"st":"allQuestions"},"S":[
	  {"i":"q3","tp":"MultipleChoice","D":{"d":["Q3"]},"s":{"e":{"t":"byQuestion","pt":10}},"C":{"chs":[{"c":true,"t":{"d":["A"]}},{"c":false,"t":{"d":["B"]}}]}},
	  {"i":"q4","tp":"Hotspot","D":{"d":["Q4"]},"s":{"e":{"t":"byQuestion","pt":4}}},
	  {"i":"info","tp":"InfoSlide","D":{"d":["Info"]},"s":{}}]}]}}}`)
	key, err := ExtractAnswerKey(html)
	if err != nil {
		t.Fatalf("ExtractAnswerKey: %v", err)
	}
	if len(key.Questions) != 4 {
		t.Fatalf("questions = %d, want 4 (info slide skipped)", len(key.Questions))
	}
	if key.MaxScore != 5+10+4 {
		t.Fatalf("max score = %v, want 19", key.MaxScore)
	}
	if key.Status != AnswerKeyPartial {
		t.Fatalf("status = %q, want partial", key.Status)
	}
}

func TestExtractAnswerKeyDuplicateChoicesNotGradable(t *testing.T) {
	html := encodePlayerData(`{"d":{"sl":{"g":[{"s":{"st":"allQuestions"},"S":[
	 {"i":"q1","tp":"MultipleChoice","D":{"d":["Q1"]},"s":{"e":{"t":"byQuestion","pt":5}},"C":{"chs":[{"c":true,"t":{"d":["Same"]}},{"c":false,"t":{"d":[" same "]}}]}}]}]}}}`)
	key, err := ExtractAnswerKey(html)
	if err != nil {
		t.Fatalf("ExtractAnswerKey: %v", err)
	}
	if key.Questions[0].Gradable || key.Status != AnswerKeyNone {
		t.Fatalf("duplicate choices must not be gradable: %+v status=%s", key.Questions[0], key.Status)
	}
}
