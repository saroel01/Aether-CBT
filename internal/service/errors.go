package service

import "errors"

// Scheduling-service error sentinels. Handlers map these to HTTP 400/409 with a clear
// message for the admin UI (Requirement 12.6).

// ErrInvalidWindow is returned when a session window has end <= start (Requirement 4.2).
var ErrInvalidWindow = errors.New("service: session window invalid (end must be after start)")

// ErrTokenConflict is returned when a session token overlaps another session's window in
// the tenant (Requirement 4.4).
var ErrTokenConflict = errors.New("service: session token overlaps another session's window")

// ErrPackageRequired is returned when transitioning a session to terjadwal/aktif while
// the exam has no linked soal package (Requirement 2.5, 4.3).
var ErrPackageRequired = errors.New("service: exam must have a linked soal package before scheduling or activating")
