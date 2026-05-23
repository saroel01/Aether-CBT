package models

import "time"

type Tenant struct {
	ID        int        `json:"id"`
	Slug      string     `json:"slug"`
	Name      string     `json:"name"`
	Logo      string     `json:"logo,omitempty"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
