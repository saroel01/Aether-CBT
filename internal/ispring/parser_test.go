package ispring

import "testing"

func TestParseDetailedResultsReadsOfficialQuizReportSummaryAndQuestionAnswers(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<quizReport xmlns="http://www.ispringsolutions.com/ispring/quizbuilder/quizresults" version="9">
  <quizSettings>
    <passingPercent>70</passingPercent>
  </quizSettings>
  <summary passed="true" percent="85" finishTimestamp="2026-05-25T09:00:00Z" />
  <questions>
    <multipleChoiceQuestion id="q1" evaluationEnabled="true" maxPoints="10" awardedPoints="10" status="correct">
      <direction><text>Ibukota Indonesia?</text></direction>
      <answers correctAnswerIndex="1" userAnswerIndex="1">
        <answer><text>Bandung</text></answer>
        <answer><text>Jakarta</text></answer>
      </answers>
    </multipleChoiceQuestion>
    <essayQuestion id="q2" evaluationEnabled="false" maxPoints="20" awardedPoints="0" status="answered">
      <direction><text>Jelaskan makna gotong royong.</text></direction>
      <userAnswer>Gotong royong adalah kerja bersama untuk kepentingan bersama.</userAnswer>
    </essayQuestion>
  </questions>
  <groups />
</quizReport>`

	report, err := ParseDetailedResults(xml)
	if err != nil {
		t.Fatalf("ParseDetailedResults returned error: %v", err)
	}

	if report.Version != "9" {
		t.Fatalf("expected version 9, got %q", report.Version)
	}
	if report.Summary == nil {
		t.Fatalf("expected summary to be parsed")
	}
	if !report.Summary.Passed || report.Summary.Percent != 85 || report.Summary.FinishTimestamp != "2026-05-25T09:00:00Z" {
		t.Fatalf("summary mismatch: %+v", report.Summary)
	}
	if len(report.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(report.Questions))
	}

	mc := report.Questions[0]
	if mc.ID != "q1" || mc.Type != "multipleChoiceQuestion" || mc.Text != "Ibukota Indonesia?" {
		t.Fatalf("multiple choice identity mismatch: %+v", mc)
	}
	if mc.UserAnswer != "Jakarta" || mc.CorrectAnswer != "Jakarta" {
		t.Fatalf("multiple choice answers mismatch: user=%q correct=%q", mc.UserAnswer, mc.CorrectAnswer)
	}
	if !mc.EvaluationEnabled || mc.Status != "correct" || mc.AwardedPoints != 10 || mc.MaxPoints != 10 {
		t.Fatalf("multiple choice scoring mismatch: %+v", mc)
	}

	essay := report.Questions[1]
	if essay.ID != "q2" || essay.Type != "essayQuestion" {
		t.Fatalf("essay identity mismatch: %+v", essay)
	}
	if essay.EvaluationEnabled {
		t.Fatalf("essay should not be evaluation-enabled: %+v", essay)
	}
	if essay.UserAnswer != "Gotong royong adalah kerja bersama untuk kepentingan bersama." {
		t.Fatalf("essay user answer mismatch: %q", essay.UserAnswer)
	}
	if essay.CorrectAnswer != "Perlu Penilaian Manual" {
		t.Fatalf("essay correct answer mismatch: %q", essay.CorrectAnswer)
	}
}

func TestParseDetailedResultsHandlesRichISpringQuestionTypes(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<quizReport version="9">
  <quizSettings><passingPercent>70</passingPercent></quizSettings>
  <summary passed="false" percent="50" />
  <questions>
    <multipleResponseQuestion id="mr" evaluationEnabled="true" maxPoints="10" awardedPoints="5" status="partially">
      <direction><text>Pilih bilangan genap.</text></direction>
      <answers>
        <answer correct="true" selected="true"><text>2</text></answer>
        <answer correct="true" selected="false"><text>4</text></answer>
        <answer correct="false" selected="true"><text>5</text></answer>
      </answers>
    </multipleResponseQuestion>
    <matchingQuestion id="match" evaluationEnabled="true" maxPoints="10" awardedPoints="10" status="correct">
      <direction><text>Pasangkan negara dan ibu kota.</text></direction>
      <premises><premise>Indonesia</premise><premise>Jepang</premise></premises>
      <responses><response>Jakarta</response><response>Tokyo</response></responses>
      <matches><match premiseIndex="0" responseIndex="0" /><match premiseIndex="1" responseIndex="1" /></matches>
      <userAnswer><match premiseIndex="0" responseIndex="0" /><match premiseIndex="1" responseIndex="1" /></userAnswer>
    </matchingQuestion>
    <sequenceQuestion id="seq" evaluationEnabled="true" maxPoints="10" awardedPoints="0" status="incorrect">
      <direction><text>Urutkan angka.</text></direction>
      <answers>
        <answer originalIndex="1" userDefinedPosition="0">Dua</answer>
        <answer originalIndex="0" userDefinedPosition="1">Satu</answer>
      </answers>
    </sequenceQuestion>
    <typeInQuestion id="typein" evaluationEnabled="true" maxPoints="5" awardedPoints="5" status="correct" userAnswer="merdeka">
      <direction><text>Kata kunci proklamasi.</text></direction>
      <acceptableAnswers><answer>merdeka</answer><answer>kemerdekaan</answer></acceptableAnswers>
    </typeInQuestion>
    <fillInTheBlankQuestionEx id="fib" evaluationEnabled="true" maxPoints="5" awardedPoints="5" status="correct">
      <direction><text>Lengkapi kalimat.</text></direction>
      <details>
        <blank userAnswer="air"><answer>air</answer><answer>H2O</answer></blank>
        <blank userAnswer=""><answer>oksigen</answer></blank>
      </details>
    </fillInTheBlankQuestionEx>
  </questions>
  <groups />
</quizReport>`

	report, err := ParseDetailedResults(xml)
	if err != nil {
		t.Fatalf("ParseDetailedResults returned error: %v", err)
	}
	if len(report.Questions) != 5 {
		t.Fatalf("expected 5 questions, got %d", len(report.Questions))
	}

	assertQuestion := func(index int, userAnswer string, correctAnswer string) {
		t.Helper()
		q := report.Questions[index]
		if q.UserAnswer != userAnswer || q.CorrectAnswer != correctAnswer {
			t.Fatalf("%s answers mismatch: user=%q correct=%q", q.ID, q.UserAnswer, q.CorrectAnswer)
		}
	}

	assertQuestion(0, "2; 5", "2; 4")
	assertQuestion(1, "Indonesia - Jakarta; Jepang - Tokyo", "Indonesia - Jakarta; Jepang - Tokyo")
	assertQuestion(2, "1. Dua; 2. Satu", "1. Satu; 2. Dua")
	assertQuestion(3, "merdeka", "merdeka; kemerdekaan")
	assertQuestion(4, "air; ______", "air, H2O; oksigen")
}

func TestParseDetailedResultsUsesISpringDefaultEvaluationWhenAttributeIsMissing(t *testing.T) {
	xml := `<quizReport version="8">
  <questions>
    <multipleChoiceQuestion id="graded" maxPoints="1" awardedPoints="1" status="correct">
      <direction><text>Graded by default?</text></direction>
      <answers correctAnswerIndex="0" userAnswerIndex="0"><answer><text>Yes</text></answer></answers>
    </multipleChoiceQuestion>
    <essayQuestion id="essay" maxPoints="1" awardedPoints="0" status="answered">
      <direction><text>Essay?</text></direction>
      <userAnswer>Text</userAnswer>
    </essayQuestion>
  </questions>
</quizReport>`

	report, err := ParseDetailedResults(xml)
	if err != nil {
		t.Fatalf("ParseDetailedResults returned error: %v", err)
	}
	if len(report.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(report.Questions))
	}
	if !report.Questions[0].EvaluationEnabled {
		t.Fatalf("multipleChoiceQuestion should be graded by default")
	}
	if report.Questions[1].EvaluationEnabled {
		t.Fatalf("essayQuestion should not be graded by default")
	}
}
