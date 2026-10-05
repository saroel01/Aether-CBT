package handlers

import (
	"bytes"
	"database/sql"
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

// MaxCSVImportRows defines the maximum number of data rows allowed in a single CSV import (P2-23).
const MaxCSVImportRows = 5000

// MaxUniquePasswordsPerImport defines the maximum number of unique passwords allowed per import (P2-23)
// to prevent CPU exhaustion and prolonged SQLite database lock times from bcrypt hashing.
const MaxUniquePasswordsPerImport = 500

// SanitizeFormulaField neutralizes CSV/formula injection for spreadsheet cells (P2-29).
func SanitizeFormulaField(s string) string {
	return utils.SanitizeFormulaField(s)
}

// ImportStudentsCSV parses a multipart CSV upload and imports students into the database
func ImportStudentsCSV(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	file, err := c.FormFile("file")
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "No file uploaded")
	}

	src, err := file.Open()
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to open uploaded file")
	}
	defer src.Close()

	// Read all bytes so we can strip a UTF-8 BOM (Excel exports prepend one) before parsing.
	raw, err := io.ReadAll(src)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to read uploaded file")
	}
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:] // strip UTF-8 BOM so the header parses correctly (review H16, Task 25)
	}

	// Phase 1 (no transaction): parse, validate and bcrypt-hash every password up front so
	// the write lock is never held during hashing (M10).
	existing := map[string]bool{}
	exRows, err := db.DB.QueryContext(c.Context(), `SELECT no_id FROM peserta WHERE tenant_id = ?`, tenantID)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load existing students")
	}
	for exRows.Next() {
		var n string
		if err := exRows.Scan(&n); err != nil {
			exRows.Close()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load existing students")
		}
		existing[n] = true
	}
	exRows.Close()
	if err := exRows.Err(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load existing students")
	}

	students, ierr := prepareCSVImport(raw, existing)
	if ierr != nil {
		return utils.ErrorResponse(c, ierr.status, ierr.msg)
	}

	// Phase 2: all-or-nothing write. Every row is validated and upserted inside a single
	// transaction, so a bad row rolls back the whole batch (review H16, Task 25).
	tx, err := db.DB.BeginTx(c.Context(), nil)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to begin import transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	for _, s := range students {
		rowIdx, kelasRef, ruangRef := s.row, s.kelas, s.ruang

		// Tenant-ref validation (mirrors CreateStudent, Task 23). Only a supplied reference is
		// validated; a normalized NULL has no parent row to check.
		var refOK int
		if kelasRef.Valid {
			if err := tx.QueryRowContext(c.Context(),
				`SELECT COUNT(*) FROM kelas WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
				kelasRef.Int64, tenantID,
			).Scan(&refOK); err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to validate kelas")
			}
			if refOK == 0 {
				return utils.ErrorResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Row %d: kelas_id %d not found in tenant", rowIdx, kelasRef.Int64))
			}
		}
		if ruangRef.Valid {
			if err := tx.QueryRowContext(c.Context(),
				`SELECT COUNT(*) FROM ruang WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
				ruangRef.Int64, tenantID,
			).Scan(&refOK); err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to validate ruang")
			}
			if refOK == 0 {
				return utils.ErrorResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Row %d: ruang_id %d not found in tenant", rowIdx, ruangRef.Int64))
			}
		}

		hasExplicitPassword := 0
		if s.passwordHash != "" {
			hasExplicitPassword = 1
		}
		// For an existing student without a password cell the CASE keeps peserta.password;
		// the inserted value is never used because the row already exists.
		_, err = tx.ExecContext(c.Context(), `
			INSERT INTO peserta (tenant_id, no_id, password, nama_peserta, kelas_id, ruang_id, jenis_kelamin)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(tenant_id, no_id) DO UPDATE SET
				nama_peserta = excluded.nama_peserta,
				kelas_id = excluded.kelas_id,
				ruang_id = excluded.ruang_id,
				jenis_kelamin = excluded.jenis_kelamin,
				password = CASE
					WHEN ? = 1 THEN excluded.password
					ELSE peserta.password
				END,
				token_version = CASE
					WHEN ? = 1 THEN peserta.token_version + 1
					ELSE peserta.token_version
				END
		`, tenantID, s.noID, s.passwordHash, s.nama, kelasRef, ruangRef, s.jenisKelamin, hasExplicitPassword, hasExplicitPassword)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Row %d: failed to insert (%v)", rowIdx, err))
		}
	}

	if err := tx.Commit(); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to commit import")
	}

	return utils.SuccessResponse(c, fiber.Map{
		"imported": len(students),
	}, "Import succeeded")
}

// csvStudentRow is one validated CSV row with its password already hashed (M10).
type csvStudentRow struct {
	row                      int
	noID, nama, jenisKelamin string
	kelas, ruang             sql.NullInt64
	passwordHash             string // "" = keep the existing student's password
}

