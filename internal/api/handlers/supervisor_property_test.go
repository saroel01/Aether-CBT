package handlers

// Feature: codebase-bug-sweep, Property 1
//
// **Property 1: Bug Condition** - C7 satu baris per peserta, skor dari sesi yang diminta.
//
// Klausa 1.7 (F): fetchRoomStatus joins hasil_tes with `LEFT JOIN hasil_tes ht ON
// p.id = ht.peserta_id AND ht.tenant_id = p.tenant_id` — a predicate that says nothing about
// which exam the result belongs to. Two independent defects follow from that single line:
//
//  1. Cardinality: hasil_tes holds one row per submitted exam, so a peserta who has sat three
//     exams multiplies into three output rows. The function's own doc comment claims "the
//     result stays one row per student", and the supervisor UI keys on that claim.
//  2. Scoping: every cek_login subquery in the same statement IS session-scoped when
//     session_id is supplied, but the hasil_tes join is not. The skor/status/waktu_selesai a
//     supervisor sees for the session they asked about can therefore come from a completely
//     different exam.
//
// Klausa 2.7 (F'): exactly one row per peserta, and when session_id is given the result
// columns come from that session or are empty.
//
// Historical rows: migration 031 added hasil_tes.exam_session_id via ALTER TABLE ADD COLUMN,
// so pre-031 rows carry NULL. This property treats them explicitly — a NULL-attributed row
// belongs to no known session, so it is NOT a candidate for a session-scoped request (folding
// it in would reinstate exactly the leak clause 2.7 closes), while an unscoped request
// (session_id absent, the legacy mapel-based dashboard) still considers it.
//
// Validates: Requirements 2.7, 3.8

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/testutil"
)

const roomStatusTimeLayout = "2006-01-02 15:04:05"

// hasilRow is one generated hasil_tes row. sessionID 0 models a historical row whose
// exam_session_id is NULL.
type hasilRow struct {
	sessionID    int
	skor         float64
	status       string
	waktuSelesai string
}

// seedRoomStatusSkeleton creates the fixed master data the property varies participants and
// results against: one tenant, one kelas, one ruang, and three exam_sessions of one exam.
func seedRoomStatusSkeleton(t *testing.T, database *sql.DB) {
	t.Helper()
	testutil.SeedTenant(t, database, 1, "default", "Default School")
	testutil.SeedKelas(t, database, 1, 1, "XII IPA 1")
	testutil.SeedRuang(t, database, 1, 1, "Ruang A", "ruang_a")
	testutil.SeedMapel(t, database, 1, 1, "Kimia", "KIM")
	testutil.SeedExam(t, database, 1, 1, 1, nil)
	for sid := 1; sid <= 3; sid++ {
		status := "draft"
		if sid == 1 {
			status = "aktif"
		}
		testutil.SeedExamSession(t, database, sid, 1, 1,
			fmt.Sprintf("2026-06-0%d 08:00:00", sid),
			fmt.Sprintf("2026-06-0%d 10:00:00", sid),
			fmt.Sprintf("TOK-%d", sid), status)
	}
}

