package handlers

// Task 1.1 — golden baseline for every list endpoint named by clause 1.15.
//
// Each test records the RAW response body, byte for byte, so the golden pins the JSON payload,
// the field order, and the row order all at once (clause 3.13). Nothing is normalized: the seed
// pins every timestamp instead, which keeps the artefact a faithful copy of what F returns.
//
// Recorded from the unfixed code. Task 6.29 re-runs these against F'.

import (
	"testing"
)

// TestGoldenGetAvailableMapels records both branches of student_exam.go GetAvailableMapels:
// the class-scoped query (peserta_id present, kelas_mapel join) and the all-subjects fallback.
//
// ORDERING CAVEAT: neither query has an ORDER BY. The recorded order is whatever SQLite's plan
// yields for this schema and data, not a guaranteed contract. See the ordering note in
// TestGoldenListEndpointOrderingIsNotPinnedBySQL.
func TestGoldenGetAvailableMapels(t *testing.T) {
	app, _ := newGoldenApp(t, "student")
	app.Get("/api/student/mapels", GetAvailableMapels)

	// peserta 100 is in kelas 11 -> Matematika + Bahasa Indonesia active, Kimia inactive.
	assertGolden(t, "get_available_mapels_class_scoped.json",
		getOK(t, app, "/api/student/mapels?peserta_id=100"))

	// No peserta_id -> the "all subjects in tenant" fallback.
	assertGolden(t, "get_available_mapels_all_subjects.json",
		getOK(t, app, "/api/student/mapels"))
}

// TestGoldenGetEducationalAnalysis records ispring.go GetEducationalAnalysis (GROUP BY over
// hasil_tes_detail; no ORDER BY).
func TestGoldenGetEducationalAnalysis(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/analysis/educational", GetEducationalAnalysis)

	assertGolden(t, "get_educational_analysis.json",
		getOK(t, app, "/api/analysis/educational"))
}

// TestGoldenGetClassSubjects records mapping_handler.go GetClassSubjects for a class that has
// both active and inactive mappings (kelas 11), so the golden also pins the is_active filter.
func TestGoldenGetClassSubjects(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/classes/:kelas_id/subjects", GetClassSubjects)

	assertGolden(t, "get_class_subjects_kelas11.json",
		getOK(t, app, "/api/classes/11/subjects"))
}

// TestGoldenGetStudents records student.go GetStudents. created_at is part of the payload and is
// pinned by the seed; without that pin this golden could never be reproduced.
func TestGoldenGetStudents(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/students", GetStudents)

	assertGolden(t, "get_students.json", getOK(t, app, "/api/students"))
}

// TestGoldenGetRooms records ruang.go GetRooms (also carries a pinned created_at).
func TestGoldenGetRooms(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/rooms", GetRooms)

	assertGolden(t, "get_rooms.json", getOK(t, app, "/api/rooms"))
}

// TestGoldenGetEssayAnswers records essay_grading.go GetEssayAnswers. This is the one list
// endpoint whose order IS pinned by SQL (ORDER BY p.nama_peserta ASC, htd.id ASC).
func TestGoldenGetEssayAnswers(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/essay-answers", GetEssayAnswers)

	assertGolden(t, "get_essay_answers.json", getOK(t, app, "/api/essay-answers"))
}

// TestGoldenGetItemAnalysis records item_analysis.go GetItemAnalysis, including the derived
// success_rate float and the difficulty bucket string. The seed produces one item per bucket
// boundary region (100% / 50% / 33.33%) so the classification ladder is covered.
func TestGoldenGetItemAnalysis(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/item-analysis", GetItemAnalysis)

	assertGolden(t, "get_item_analysis.json", getOK(t, app, "/api/item-analysis"))
}

// TestGoldenListEndpointOrderingIsNotPinnedBySQL documents, as an executable note for task 6.29,
// which recorded payloads have an order that SQL guarantees and which do not.
//
// For the endpoints WITHOUT an ORDER BY the golden file still captures the observed order, but a
// byte-exact match after the fix only proves "the plan did not change" — it is not a behavioural
// contract. If clause 2.15 requires rewriting one of those queries (for instance to add a
// nullable scan target), 6.29 must compare the payload as a SET of rows, not as bytes.
func TestGoldenListEndpointOrderingIsNotPinnedBySQL(t *testing.T) {
	orderPinnedBySQL := map[string]bool{
		"GetAvailableMapels (class-scoped)": false, // no ORDER BY
		"GetAvailableMapels (fallback)":     false, // no ORDER BY
		"GetEducationalAnalysis":            false, // GROUP BY only, no ORDER BY
		"GetClassSubjects":                  false, // no ORDER BY
		"GetStudents":                       false, // no ORDER BY
		"GetRooms":                          false, // no ORDER BY
		"GetItemAnalysis":                   false, // GROUP BY only, no ORDER BY
		"GetEssayAnswers":                   true,  // ORDER BY p.nama_peserta ASC, htd.id ASC
	}

	var unpinned int
	for name, pinned := range orderPinnedBySQL {
		if !pinned {
			unpinned++
			t.Logf("row order NOT pinned by SQL: %s — golden order is incidental, compare as a set in task 6.29", name)
		}
	}
	if unpinned != 7 {
		t.Fatalf("ordering inventory drifted: %d unpinned endpoints, want 7 — re-check the queries before trusting the goldens", unpinned)
	}
}