type csvImportError struct {
	status int
	msg    string
}

func badCSV(format string, args ...any) *csvImportError {
	return &csvImportError{status: fiber.StatusBadRequest, msg: fmt.Sprintf(format, args...)}
}

// prepareCSVImport parses and validates the whole CSV and hashes every distinct password,
// without touching the database. existing holds the tenant's current no_id values: a new
// student must have a password (L5); an existing one without a password keeps theirs.
func prepareCSVImport(raw []byte, existing map[string]bool) ([]csvStudentRow, *csvImportError) {
	reader := csv.NewReader(bytes.NewReader(raw))
	// Skip header line
	header, err := reader.Read()
	if err != nil {
		return nil, badCSV("Failed to read CSV header")
	}

	// Validate minimal header fields
	if len(header) < 5 {
		return nil, badCSV("Invalid CSV format. Required fields: no_id, nama_peserta, kelas_id, ruang_id, jenis_kelamin")
	}

	var out []csvStudentRow
	passwordCache := make(map[string]string)
	seenNoIDs := make(map[string]int) // tracks no_id to row number for uniqueness validation
	rowIdx := 1                       // header is row 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, badCSV("CSV parse error on row %d", rowIdx+1)
		}
		if (rowIdx - 1) >= MaxCSVImportRows {
			return nil, badCSV("Jumlah baris CSV melebihi batas maksimum (%d baris)", MaxCSVImportRows)
		}
		rowIdx++

		rawNoID := strings.Trim(record[0], " ")
		rawNamaPeserta := strings.Trim(record[1], " ")
		kelasCell := strings.TrimSpace(record[2])
		ruangCell := strings.TrimSpace(record[3])
		jenisKelamin := strings.TrimSpace(record[4])

		if strings.TrimSpace(rawNoID) == "" || strings.TrimSpace(rawNamaPeserta) == "" {
			return nil, badCSV("Row %d: missing/invalid no_id, nama, kelas_id, or ruang_id", rowIdx)
		}

		// Prevent formula injection (P2-29)
		noID := SanitizeFormulaField(rawNoID)
		namaPeserta := SanitizeFormulaField(rawNamaPeserta)

		// Validate uniqueness of no_id within CSV (P2-23)
		if prevRow, exists := seenNoIDs[noID]; exists {
			return nil, badCSV("Row %d: duplikat nomor peserta '%s' dalam berkas CSV (sebelumnya di baris %d)", rowIdx, rawNoID, prevRow)
		}
		seenNoIDs[noID] = rowIdx

		// Password handling (P2-22, L5): a non-empty 6th column sets the password. Without it
		// (or with the legacy default "siswa123", which old templates still carry), an
		// existing student keeps theirs and a new student is rejected (no default password).
		password := ""
		if len(record) > 5 {
			if p := strings.TrimSpace(record[5]); p != "siswa123" {
				password = p
			}
		}

		// A blank cell, or the legacy sentinel 0, means "not assigned" and is normalized to
		// SQL NULL — the same rule CreateStudent applies (clause 2.2). Anything else must be a
		// parseable, positive id, so a typo is still rejected instead of silently dropping the
		// student's class or room assignment.
		kelasRef, err := parseOptionalCSVRef(kelasCell)
		if err != nil {
			return nil, badCSV("Row %d: missing/invalid no_id, nama, kelas_id, or ruang_id", rowIdx)
		}
		ruangRef, err := parseOptionalCSVRef(ruangCell)
		if err != nil {
			return nil, badCSV("Row %d: missing/invalid no_id, nama, kelas_id, or ruang_id", rowIdx)
		}

		if password == "" && !existing[noID] {
			return nil, badCSV("Row %d: password wajib untuk siswa baru", rowIdx)
		}
		// Same minimum as CreateStudent (L5).
		if password != "" && len(password) < 6 {
			return nil, badCSV("Row %d: password minimal 6 karakter", rowIdx)
		}

		var passwordHash string
		if password != "" {
			var cached bool
			passwordHash, cached = passwordCache[password]
			if !cached {
				if len(passwordCache) >= MaxUniquePasswordsPerImport {
					return nil, badCSV("Jumlah password unik dalam satu berkas CSV melebihi batas maksimum (%d)", MaxUniquePasswordsPerImport)
				}
				var hashErr error
				passwordHash, hashErr = utils.HashPassword(password)
				if hashErr != nil {
					return nil, &csvImportError{status: fiber.StatusInternalServerError, msg: fmt.Sprintf("Row %d: failed to hash password", rowIdx)}
				}
				passwordCache[password] = passwordHash
			}
		}

		out = append(out, csvStudentRow{
			row: rowIdx, noID: noID, nama: namaPeserta, jenisKelamin: jenisKelamin,
			kelas: kelasRef, ruang: ruangRef, passwordHash: passwordHash,
		})
	}
	return out, nil
}

