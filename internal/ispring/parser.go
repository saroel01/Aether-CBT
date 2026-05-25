package ispring

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Report struct {
	Version        string
	Summary        *Summary
	PassingPercent *float64
	Questions      []Question
}

type Summary struct {
	Passed          bool
	Percent         float64
	FinishTimestamp string
}

type Question struct {
	ID                string
	Type              string
	Text              string
	Status            string
	EvaluationEnabled bool
	AwardedPoints     float64
	MaxPoints         float64
	MaxAttempts       int
	UsedAttempts      int
	UserAnswer        string
	CorrectAnswer     string
}

type quizReportXML struct {
	XMLName      xml.Name        `xml:"quizReport"`
	Version      string          `xml:"version,attr"`
	QuizSettings quizSettingsXML `xml:"quizSettings"`
	Summary      *summaryXML     `xml:"summary"`
	Questions    questionsXML    `xml:"questions"`
}

type quizSettingsXML struct {
	PassingPercent string `xml:"passingPercent"`
}

type summaryXML struct {
	Passed          string `xml:"passed,attr"`
	Percent         string `xml:"percent,attr"`
	FinishTimestamp string `xml:"finishTimestamp,attr"`
}

type questionsXML struct {
	List []rawQuestionXML `xml:",any"`
}

type rawQuestionXML struct {
	XMLName           xml.Name      `xml:""`
	ID                string        `xml:"id,attr"`
	Status            string        `xml:"status,attr"`
	EvaluationEnabled string        `xml:"evaluationEnabled,attr"`
	AwardedPoints     string        `xml:"awardedPoints,attr"`
	MaxPoints         string        `xml:"maxPoints,attr"`
	MaxAttempts       string        `xml:"maxAttempts,attr"`
	UsedAttempts      string        `xml:"usedAttempts,attr"`
	UserAnswerAttr    string        `xml:"userAnswer,attr"`
	Direction         richTextXML   `xml:"direction"`
	Answers           answersXML    `xml:"answers"`
	UserAnswerNode    userAnswerXML `xml:"userAnswer"`
	Premises          []richTextXML `xml:"premises>premise"`
	Responses         []richTextXML `xml:"responses>response"`
	Matches           []matchXML    `xml:"matches>match"`
	Details           detailsXML    `xml:"details"`
	AcceptableAnswers []string      `xml:"acceptableAnswers>answer"`
	Objects           []richTextXML `xml:"objects>object"`
	Destinations      []richTextXML `xml:"destinations>destination"`
}

type answersXML struct {
	CorrectAnswerIndex string      `xml:"correctAnswerIndex,attr"`
	UserAnswerIndex    string      `xml:"userAnswerIndex,attr"`
	List               []answerXML `xml:"answer"`
}

type answerXML struct {
	Text                richTextXML `xml:"text"`
	CharData            string      `xml:",chardata"`
	Correct             string      `xml:"correct,attr"`
	Selected            string      `xml:"selected,attr"`
	CustomAnswer        string      `xml:"customAnswer,attr"`
	OriginalIndex       string      `xml:"originalIndex,attr"`
	UserDefinedPosition string      `xml:"userDefinedPosition,attr"`
}

type userAnswerXML struct {
	Text    string     `xml:",chardata"`
	Matches []matchXML `xml:"match"`
}

type matchXML struct {
	PremiseIndex     string `xml:"premiseIndex,attr"`
	ResponseIndex    string `xml:"responseIndex,attr"`
	ObjectIndex      string `xml:"objectIndex,attr"`
	DestinationIndex string `xml:"destinationIndex,attr"`
	LabelIndex       string `xml:"labelIndex,attr"`
	StatementIndex   string `xml:"statementIndex,attr"`
}

type detailsXML struct {
	Blanks []blankXML `xml:"blank"`
	Words  []wordXML  `xml:"word"`
}

type blankXML struct {
	UserAnswer         string   `xml:"userAnswer,attr"`
	UserAnswerIndex    string   `xml:"userAnswerIndex,attr"`
	CorrectAnswerIndex string   `xml:"correctAnswerIndex,attr"`
	Answers            []string `xml:"answer"`
}

