package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"

	"github.com/saroel01/aether-cbt/internal/db"
	"github.com/saroel01/aether-cbt/internal/utils"
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

// ExportEssayResults exports student essay responses in CSV, XLSX, or PDF formats.
// Features a dynamic, robust layout with senior-developer visual quality.
func ExportEssayResults(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	role := c.Locals("role").(string)
	format := strings.ToLower(c.Params("format"))

	if role != "admin" && role != "supervisor" {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	// Fetch student essay answers
	rows, err := db.DB.Query(`
		SELECT p.no_id, p.nama_peserta, COALESCE(k.nama_kelas, '—'), COALESCE(m.nama_mapel, '—'),
		       hd.question_id, hd.question_text, hd.user_answer, hd.awarded_points, hd.max_points
		FROM hasil_tes_detail hd
		JOIN hasil_tes ht ON hd.hasil_tes_id = ht.id
		JOIN peserta p ON ht.peserta_id = p.id
		LEFT JOIN kelas k ON p.kelas_id = k.id
		LEFT JOIN mapel m ON ht.mapel_id = m.id
		WHERE ht.tenant_id = ? AND hd.question_type = 'essayQuestion'
		ORDER BY k.nama_kelas ASC, p.no_id ASC
	`, tenantID)

	if err != nil {
		log.Printf("Failed to query essay answers: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load essay results")
	}
	defer rows.Close()

	// Get Tenant Name for headers
	var tenantName string
	db.DB.QueryRow("SELECT name FROM tenants WHERE id = ?", tenantID).Scan(&tenantName)
	if tenantName == "" {
		tenantName = "Aether CBT Platform"
	}

	switch format {
	case "csv":
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)
		writer.Write([]string{"NIS", "NAMA SISWA", "KELAS", "MATA PELAJARAN", "ID SOAL", "PERTANYAAN ESAI", "JAWABAN SISWA", "SKOR SEMENTARA", "SKOR MAKSIMAL"})

		for rows.Next() {
			var noID, name, className, mapelName, qID, qText, userAns string
			var score, maxScore float64
			rows.Scan(&noID, &name, &className, &mapelName, &qID, &qText, &userAns, &score, &maxScore)

			writer.Write([]string{
				noID,
				name,
				className,
				mapelName,
				qID,
				qText,
				userAns,
				strconv.FormatFloat(score, 'f', 2, 64),
				strconv.FormatFloat(maxScore, 'f', 2, 64),
			})
		}
		writer.Flush()

		c.Set("Content-Type", "text/csv")
		c.Set("Content-Disposition", "attachment; filename=rekap_essai_siswa.csv")
		return c.Send(buf.Bytes())

	case "xlsx":
		f := excelize.NewFile()
		defer f.Close()

		sheetName := "Rekap Esai Siswa"
		f.SetSheetName("Sheet1", sheetName)

		// Set column headers
		headers := []string{"NIS", "NAMA SISWA", "KELAS", "MATA PELAJARAN", "ID SOAL", "PERTANYAAN ESAI", "JAWABAN ESAI SISWA", "SKOR SEMENTARA", "SKOR MAKSIMAL"}
		for colIdx, val := range headers {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
			f.SetCellValue(sheetName, cell, val)
		}

		// Header Style (Steel Blue Hex 4682B4, White, Bold, Centered)
		headerStyle, _ := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{"4682B4"}, Pattern: 1},
			Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		f.SetRowStyle(sheetName, 1, 1, headerStyle)

		rowIdx := 2
		for rows.Next() {
			var noID, name, className, mapelName, qID, qText, userAns string
			var score, maxScore float64
			rows.Scan(&noID, &name, &className, &mapelName, &qID, &qText, &userAns, &score, &maxScore)

			f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowIdx), noID)
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowIdx), name)
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowIdx), className)
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowIdx), mapelName)
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowIdx), qID)
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowIdx), qText)
			f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowIdx), userAns)
			f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowIdx), score)
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", rowIdx), maxScore)

			rowIdx++
		}

		// Apply grid borders and auto wrap on long columns
		dataStyle, _ := f.NewStyle(&excelize.Style{
			Border: []excelize.Border{
				{Type: "top", Color: "D3D3D3", Style: 1},
				{Type: "bottom", Color: "D3D3D3", Style: 1},
				{Type: "left", Color: "D3D3D3", Style: 1},
				{Type: "right", Color: "D3D3D3", Style: 1},
			},
		})
		wrapStyle, _ := f.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
			Border: []excelize.Border{
				{Type: "top", Color: "D3D3D3", Style: 1},
				{Type: "bottom", Color: "D3D3D3", Style: 1},
				{Type: "left", Color: "D3D3D3", Style: 1},
				{Type: "right", Color: "D3D3D3", Style: 1},
			},
		})

		if rowIdx > 2 {
			f.SetCellStyle(sheetName, "A2", fmt.Sprintf("I%d", rowIdx-1), dataStyle)
			f.SetCellStyle(sheetName, "F2", fmt.Sprintf("G%d", rowIdx-1), wrapStyle)
		}

		// Set deliberate, spacious column widths for readability
		f.SetColWidth(sheetName, "A", "A", 15) // NIS
		f.SetColWidth(sheetName, "B", "B", 25) // Nama
		f.SetColWidth(sheetName, "C", "C", 12) // Kelas
		f.SetColWidth(sheetName, "D", "D", 25) // Mapel
		f.SetColWidth(sheetName, "E", "E", 12) // ID Soal
		f.SetColWidth(sheetName, "F", "F", 40) // Soal
		f.SetColWidth(sheetName, "G", "G", 60) // Jawaban
		f.SetColWidth(sheetName, "H", "I", 18) // Skor

		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to build Excel sheet")
		}

		c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Set("Content-Disposition", "attachment; filename=rekap_essai_siswa.xlsx")
		return c.Send(buf.Bytes())

	case "pdf":
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.SetMargins(15, 15, 15)
		pdf.AliasNbPages("")
		pdf.AddPage()

		// Kop Surat / Header Dokumen Formal
		pdf.SetFont("Arial", "B", 16)
		pdf.SetTextColor(70, 130, 180) // Steel Blue
		pdf.CellFormat(180, 8, strings.ToUpper(tenantName), "", 1, "C", false, 0, "")
		
		pdf.SetTextColor(50, 50, 50)
		pdf.SetFont("Arial", "B", 12)
		pdf.CellFormat(180, 6, "REKAPITULASI JAWABAN ESAI SISWA", "", 1, "C", false, 0, "")
		
		pdf.SetFont("Arial", "I", 9)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(180, 5, fmt.Sprintf("Dicetak otomatis pada: %s", time.Now().Format("02 January 2006, 15:04 MST")), "", 1, "C", false, 0, "")
		
		// Double Line Divider
		pdf.SetDrawColor(70, 130, 180)
		pdf.SetLineWidth(0.8)
		pdf.Line(15, 36, 195, 36)
		pdf.SetLineWidth(0.2)
		pdf.Line(15, 37.2, 195, 37.2)
		pdf.Ln(8)

		var hasData bool

		for rows.Next() {
			hasData = true
			var noID, name, className, mapelName, qID, qText, userAns string
			var score, maxScore float64
			rows.Scan(&noID, &name, &className, &mapelName, &qID, &qText, &userAns, &score, &maxScore)

			// 1. Bar Identitas Siswa (Steel Blue fill, Bold text)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(255, 255, 255)
			pdf.SetFillColor(70, 130, 180) // Steel Blue
			pdf.CellFormat(180, 7, fmt.Sprintf("  %s (%s) — Kelas: %s", name, noID, className), "1", 1, "L", true, 0, "")

			// 2. Bar Info Kuis (White background, light gray text)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(100, 100, 100)
			pdf.SetDrawColor(200, 200, 200)
			pdf.CellFormat(180, 5, fmt.Sprintf("  MATA PELAJARAN: %s   |   KODE SOAL: %s", strings.ToUpper(mapelName), qID), "LR", 1, "L", false, 0, "")

			// 3. Panel Pertanyaan (Soft Gray background)
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetTextColor(60, 60, 60)
			pdf.SetFillColor(245, 245, 245)
			pdf.CellFormat(180, 5, "  Pertanyaan Soal:", "LR", 1, "L", true, 0, "")
			
			pdf.SetFont("Arial", "", 9)
			pdf.MultiCell(180, 5.5, "  "+qText, "LR", "L", true)

			// 4. Panel Jawaban Siswa (White background, Blue text)
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetTextColor(60, 60, 60)
			pdf.CellFormat(180, 5, "  Lembar Jawaban Siswa:", "LR", 1, "L", false, 0, "")
			
			pdf.SetFont("Arial", "I", 9.5)
			pdf.SetTextColor(30, 30, 90) // dark blue text
			if userAns == "" {
				userAns = "[Siswa tidak mengisi jawaban esai]"
			}
			pdf.MultiCell(180, 5.5, "  "+userAns, "LR", "L", false)

			// 5. Panel Penilaian Korektor (Yellowish background, clean boxes)
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetTextColor(60, 60, 60)
			pdf.SetFillColor(253, 253, 240) // soft yellow tint
			pdf.CellFormat(110, 8, fmt.Sprintf("  Skor Sementara Kuis:  %.1f / %.1f", score, maxScore), "1", 0, "L", true, 0, "")
			pdf.CellFormat(70, 8, "Nilai Akhir Guru:  ________  (Paraf: ____)  ", "1", 1, "R", true, 0, "")
			
			// Spacing between cards
			pdf.Ln(6)
		}

		if !hasData {
			pdf.SetFont("Arial", "I", 10)
			pdf.SetTextColor(120, 120, 120)
			pdf.CellFormat(180, 20, "Tidak ada data jawaban esai siswa yang terekam pada tenant ini.", "", 1, "C", false, 0, "")
		}

		// Footer - Page Number (Halaman X dari Y)
		pdf.SetFooterFunc(func() {
			pdf.SetY(-15)
			pdf.SetFont("Arial", "I", 8)
			pdf.SetTextColor(120, 120, 120)
			pdf.CellFormat(180, 10, fmt.Sprintf("Halaman %d/{nb}  |  Aether CBT Multi-Tenant", pdf.PageNo()), "", 0, "C", false, 0, "")
		})

		var buf bytes.Buffer
		if err := pdf.Output(&buf); err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to render PDF document")
		}

		c.Set("Content-Type", "application/pdf")
		c.Set("Content-Disposition", "attachment; filename=rekap_essai_siswa.pdf")
		return c.Send(buf.Bytes())

	default:
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid export format. Supported formats: csv, xlsx, pdf")
	}
}
