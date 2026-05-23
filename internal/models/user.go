package models

import "time"

type User struct {
	ID           int        `json:"id"`
	TenantID     int        `json:"tenant_id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	RuangID      *int       `json:"ruang_id,omitempty"`
	FullName     string     `json:"full_name,omitempty"`
	IsActive     bool       `json:"is_active"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}
