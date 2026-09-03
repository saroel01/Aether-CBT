package handlers

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

type SupervisorLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SupervisorLoginResponse struct {
	Token    string `json:"token"`
	RuangID  int    `json:"ruang_id"`
	RoomName string `json:"room_name"`
}

// SupervisorLogin handles authenticating a room supervisor using Room credentials
func SupervisorLogin(c *fiber.Ctx) error {
	var req SupervisorLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	tenantID := c.Locals("tenant_id").(int)

	var id int
	var passwordHash string
	var namaRuang string
	err := db.DB.QueryRow(`
		SELECT id, password_hash, nama_ruang 
		FROM ruang 
		WHERE username = ? AND tenant_id = ? AND deleted_at IS NULL
	`, req.Username, tenantID).Scan(&id, &passwordHash, &namaRuang)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	if !utils.CheckPasswordHash(req.Password, passwordHash) {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Invalid credentials")
	}

	// Generate JWT token with supervisor role
	token, err := utils.GenerateToken(id, tenantID, "supervisor")
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to generate token")
	}

	return utils.SuccessResponse(c, SupervisorLoginResponse{
		Token:    token,
		RuangID:  id,
		RoomName: namaRuang,
	}, "Supervisor login successful")
}

type LiveStudentStatus struct {
	ID             int        `json:"id"`
	NoID           string     `json:"no_id"`
	NamaPeserta    string     `json:"nama_peserta"`
	KelasID        int        `json:"kelas_id"`
	NamaKelas      string     `json:"nama_kelas"`
	IsLoggedIn     bool       `json:"is_logged_in"`
	LoginTime      *time.Time `json:"login_time,omitempty"`
	SessionID      *int       `json:"session_id,omitempty"`
	MapelID        *int       `json:"mapel_id,omitempty"`
	NamaMapel      *string    `json:"nama_mapel,omitempty"`
	Skor           *float64   `json:"skor,omitempty"`
	SkorMaks       *float64   `json:"skor_maks,omitempty"`
	HasilStatus    *string    `json:"hasil_status,omitempty"`
	WaktuSelesai   *time.Time `json:"waktu_selesai,omitempty"`
	TabSwitches    int        `json:"tab_switches"`
	AnsweredCount  int        `json:"answered_count"`
	TotalQuestions int        `json:"total_questions"`
	Locked         bool       `json:"locked"`
	// Status is the effective per-session state: not_logged_in | in_progress | locked |
	// submitted (Requirement 11.1, 11.2). Computed server-side from cek_login + hasil_tes.
	Status string `json:"status"`
}

