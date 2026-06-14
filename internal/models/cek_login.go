package models

import "time"

// CekLogin represents a participant's active exam-session record. It links a participant
// to a scheduled ExamSession (SessionID), carries the per-attempt AttemptToken and the
// content-serving ContentToken, and tracks anti-cheat state (Locked, TabSwitchCount) and
// progress (AnsweredCount, TotalQuestions). The legacy mapel-based columns are retained
// for the transition period (Requirements 7.1, 7.2, 10.1, 10.2, 14.4).
type CekLogin struct {
	ID             int        `json:"id"`
	TenantID       int        `json:"tenant_id"`
	PesertaID      int        `json:"peserta_id"`
	RuangID        *int       `json:"ruang_id,omitempty"`
	MapelID        *int       `json:"mapel_id,omitempty"`   // legacy; session-based path uses SessionID
	SessionID      *int       `json:"session_id,omitempty"` // links to exam_session (Req 7.1)
	AttemptToken   *string    `json:"attempt_token,omitempty"`
	ContentToken   *string    `json:"content_token,omitempty"`
	Locked         bool       `json:"locked"` // server-enforced lock (Req 10.2)
	TabSwitchCount int        `json:"tab_switch_count"`
	AnsweredCount  int        `json:"answered_count"`
	TotalQuestions int        `json:"total_questions"`
	LoginTime      time.Time  `json:"login_time"`
	LastActivity   *time.Time `json:"last_activity,omitempty"`
}