// parseOptionalCSVRef converts a kelas_id / ruang_id cell into a value safe to write into a
// FOREIGN KEY column. An empty cell or an explicit 0 normalizes to SQL NULL ("not assigned",
// clause 2.2); a positive integer is passed through unchanged; anything else is an error so a
// malformed cell is still reported to the admin rather than silently discarded.
func parseOptionalCSVRef(cell string) (sql.NullInt64, error) {
	if cell == "" {
		return sql.NullInt64{}, nil
	}
	id, err := strconv.ParseInt(cell, 10, 64)
	if err != nil || id < 0 {
		return sql.NullInt64{}, fmt.Errorf("invalid reference %q", cell)
	}
	return db.NullableFK(id), nil
}

// scoreSourceLabel renders hasil_tes.score_source for the SUMBER_SKOR column (audit C1, D5).
func scoreSourceLabel(source string) string {
	switch source {
	case "server":
		return "server"
	case "mixed":
		return "campuran"
	case "unmatched":
		return "kunci tidak cocok"
	default:
		return "dilaporkan klien"
	}
}

// Export/list caps (L4). Queries fetch cap+1 rows to detect truncation. Variables so
// tests can lower them.
var (
	maxExportRows    = 20000
	maxEssayListRows = 2000
)

// exportFilename appends _TERPOTONG when the export hit maxExportRows.
func exportFilename(base, ext string, truncated bool) string {
	if truncated {
		base += "_TERPOTONG"
	}
	return base + "." + ext
}

