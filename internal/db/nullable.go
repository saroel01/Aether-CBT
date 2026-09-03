package db

import "database/sql"

// NullableFK maps a participant's optional master-data reference to a value that is safe to
// write into a FOREIGN KEY column.
//
// peserta.kelas_id and peserta.ruang_id reference kelas(id) / ruang(id) (migration 009). Both
// are optional in practice: a student can be enrolled before a class or room has been
// assigned. Before codebase-bug-sweep clause 2.2 every write path expressed "not assigned"
// as the integer 0, which is not NULL and therefore violates the referential constraint —
// invisible only because the DSN never actually enabled foreign_keys (clause 1.1).
//
// NullableFK is the single place that decision now lives: any id <= 0 (absent, blank, or the
// legacy sentinel 0) becomes NULL, and any positive id is passed through unchanged so valid
// relations keep being stored exactly as before (preservation clause 3.2).
func NullableFK(id int64) sql.NullInt64 {
	if id <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: id, Valid: true}
}

// NullableFKPtr is the NullableFK equivalent for a request field decoded into a pointer, where
// a nil pointer means "the client did not supply the field at all". Both nil and a pointer to
// a non-positive value normalize to NULL, so "absent" and "supplied as 0" stop collapsing onto
// the same wrong representation.
func NullableFKPtr(id *int64) sql.NullInt64 {
	if id == nil {
		return sql.NullInt64{}
	}
	return NullableFK(*id)
}
