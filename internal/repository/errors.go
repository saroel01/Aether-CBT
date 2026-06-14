package repository

import "errors"

// ErrNotFound is returned by a tenant-scoped repository operation when no matching row
// belongs to the caller's tenant (Requirement 15.2). Handlers map this to HTTP 404.
var ErrNotFound = errors.New("repository: record not found")

// ErrConflict is returned when an operation is rejected due to a conflicting state
// (e.g. deleting an exam that still has scheduled/active sessions, or a token that
// overlaps another session's window). Handlers map this to HTTP 409/400 with a message.
var ErrConflict = errors.New("repository: conflicting state")

// ErrInvalidReference is returned when a create/update references an entity (e.g. a
// mapel or soal package) that does not exist in the caller's tenant. Handlers map this
// to HTTP 400 with a validation message (Requirements 2.2, 2.3).
var ErrInvalidReference = errors.New("repository: referenced entity not found in tenant")
