package handlers

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
)

// GetRoomStatusSSE streams real-time room status to supervisors or admins using Server-Sent Events (SSE)
func GetRoomStatusSSE(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)
	ruangID := c.Locals("user_id").(int) // for supervisor, user_id maps to ruang_id

	if role != "supervisor" && role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	if role == "admin" {
		queryRoomID := c.QueryInt("room_id", 0)
		if queryRoomID > 0 {
			ruangID = queryRoomID
		} else {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Room ID required for admin")
		}
	}

	// Set headers for SSE
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Send initial heartbeat / comment to establish connection
		fmt.Fprintf(w, ": ok\n\n")
		w.Flush()

		// Stream loop
		for {
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
				// Database connection or query error, terminate stream
				break
			}

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
				if err == nil {
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
			}
			rows.Close()

			jsonData, err := json.Marshal(list)
			if err == nil {
				_, err = fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
				if err != nil {
					// Client disconnected
					break
				}
				err = w.Flush()
				if err != nil {
					// Client disconnected
					break
				}
			}

			time.Sleep(2 * time.Second)
		}
	})

	return nil
}
