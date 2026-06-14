package models

import "time"

// Exam is a reusable exam definition (mapel + tingkat + soal package + duration + KKM +
// shuffle settings). A definition is run one or more times via ExamSession (Requirement 2.1).
type Exam struct {
	ID               int        `json:"id"`
	TenantID         int        `json:"tenant_id"`
	MapelID          int        `json:"mapel_id"`
	Tingkat          *string    `json:"tingkat,omitempty"`
	SoalPackageID    *int       `json:"soal_package_id,omitempty"` // nullable while draft
	DurasiMenit      int        `json:"durasi_menit"`
	KKM              float64    `json:"kkm"`
	ShuffleQuestions bool       `json:"shuffle_questions"`
	ShuffleAnswers   bool       `json:"shuffle_answers"`
	Nama             *string    `json:"nama,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}