// ExportResultsCSV queries results and streams them back as a downloadable CSV sheet
func ExportResultsCSV(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)

	scope, scopeArgs, err := resultScope(c)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	rows, err := db.DB.Query(`
		SELECT p.no_id, p.nama_peserta, COALESCE(k.nama_kelas, '—'), COALESCE(m.nama_mapel, '—'),
		       ht.skor, ht.skor_maks, ht.status, ht.created_at, COALESCE(ht.score_source, 'client')
		FROM hasil_tes ht
		JOIN peserta p ON ht.peserta_id = p.id
		LEFT JOIN kelas k ON p.kelas_id = k.id
		LEFT JOIN mapel m ON ht.mapel_id = m.id
		WHERE ht.tenant_id = ?`+scope+`
		ORDER BY k.nama_kelas ASC, p.no_id ASC
		LIMIT ?
	`, append(append([]any{tenantID}, scopeArgs...), maxExportRows+1)...)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load exam results")
	}
	defer rows.Close()

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buf)

	// Write CSV headers
	writer.Write([]string{"NO_ID", "NAMA_PESERTA", "KELAS", "MATA_PELAJARAN", "SKOR", "SKOR_MAKSIMAL", "STATUS", "WAKTU_SUBMIT", "SUMBER_SKOR"})

	n, truncated := 0, false
	for rows.Next() {
		if n++; n > maxExportRows {
			truncated = true
			break
		}
		var noID, namaPeserta, namaKelas, namaMapel, status, createdAt, scoreSource string
		var skor, skorMaks sql.NullFloat64

		err = rows.Scan(&noID, &namaPeserta, &namaKelas, &namaMapel, &skor, &skorMaks, &status, &createdAt, &scoreSource)
		if err != nil {
			log.Printf("[export-csv] scan error: %v", err)
			continue
		}

		skorStr := "—"
		if skor.Valid {
			skorStr = strconv.FormatFloat(skor.Float64, 'f', 2, 64)
		}
		skorMaksStr := "—"
		if skorMaks.Valid {
			skorMaksStr = strconv.FormatFloat(skorMaks.Float64, 'f', 2, 64)
		}

		writer.Write([]string{
			SanitizeFormulaField(noID),
			SanitizeFormulaField(namaPeserta),
			SanitizeFormulaField(namaKelas),
			SanitizeFormulaField(namaMapel),
			skorStr,
			skorMaksStr,
			status,
			createdAt,
			scoreSourceLabel(scoreSource),
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[export-csv] row iteration error: %v", err)
	}
	writer.Flush()

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename="+exportFilename("rekap_hasil_ujian", "csv", truncated))
	if truncated {
		c.Set("X-Export-Truncated", "true")
		log.Printf("[export-csv] tenant=%d: export truncated at %d rows", tenantID, maxExportRows)
	}
	return c.Send(buf.Bytes())
}

// ExportEssayResults exports student essay responses in CSV, XLSX, or PDF formats.
// Features a dynamic, robust layout with senior-developer visual quality.
func ExportEssayResults(c *fiber.Ctx) error {
	tenantID := c.Locals("tenant_id").(int)
	format := strings.ToLower(c.Params("format"))

	scope, scopeArgs, err := resultScope(c)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Unauthorized access")
	}

	// Fetch student essay answers (capped at maxExportRows; L4 flags truncation).
	rows, err := db.DB.Query(`
		SELECT p.no_id, p.nama_peserta, COALESCE(k.nama_kelas, '—'), COALESCE(m.nama_mapel, '—'),
		       hd.question_id, hd.question_text, hd.user_answer, hd.awarded_points, hd.max_points
		FROM hasil_tes_detail hd
		JOIN hasil_tes ht ON hd.hasil_tes_id = ht.id
		JOIN peserta p ON ht.peserta_id = p.id
		LEFT JOIN kelas k ON p.kelas_id = k.id
		LEFT JOIN mapel m ON ht.mapel_id = m.id
		WHERE ht.tenant_id = ? AND hd.question_type = 'essayQuestion'`+scope+`
		ORDER BY k.nama_kelas ASC, p.no_id ASC
		LIMIT ?
	`, append(append([]any{tenantID}, scopeArgs...), maxExportRows+1)...)

	if err != nil {
		log.Printf("Failed to query essay answers: %v", err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to load essay results")
	}
	defer rows.Close()

	// next caps iteration at maxExportRows and records whether rows were cut off (L4).
	n, truncated := 0, false
	next := func() bool {
		if !rows.Next() {
			return false
		}
		if n++; n > maxExportRows {
			truncated = true
			return false
		}
		return true
	}
	markTruncated := func() {
		if truncated {
			c.Set("X-Export-Truncated", "true")
			log.Printf("[export-essay] tenant=%d: export truncated at %d rows", tenantID, maxExportRows)
		}
	}

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

		for next() {
			var noID, name, className, mapelName, qID, qText, userAns string
			var score, maxScore float64
			if err := rows.Scan(&noID, &name, &className, &mapelName, &qID, &qText, &userAns, &score, &maxScore); err != nil {
				log.Printf("[export-essay-csv] scan error: %v", err)
				continue
			}

			writer.Write([]string{
				SanitizeFormulaField(noID),
				SanitizeFormulaField(name),
				SanitizeFormulaField(className),
				SanitizeFormulaField(mapelName),
				SanitizeFormulaField(qID),
				SanitizeFormulaField(qText),
				SanitizeFormulaField(userAns),
				strconv.FormatFloat(score, 'f', 2, 64),
				strconv.FormatFloat(maxScore, 'f', 2, 64),
			})
		}
		if err := rows.Err(); err != nil {
			log.Printf("[export-essay-csv] iteration error: %v", err)
		}
		writer.Flush()

		c.Set("Content-Type", "text/csv")
		markTruncated()
		c.Set("Content-Disposition", "attachment; filename="+exportFilename("rekap_essai_siswa", "csv", truncated))
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
			Fill:      excelize.Fill{Type: "pattern", Color: []string{"4682B4"}, Pattern: 1},
			Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		f.SetRowStyle(sheetName, 1, 1, headerStyle)

		rowIdx := 2
		for next() {
			var noID, name, className, mapelName, qID, qText, userAns string
			var score, maxScore float64
			if err := rows.Scan(&noID, &name, &className, &mapelName, &qID, &qText, &userAns, &score, &maxScore); err != nil {
				log.Printf("[export-essay-xlsx] scan error: %v", err)
				continue
			}

			f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowIdx), SanitizeFormulaField(noID))
			f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowIdx), SanitizeFormulaField(name))
			f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowIdx), SanitizeFormulaField(className))
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowIdx), SanitizeFormulaField(mapelName))
			f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowIdx), SanitizeFormulaField(qID))
			f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowIdx), SanitizeFormulaField(qText))
			f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowIdx), SanitizeFormulaField(userAns))
			f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowIdx), score)
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", rowIdx), maxScore)

			rowIdx++
		}
		if err := rows.Err(); err != nil {
			log.Printf("[export-essay-xlsx] iteration error: %v", err)
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
		markTruncated()
		c.Set("Content-Disposition", "attachment; filename="+exportFilename("rekap_essai_siswa", "xlsx", truncated))
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

		for next() {
			var noID, name, className, mapelName, qID, qText, userAns string
			var score, maxScore float64
			if err := rows.Scan(&noID, &name, &className, &mapelName, &qID, &qText, &userAns, &score, &maxScore); err != nil {
				log.Printf("[export-essay-pdf] scan error: %v", err)
				continue
			}
			hasData = true

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
		if err := rows.Err(); err != nil {
			log.Printf("[export-essay-pdf] iteration error: %v", err)
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
		markTruncated()
		c.Set("Content-Disposition", "attachment; filename="+exportFilename("rekap_essai_siswa", "pdf", truncated))
		return c.Send(buf.Bytes())

	default:
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid export format. Supported formats: csv, xlsx, pdf")
	}
}
