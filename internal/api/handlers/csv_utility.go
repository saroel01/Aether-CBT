package handlers

import (
	"bytes"
	"encoding/csv"
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/anomalyco/aether-cbt/internal/db"
	"github.com/anomalyco/aether-cbt/internal/utils"
)

// ImportStudentsCSV parses a multipart CSV upload and imports students into the database
func ImportStudentsCSV(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Only administrators can import students")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "No file uploaded")
	}

	src, err := file.Open()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to open uploaded file")
	}
	defer src.Close()

	reader := csv.NewReader(src)
	// Skip header line
	header, err := reader.Read()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Failed to read CSV header")
	}

	// Validate minimal header fields
	if len(header) < 5 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid CSV format. Required fields: no_id, nama_peserta, kelas_id, ruang_id, jenis_kelamin")
	}

	var successCount int
	var errorCount int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errorCount++
			continue
		}

		noID := record[0]
		namaPeserta := record[1]
		kelasIDStr := record[2]
		ruangIDStr := record[3]
		jenisKelamin := record[4]
		password := "siswa123" // default password if not provided
		if len(record) > 5 && record[5] != "" {
			password = record[5]
		}

		kelasID, _ := strconv.Atoi(kelasIDStr)
		ruangID, _ := strconv.Atoi(ruangIDStr)

		if noID == "" || namaPeserta == "" || kelasID <= 0 || ruangID <= 0 {
			errorCount++
			continue
		}

		// Insert into db (ignore duplicates)
		_, err = db.DB.Exec(`
			INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id, jenis_kelamin)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(tenant_id, no_id) DO UPDATE SET
				nama_peserta = excluded.nama_peserta,
				kelas_id = excluded.kelas_id,
				ruang_id = excluded.ruang_id,
				jenis_kelamin = excluded.jenis_kelamin,
				password = excluded.password
		`, tenantID, noID, password, namaPeserta, kelasID, ruangID, jenisKelamin)

		if err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	return utils.SuccessResponse(c, fiber.Map{
		"success_count": successCount,
		"error_count":   errorCount,
	}, "CSV Import processed")
}

// ExportResultsCSV queries results and streams them back as a downloadable CSV sheet
func ExportResultsCSV(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)

	if role != "admin" && role != "supervisor" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	rows, err := db.DB.Query(`
		SELECT p.no_id, p.nama_peserta, COALESCE(k.nama_kelas, '—'), COALESCE(m.nama_mapel, '—'),
		       ht.skor, ht.skor_maks, ht.status, ht.created_at
		FROM hasil_tes ht
		JOIN peserta p ON ht.peserta_id = p.id
		LEFT JOIN kelas k ON p.kelas_id = k.id
		LEFT JOIN mapel m ON ht.mapel_id = m.id
		WHERE ht.tenant_id = ?
		ORDER BY k.nama_kelas ASC, p.no_id ASC
	`, tenantID)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load exam results")
	}
	defer rows.Close()

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write CSV headers
	writer.Write([]string{"NO_ID", "NAMA_PESERTA", "KELAS", "MATA_PELAJARAN", "SKOR", "SKOR_MAKSIMAL", "STATUS", "WAKTU_SUBMIT"})

	for rows.Next() {
		var noID, namaPeserta, namaKelas, namaMapel, status, createdAt string
		var skor, skorMaks float64

		err = rows.Scan(&noID, &namaPeserta, &namaKelas, &namaMapel, &skor, &skorMaks, &status, &createdAt)
		if err != nil {
			continue
		}

		writer.Write([]string{
			noID,
			namaPeserta,
			namaKelas,
			namaMapel,
			strconv.FormatFloat(skor, 'f', 2, 64),
			strconv.FormatFloat(skorMaks, 'f', 2, 64),
			status,
			createdAt,
		})
	}
	writer.Flush()

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=rekap_hasil_ujian.csv")
	return c.Send(buf.Bytes())
}