type wordXML struct {
	Value      string `xml:",chardata"`
	UserAnswer string `xml:"userAnswer,attr"`
	Correct    string `xml:"correct,attr"`
}

type richTextXML struct {
	Text string
}

func (r *richTextXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var parts []string
	var walk func(xml.StartElement) error
	walk = func(current xml.StartElement) error {
		for {
			tok, err := d.Token()
			if err != nil {
				return err
			}
			switch t := tok.(type) {
			case xml.CharData:
				text := strings.TrimSpace(string(t))
				if text != "" {
					parts = append(parts, text)
				}
			case xml.StartElement:
				if t.Name.Local == "picture" {
					for _, attr := range t.Attr {
						if attr.Name.Local == "altText" && strings.TrimSpace(attr.Value) != "" {
							parts = append(parts, "("+strings.TrimSpace(attr.Value)+")")
						}
					}
				}
				if err := walk(t); err != nil {
					return err
				}
			case xml.EndElement:
				if t.Name.Local == current.Name.Local {
					return nil
				}
			}
		}
	}
	if err := walk(start); err != nil {
		return err
	}
	r.Text = strings.Join(parts, " ")
	return nil
}

func ParseDetailedResults(detailXML string) (*Report, error) {
	detailXML = strings.TrimSpace(detailXML)
	if detailXML == "" {
		return &Report{}, nil
	}

	var parsed quizReportXML
	if err := xml.Unmarshal([]byte(detailXML), &parsed); err != nil {
		return nil, fmt.Errorf("parse iSpring detail XML: %w", err)
	}
	if parsed.XMLName.Local != "quizReport" {
		return nil, fmt.Errorf("unsupported iSpring detail XML root %q", parsed.XMLName.Local)
	}

	report := &Report{
		Version:   parsed.Version,
		Summary:   parseSummary(parsed.Summary),
		Questions: make([]Question, 0, len(parsed.Questions.List)),
	}
	if passingPercent, ok := parseFloat(parsed.QuizSettings.PassingPercent); ok {
		report.PassingPercent = &passingPercent
	}

	for _, raw := range parsed.Questions.List {
		report.Questions = append(report.Questions, parseQuestion(raw))
	}

	return report, nil
}

func parseSummary(s *summaryXML) *Summary {
	if s == nil {
		return nil
	}
	percent, _ := parseFloat(s.Percent)
	return &Summary{
		Passed:          parseBool(s.Passed),
		Percent:         percent,
		FinishTimestamp: strings.TrimSpace(s.FinishTimestamp),
	}
}

func parseQuestion(raw rawQuestionXML) Question {
	q := Question{
		ID:     strings.TrimSpace(raw.ID),
		Type:   raw.XMLName.Local,
		Text:   clean(raw.Direction.Text),
		Status: strings.TrimSpace(raw.Status),
	}
	if strings.TrimSpace(raw.EvaluationEnabled) == "" {
		q.EvaluationEnabled = defaultEvaluationEnabled(q.Type)
	} else {
		q.EvaluationEnabled = parseBool(raw.EvaluationEnabled)
	}
	q.AwardedPoints, _ = parseFloat(raw.AwardedPoints)
	q.MaxPoints, _ = parseFloat(raw.MaxPoints)
	q.MaxAttempts, _ = parseInt(raw.MaxAttempts)
	q.UsedAttempts, _ = parseInt(raw.UsedAttempts)
	q.UserAnswer, q.CorrectAnswer = resolveAnswers(raw)
	return q
}

func defaultEvaluationEnabled(questionType string) bool {
	switch questionType {
	case "yesNoQuestion",
		"pickOneQuestion",
		"pickManyQuestion",
		"shortAnswerQuestion",
		"rankingQuestion",
		"numericSurveyQuestion",
		"matchingSurveyQuestion",
		"whichWordQuestion",
		"likertScaleQuestion",
		"multipleChoiceTextSurveyQuestion",
		"fillInTheBlankSurveyQuestion",
		"essayQuestion":
		return false
	default:
		return true
	}
}

