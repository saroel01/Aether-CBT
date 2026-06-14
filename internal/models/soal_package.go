package models

import "time"

// SoalPackage is the metadata record for an uploaded iSpring HTML5 quiz package. The
// package files live on disk under data/soal/{tenant_slug}/{package_uuid}/ (Requirement 3.6).
type SoalPackage struct {
	ID             int        `json:"id"`
	TenantID       int        `json:"tenant_id"`
	Nama           string     `json:"nama"`
	PackageUUID    string     `json:"package_uuid"`
	EntryPath      string     `json:"entry_path"`
	IspringVersion *string    `json:"ispring_version,omitempty"` // best-effort from index.html header (Req 3.6a)
	TotalSize      int64      `json:"total_size"`
	Checksum       *string    `json:"checksum,omitempty"` // sha256 of the uploaded archive (audit/dedup)
	UploadedBy     *int       `json:"uploaded_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}