// fetchRoomStatus queries the per-student room overview, optionally scoped to a single
// exam_session (Requirement 11.1). Shared by GetRoomStatus and the SSE stream so the two
// never drift apart. Each cek_login subquery is tenant-scoped (and session-scoped when
// sessionID > 0); the result stays one row per student.
func fetchRoomStatus(tenantID, ruangID, sessionID int) ([]LiveStudentStatus, error) {
	// sub is the per-student cek_login predicate fragment; subArgs are its bind values.
	sub := "cl.peserta_id = p.id AND cl.tenant_id = ?"
	subArgs := []any{tenantID}
	if sessionID > 0 {
		sub += " AND cl.session_id = ?"
		subArgs = append(subArgs, sessionID)
	}

	// hasilScope constrains which hasil_tes row may supply skor/status/waktu_selesai.
	//
	// hasil_tes holds one row per submitted exam, so joining it on peserta_id alone both
	// multiplied a participant into one output row per exam they had ever sat (breaking the
	// one-row-per-student contract above) and let the score shown for the requested session
	// come from a different exam entirely — while every cek_login subquery in the same
	// statement was already session-scoped (codebase-bug-sweep clause 2.7).
	//
	// Historical rows are handled explicitly: migration 031 added exam_session_id via ALTER
	// TABLE ADD COLUMN, so pre-031 rows carry NULL. A NULL-attributed row belongs to no known
	// session, so `h.exam_session_id = ?` correctly excludes it from a session-scoped request —
	// admitting it would reinstate the very leak this scope closes. An unscoped request
	// (sessionID == 0, the legacy mapel-based dashboard) still considers those rows.
	hasilScope := ""
	var hasilArgs []any
	if sessionID > 0 {
		hasilScope = " AND h.exam_session_id = ?"
		hasilArgs = append(hasilArgs, sessionID)
	}

	// The join stays a LEFT JOIN on a single-row selector, so a participant with no matching
	// hasil_tes still appears exactly once with empty result columns (clause 3.8). Ties are
	// broken toward the most recent result, which is what a live monitor should show.
	hasilJoin := `
		LEFT JOIN hasil_tes ht ON ht.id = (
			SELECT h.id FROM hasil_tes h
			WHERE h.peserta_id = p.id AND h.tenant_id = p.tenant_id` + hasilScope + `
			ORDER BY h.waktu_selesai DESC, h.id DESC
			LIMIT 1
		)`

	query := `
		SELECT p.id, p.no_id, p.nama_peserta, COALESCE(p.kelas_id, 0), COALESCE(k.nama_kelas, '—'),
		       EXISTS(SELECT 1 FROM cek_login cl WHERE ` + sub + `) AS is_logged_in,
		       (SELECT cl.login_time FROM cek_login cl WHERE ` + sub + `) AS login_time,
		       (SELECT cl.mapel_id FROM cek_login cl WHERE ` + sub + `) AS mapel_id,
		       (SELECT cl.session_id FROM cek_login cl WHERE ` + sub + `) AS session_id,
		       COALESCE((SELECT cl.locked FROM cek_login cl WHERE ` + sub + `), 0) AS locked,
		       (SELECT m.nama_mapel FROM mapel m WHERE m.id = (SELECT cl.mapel_id FROM cek_login cl WHERE ` + sub + `)) AS nama_mapel,
		       ht.skor, ht.skor_maks, ht.status, ht.waktu_selesai,
		       COALESCE((SELECT cl.tab_switch_count FROM cek_login cl WHERE ` + sub + `), 0) AS tab_switches,
		       COALESCE((SELECT cl.answered_count FROM cek_login cl WHERE ` + sub + `), 0) AS answered_count,
		       COALESCE((SELECT cl.total_questions FROM cek_login cl WHERE ` + sub + `), 0) AS total_questions
		FROM peserta p
		LEFT JOIN kelas k ON p.kelas_id = k.id` + hasilJoin + `
		WHERE p.ruang_id = ? AND p.tenant_id = ? AND p.deleted_at IS NULL
	`
	// Bind order follows the order the placeholders appear in the statement: 9 cek_login
	// subqueries in the SELECT list, then the hasil_tes join scope, then the trailing
	// ruang/tenant of the WHERE clause.
	args := make([]any, 0, 9*len(subArgs)+len(hasilArgs)+2)
	for i := 0; i < 9; i++ {
		args = append(args, subArgs...)
	}
	args = append(args, hasilArgs...)
	args = append(args, ruangID, tenantID)

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []LiveStudentStatus
	for rows.Next() {
		var s LiveStudentStatus
		var loginTimeNull, waktuSelesaiNull sql.NullTime
		var mapelIDNull, sessionIDNull sql.NullInt64
		var lockedInt int
		var namaMapelNull sql.NullString
		var skorNull, skorMaksNull sql.NullFloat64
		var hasilStatusNull sql.NullString

		if err := rows.Scan(
			&s.ID, &s.NoID, &s.NamaPeserta, &s.KelasID, &s.NamaKelas,
			&s.IsLoggedIn, &loginTimeNull, &mapelIDNull, &sessionIDNull, &lockedInt, &namaMapelNull,
			&skorNull, &skorMaksNull, &hasilStatusNull, &waktuSelesaiNull,
			&s.TabSwitches, &s.AnsweredCount, &s.TotalQuestions,
		); err != nil {
			return nil, err
		}
		if loginTimeNull.Valid {
			s.LoginTime = &loginTimeNull.Time
		}
		if mapelIDNull.Valid {
			idVal := int(mapelIDNull.Int64)
			s.MapelID = &idVal
		}
		if sessionIDNull.Valid {
			idVal := int(sessionIDNull.Int64)
			s.SessionID = &idVal
		}
		if namaMapelNull.Valid {
			s.NamaMapel = &namaMapelNull.String
		}
		if skorNull.Valid {
			s.Skor = &skorNull.Float64
		}
		if skorMaksNull.Valid {
			s.SkorMaks = &skorMaksNull.Float64
		}
		if hasilStatusNull.Valid {
			hs := hasilStatusNull.String
			s.HasilStatus = &hs
		}
		if waktuSelesaiNull.Valid {
			s.WaktuSelesai = &waktuSelesaiNull.Time
		}
		s.Locked = lockedInt != 0
		s.Status = computeStudentStatus(s.IsLoggedIn, s.Locked, s.HasilStatus)
		list = append(list, s)
	}
	return list, nil
}

// computeStudentStatus derives the effective state: submitted beats locked beats in-progress;
// a student with no active session is not_logged_in (Requirement 11.1).
func computeStudentStatus(isLoggedIn, locked bool, hasilStatus *string) string {
	if !isLoggedIn {
		return "not_logged_in"
	}
	if hasilStatus != nil && *hasilStatus == "submitted" {
		return "submitted"
	}
	if locked {
		return "locked"
	}
	return "in_progress"
}

// GetRoomStatus returns real-time status of all students in supervisor's assigned room
func GetRoomStatus(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)
	ruangID := c.Locals("user_id").(int) // for supervisor, user_id maps to ruang_id

	if role != "supervisor" && role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	if role == "admin" {
		// If admin is viewing, they can pass a room_id query parameter
		queryRoomID := c.QueryInt("room_id", 0)
		if queryRoomID > 0 {
			ruangID = queryRoomID
		} else {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Room ID required for admin")
		}
	}

	sessionID := c.QueryInt("session_id", 0) // optional: scope to one exam_session
	list, err := fetchRoomStatus(tenantID, ruangID, sessionID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch room status")
	}
	return utils.SuccessResponse(c, list, "Room status retrieved")
}

// ResetStudentSession resets a student's session. When session_id is provided it targets ONLY
// that session (clearing its server lock by removing the row), leaving any other active
// session intact; otherwise it falls back to resetting all of the peserta's active sessions
// (Requirement 10.4, 11.3).
func ResetStudentSession(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "supervisor" && role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	var req struct {
		PesertaID int `json:"peserta_id"`
		SessionID int `json:"session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if req.PesertaID <= 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid student ID")
	}

	var (
		execErr error
		message = "Student session reset successful"
	)
	if req.SessionID > 0 {
		// Unlock is implicit: removing the cek_login row clears its lock so the student can
		// restart that session (Requirement 10.4).
		_, execErr = db.DB.Exec(
			`DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ? AND session_id = ?`,
			req.PesertaID, tenantID, req.SessionID,
		)
	} else {
		_, execErr = db.DB.Exec(
			`DELETE FROM cek_login WHERE peserta_id = ? AND tenant_id = ?`,
			req.PesertaID, tenantID,
		)
	}
	if execErr != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to reset student session")
	}
	return utils.SuccessResponse(c, nil, message)
}