func TestRoomStatusCardinalityAndSessionScoping(t *testing.T) {
	database, cleanup := testutil.NewMigratedDB(t)
	defer cleanup()
	db.DB = database
	seedRoomStatusSkeleton(t, database)

	rapid.Check(t, func(rt *rapid.T) {
		// All fixture mutations for one iteration go through a single transaction and one
		// commit. Each standalone Exec would otherwise cost a WAL fsync, and at ~19 writes ×
		// 100 iterations that fixture cost dominated the whole run.
		tx, err := database.Begin()
		if err != nil {
			rt.Fatalf("begin fixture tx: %v", err)
		}
		defer tx.Rollback() //nolint:errcheck

		// Each iteration starts from the fixed skeleton with all per-attempt data cleared.
		for _, table := range []string{"hasil_tes", "cek_login", "peserta"} {
			if _, err := tx.Exec("DELETE FROM " + table); err != nil {
				rt.Fatalf("clear %s: %v", table, err)
			}
		}

		numParticipants := rapid.IntRange(1, 4).Draw(rt, "num_participants")
		// requestedSession 0 means "no session_id query parameter".
		requestedSession := rapid.SampledFrom([]int{0, 1, 2, 3}).Draw(rt, "requested_session")

		expected := make(map[int][]hasilRow, numParticipants)
		base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
		var skorSeq float64

		for p := 1; p <= numParticipants; p++ {
			if _, err := tx.Exec(`INSERT INTO peserta (id, tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id) VALUES (?, 1, ?, 'hash', ?, 1, 1)`,
				p, fmt.Sprintf("2026%03d", p), fmt.Sprintf("Siswa %d", p)); err != nil {
				rt.Fatalf("seed peserta %d: %v", p, err)
			}

			hasilCount := rapid.IntRange(0, 3).Draw(rt, fmt.Sprintf("hasil_count_p%d", p))
			for h := 0; h < hasilCount; h++ {
				// 0 = historical row (exam_session_id NULL); 1..3 = attributed to that session.
				sid := rapid.SampledFrom([]int{0, 1, 2, 3}).Draw(rt, fmt.Sprintf("hasil_session_p%d_%d", p, h))
				status := rapid.SampledFrom([]string{"in_progress", "submitted"}).Draw(rt, fmt.Sprintf("hasil_status_p%d_%d", p, h))
				skorSeq++ // unique per row, so a returned skor identifies which row it came from
				row := hasilRow{
					sessionID:    sid,
					skor:         skorSeq,
					status:       status,
					waktuSelesai: base.Add(time.Duration(skorSeq) * time.Minute).Format(roomStatusTimeLayout),
				}
				insertHasil(rt, tx, p, row)
				expected[p] = append(expected[p], row)
			}
		}

		if err := tx.Commit(); err != nil {
			rt.Fatalf("commit fixture tx: %v", err)
		}

		list, err := fetchRoomStatus(1, 1, requestedSession)
		if err != nil {
			rt.Fatalf("fetchRoomStatus(tenant=1, ruang=1, session=%d): %v", requestedSession, err)
		}

		// (a) Cardinality: exactly one output row per participant, no duplicates, none missing.
		if len(list) != numParticipants {
			rt.Fatalf("fetchRoomStatus returned %d rows for %d participants (session=%d); hasil_tes rows per participant: %s",
				len(list), numParticipants, requestedSession, describeExpected(expected, numParticipants))
		}
		seen := make(map[int]int, len(list))
		for _, s := range list {
			seen[s.ID]++
		}
		for p := 1; p <= numParticipants; p++ {
			if seen[p] != 1 {
				rt.Fatalf("participant %d appears %d times, want exactly 1 (session=%d)", p, seen[p], requestedSession)
			}
		}

		// (b) Scoping: the result columns come from the requested session, or are empty.
		for _, s := range list {
			candidates := scopedCandidates(expected[s.ID], requestedSession)
			if len(candidates) == 0 {
				if s.Skor != nil || s.HasilStatus != nil || s.WaktuSelesai != nil {
					rt.Fatalf("participant %d has no hasil_tes for session=%d but got skor=%v status=%v waktu_selesai=%v (its rows: %+v)",
						s.ID, requestedSession, deref(s.Skor), derefStr(s.HasilStatus), s.WaktuSelesai, expected[s.ID])
				}
				continue
			}
			if s.Skor == nil {
				rt.Fatalf("participant %d has %d hasil_tes row(s) in scope (session=%d) but skor is empty", s.ID, len(candidates), requestedSession)
			}
			match, ok := findBySkor(candidates, *s.Skor)
			if !ok {
				rt.Fatalf("participant %d skor=%v does not belong to session=%d; in-scope rows: %+v (all rows: %+v)",
					s.ID, *s.Skor, requestedSession, candidates, expected[s.ID])
			}
			if derefStr(s.HasilStatus) != match.status {
				rt.Fatalf("participant %d status = %q, want %q (row %+v)", s.ID, derefStr(s.HasilStatus), match.status, match)
			}
			if s.WaktuSelesai == nil || s.WaktuSelesai.UTC().Format(roomStatusTimeLayout) != match.waktuSelesai {
				rt.Fatalf("participant %d waktu_selesai = %v, want %s (row %+v)", s.ID, s.WaktuSelesai, match.waktuSelesai, match)
			}
		}
	})
}

// scopedCandidates returns the hasil_tes rows that may legitimately supply the result columns
// for a request. A session-scoped request admits only rows attributed to that session;
// an unscoped request (sessionID == 0) admits only rows from the room's active session (session 1).
// Historical rows (NULL-attributed) and inactive session rows are not candidates.
func scopedCandidates(rows []hasilRow, requestedSession int) []hasilRow {
	targetSession := requestedSession
	if requestedSession == 0 {
		targetSession = 1 // active session of Ruang A
	}
	var out []hasilRow
	for _, r := range rows {
		if r.sessionID == targetSession {
			out = append(out, r)
		}
	}
	return out
}

func findBySkor(rows []hasilRow, skor float64) (hasilRow, bool) {
	for _, r := range rows {
		if r.skor == skor {
			return r, true
		}
	}
	return hasilRow{}, false
}

func insertHasil(rt *rapid.T, tx *sql.Tx, pesertaID int, row hasilRow) {
	var sessionArg any
	if row.sessionID > 0 {
		sessionArg = row.sessionID
	}
	if _, err := tx.Exec(`
		INSERT INTO hasil_tes (tenant_id, peserta_id, mapel_id, exam_session_id, skor, skor_maks, status, waktu_selesai)
		VALUES (1, ?, 1, ?, ?, 100, ?, ?)`,
		pesertaID, sessionArg, row.skor, row.status, row.waktuSelesai,
	); err != nil {
		rt.Fatalf("insert hasil_tes for peserta %d: %v", pesertaID, err)
	}
}

func describeExpected(expected map[int][]hasilRow, n int) string {
	out := ""
	for p := 1; p <= n; p++ {
		out += fmt.Sprintf("p%d=%d ", p, len(expected[p]))
	}
	return out
}

func deref(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
