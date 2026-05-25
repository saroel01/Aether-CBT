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
	ID             int            `json:"id"`
	NoID           string         `json:"no_id"`
	NamaPeserta    string         `json:"nama_peserta"`
	KelasID        int            `json:"kelas_id"`
	NamaKelas      string         `json:"nama_kelas"`
	IsLoggedIn     bool           `json:"is_logged_in"`
	LoginTime      *time.Time     `json:"login_time,omitempty"`
	MapelID        *int           `json:"mapel_id,omitempty"`
	NamaMapel      *string        `json:"nama_mapel,omitempty"`
	Skor           *float64       `json:"skor,omitempty"`
	SkorMaks       *float64       `json:"skor_maks,omitempty"`
	HasilStatus    *string        `json:"hasil_status,omitempty"`
	WaktuSelesai   *time.Time     `json:"waktu_selesai,omitempty"`
	TabSwitches    int            `json:"tab_switches"`
	AnsweredCount  int            `json:"answered_count"`
	TotalQuestions int            `json:"total_questions"`
}

// GetRoomStatus returns real-time status of all students in supervisor's assigned room
func GetRoomStatus(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)
	ruangID := c.Locals("user_id").(int) // for supervisor, user_id maps to ruang_id

	// Allow admin to also pass a room parameter or use their own scope, but supervisor is locked to their room
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

	rows, err := db.DB.Query(`
		SELECT p.id, p.no_id, p.nama_peserta, p.kelas_id, COALESCE(k.nama_kelas, '—'),
		       EXISTS(SELECT 1 FROM cek_login cl WHERE cl.peserta_id = p.id AND cl.tenant_id = ?) AS is_logged_in,
		       (SELECT cl.login_time FROM cek_login cl WHERE cl.peserta_id = p.id AND cl.tenant_id = ?) AS login_time,
		       (SELECT cl.mapel_id FROM cek_login cl WHERE cl.peserta_id = p.id AND cl.tenant_id = ?) AS mapel_id,
		       (SELECT m.nama_mapel FROM mapel m WHERE m.id = (SELECT cl.mapel_id FROM cek_login cl WHERE cl.peserta_id = p.id AND cl.tenant_id = ?)) AS nama_mapel,
		       ht.skor, ht.skor_maks, ht.status, ht.waktu_selesai,
		       COALESCE((SELECT cl.tab_switch_count FROM cek_login cl WHERE cl.peserta_id = p.id AND cl.tenant_id = ?), 0) AS tab_switches,
		       COALESCE((SELECT cl.answered_count FROM cek_login cl WHERE cl.peserta_id = p.id AND cl.tenant_id = ?), 0) AS answered_count,
		       COALESCE((SELECT cl.total_questions FROM cek_login cl WHERE cl.peserta_id = p.id AND cl.tenant_id = ?), 0) AS total_questions
		FROM peserta p
		LEFT JOIN kelas k ON p.kelas_id = k.id
		LEFT JOIN hasil_tes ht ON p.id = ht.peserta_id AND ht.tenant_id = p.tenant_id
		WHERE p.ruang_id = ? AND p.tenant_id = ? AND p.deleted_at IS NULL
	`, tenantID, tenantID, tenantID, tenantID, tenantID, tenantID, tenantID, ruangID, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch room status")
	}
	defer rows.Close()

	var list []LiveStudentStatus
	for rows.Next() {
		var s LiveStudentStatus
		var loginTimeNull, waktuSelesaiNull sql.NullTime
		var mapelIDNull sql.NullInt64
		var namaMapelNull sql.NullString
		var skorNull, skorMaksNull sql.NullFloat64
		var hasilStatusNull sql.NullString

		err = rows.Scan(
			&s.ID, &s.NoID, &s.NamaPeserta, &s.KelasID, &s.NamaKelas,
			&s.IsLoggedIn, &loginTimeNull, &mapelIDNull, &namaMapelNull,
			&skorNull, &skorMaksNull, &hasilStatusNull, &waktuSelesaiNull,
			&s.TabSwitches, &s.AnsweredCount, &s.TotalQuestions,
		)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to scan room status row")
		}

		if loginTimeNull.Valid {
			s.LoginTime = &loginTimeNull.Time
		}
		if mapelIDNull.Valid {
			idVal := int(mapelIDNull.Int64)
			s.MapelID = &idVal
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
			s.HasilStatus = &hasilStatusNull.String
		}
		if waktuSelesaiNull.Valid {
			s.WaktuSelesai = &waktuSelesaiNull.Time
		}

		list = append(list, s)
	}

	return utils.SuccessResponse(c, list, "Room status retrieved")
}

// ResetStudentSession resets a student's session by removing them from active logs
func ResetStudentSession(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "supervisor" && role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	var req struct {
		PesertaID int `json:"peserta_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}

	_, err := db.DB.Exec(`
		DELETE FROM cek_login 
		WHERE peserta_id = ? AND tenant_id = ?
	`, req.PesertaID, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to reset student session")
	}

	return utils.SuccessResponse(c, nil, "Student session reset successful")
}