func resolveAnswers(raw rawQuestionXML) (string, string) {
	switch raw.XMLName.Local {
	case "multipleChoiceQuestion", "trueFalseQuestion", "pickOneQuestion", "yesNoQuestion":
		return answerByIndex(raw.Answers.List, raw.Answers.UserAnswerIndex), answerByIndex(raw.Answers.List, raw.Answers.CorrectAnswerIndex)
	case "multipleResponseQuestion", "pickManyQuestion":
		return selectedAnswers(raw.Answers.List), correctAnswers(raw.Answers.List)
	case "matchingQuestion", "matchingSurveyQuestion":
		return matchPairs(raw.UserAnswerNode.Matches, raw.Premises, raw.Responses, matchModePremiseResponse), matchPairs(raw.Matches, raw.Premises, raw.Responses, matchModePremiseResponse)
	case "sequenceQuestion", "rankingQuestion":
		return sequenceUserAnswer(raw.Answers.List), sequenceCorrectAnswer(raw.Answers.List)
	case "typeInQuestion", "shortAnswerQuestion":
		return clean(raw.UserAnswerAttr), strings.Join(cleanStrings(raw.AcceptableAnswers), "; ")
	case "fillInTheBlankQuestion", "fillInTheBlankQuestionEx", "fillInTheBlankSurveyQuestion":
		return blanksUserAnswer(raw.Details.Blanks), blanksCorrectAnswer(raw.Details.Blanks)
	case "essayQuestion":
		return clean(raw.UserAnswerNode.Text), "Perlu Penilaian Manual"
	case "multipleChoiceTextQuestion", "multipleChoiceTextSurveyQuestion":
		return blanksChoiceUserAnswer(raw.Details.Blanks), blanksChoiceCorrectAnswer(raw.Details.Blanks)
	case "wordBankQuestion", "whichWordQuestion":
		return wordsUserAnswer(raw.Details.Words), wordsCorrectAnswer(raw.Details.Words)
	case "dndQuestion":
		return matchPairs(raw.UserAnswerNode.Matches, raw.Objects, raw.Destinations, matchModeObjectDestination), matchPairs(raw.Matches, raw.Objects, raw.Destinations, matchModeObjectDestination)
	case "numericQuestion", "numericSurveyQuestion":
		return clean(raw.UserAnswerAttr), numericCorrectAnswer(raw)
	default:
		if raw.UserAnswerAttr != "" {
			return clean(raw.UserAnswerAttr), ""
		}
		if raw.UserAnswerNode.Text != "" {
			return clean(raw.UserAnswerNode.Text), ""
		}
		return "", ""
	}
}

func answerByIndex(answers []answerXML, indexValue string) string {
	index, ok := parseInt(indexValue)
	if !ok || index < 0 || index >= len(answers) {
		return ""
	}
	return clean(answerText(answers[index]))
}

func selectedAnswers(answers []answerXML) string {
	var out []string
	for _, answer := range answers {
		if parseBool(answer.Selected) {
			out = append(out, answerText(answer))
		}
	}
	return strings.Join(cleanStrings(out), "; ")
}

func correctAnswers(answers []answerXML) string {
	var out []string
	for _, answer := range answers {
		if parseBool(answer.Correct) {
			out = append(out, answerText(answer))
		}
	}
	return strings.Join(cleanStrings(out), "; ")
}

func answerText(answer answerXML) string {
	text := clean(answer.Text.Text)
	if text == "" {
		text = clean(answer.CharData)
	}
	custom := clean(answer.CustomAnswer)
	if custom == "" {
		return text
	}
	if text == "" {
		return custom
	}
	return text + " " + custom
}

type matchMode int

const (
	matchModePremiseResponse matchMode = iota
	matchModeObjectDestination
)

