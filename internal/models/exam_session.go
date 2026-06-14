package models

import "time"

// ExamSession status values, matching the exam_session.status CHECK constraint.
const (
	SessionStatusDraft      = "draft"
	SessionStatusTerjadwal  = "terjadwal"
	SessionStatusAktif      = "aktif"
	SessionStatusSelesai    = "selesai"
	SessionStatusDibatalkan = "dibatalkan"
)

// ExamSession is a scheduled wave of an Exam: a time window [WaktuMulai, WaktuSelesai],
// a unique per-session Token, and the set of classes/rooms whose participants are
// eligible (Requirement 4.1).
type ExamSession struct {
	ID           int        `json:"id"`
	TenantID     int        `json:"tenant_id"`
	ExamID       int        `json:"exam_id"`
	Nama         *string    `json:"nama,omitempty"`
	WaktuMulai   time.Time  `json:"waktu_mulai"`
	WaktuSelesai time.Time  `json:"waktu_selesai"`
	Token        string     `json:"token"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// ExamSessionKelas links a session to a class whose participants are eligible.
type ExamSessionKelas struct {
	ID        int `json:"id"`
	SessionID int `json:"session_id"`
	KelasID   int `json:"kelas_id"`
}

// ExamSessionRuang links a session to a room; when rooms are set, only participants in
// those rooms are eligible (Requirement 5.2).
type ExamSessionRuang struct {
	ID        int `json:"id"`
	SessionID int `json:"session_id"`
	RuangID   int `json:"ruang_id"`
}