func matchPairs(matches []matchXML, left []richTextXML, right []richTextXML, mode matchMode) string {
	if len(matches) == 0 {
		return ""
	}

	type pair struct {
		leftIndex  int
		rightIndex int
	}
	var pairs []pair
	for _, match := range matches {
		var leftValue, rightValue string
		if mode == matchModeObjectDestination {
			leftValue = match.ObjectIndex
			rightValue = match.DestinationIndex
		} else {
			leftValue = match.PremiseIndex
			rightValue = match.ResponseIndex
		}
		leftIndex, leftOK := parseInt(leftValue)
		rightIndex, rightOK := parseInt(rightValue)
		if leftOK && rightOK {
			pairs = append(pairs, pair{leftIndex: leftIndex, rightIndex: rightIndex})
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].leftIndex < pairs[j].leftIndex })

	var out []string
	for _, pair := range pairs {
		if pair.leftIndex >= 0 && pair.leftIndex < len(left) && pair.rightIndex >= 0 && pair.rightIndex < len(right) {
			out = append(out, clean(left[pair.leftIndex].Text)+" - "+clean(right[pair.rightIndex].Text))
		}
	}
	return strings.Join(cleanStrings(out), "; ")
}

func sequenceUserAnswer(answers []answerXML) string {
	var items []sequenceItem
	for index, answer := range answers {
		position, ok := parseInt(answer.UserDefinedPosition)
		if !ok {
			position = index
		}
		items = append(items, sequenceItem{position: position, text: answerText(answer)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].position < items[j].position })
	return numbered(items)
}

func sequenceCorrectAnswer(answers []answerXML) string {
	var items []sequenceItem
	for index, answer := range answers {
		position, ok := parseInt(answer.OriginalIndex)
		if !ok {
			position = index
		}
		items = append(items, sequenceItem{position: position, text: answerText(answer)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].position < items[j].position })
	return numbered(items)
}

type sequenceItem struct {
	position int
	text     string
}

func numbered(items []sequenceItem) string {
	var out []string
	for i, item := range items {
		out = append(out, fmt.Sprintf("%d. %s", i+1, clean(item.text)))
	}
	return strings.Join(cleanStrings(out), "; ")
}

func blanksUserAnswer(blanks []blankXML) string {
	var out []string
	for _, blank := range blanks {
		answer := clean(blank.UserAnswer)
		if answer == "" {
			answer = "______"
		}
		out = append(out, answer)
	}
	return strings.Join(out, "; ")
}

func blanksCorrectAnswer(blanks []blankXML) string {
	var out []string
	for _, blank := range blanks {
		out = append(out, strings.Join(cleanStrings(blank.Answers), ", "))
	}
	return strings.Join(cleanStrings(out), "; ")
}

func blanksChoiceUserAnswer(blanks []blankXML) string {
	var out []string
	for _, blank := range blanks {
		answer := answerStringByIndex(blank.Answers, blank.UserAnswerIndex)
		if answer == "" {
			answer = "______"
		}
		out = append(out, answer)
	}
	return strings.Join(out, "; ")
}

func blanksChoiceCorrectAnswer(blanks []blankXML) string {
	var out []string
	for _, blank := range blanks {
		out = append(out, answerStringByIndex(blank.Answers, blank.CorrectAnswerIndex))
	}
	return strings.Join(cleanStrings(out), "; ")
}

func answerStringByIndex(answers []string, indexValue string) string {
	index, ok := parseInt(indexValue)
	if !ok || index < 0 || index >= len(answers) {
		return ""
	}
	return clean(answers[index])
}

func wordsUserAnswer(words []wordXML) string {
	var out []string
	for _, word := range words {
		answer := clean(word.UserAnswer)
		if answer == "" {
			answer = "______"
		}
		out = append(out, answer)
	}
	return strings.Join(out, "; ")
}

func wordsCorrectAnswer(words []wordXML) string {
	var out []string
	for _, word := range words {
		if parseBool(word.Correct) || clean(word.Value) != "" {
			out = append(out, clean(word.Value))
		}
	}
	return strings.Join(cleanStrings(out), "; ")
}

func numericCorrectAnswer(raw rawQuestionXML) string {
	var out []string
	for _, answer := range raw.Answers.List {
		if text := answerText(answer); text != "" {
			out = append(out, text)
		}
	}
	return strings.Join(cleanStrings(out), ", ")
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if cleaned := clean(value); cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

func clean(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func parseBool(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true") || strings.TrimSpace(value) == "1"
}

func parseFloat(value string) (float64, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed, err == nil
}

func parseInt(value string) (int, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	return parsed, err == nil
}
