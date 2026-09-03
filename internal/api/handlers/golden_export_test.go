package handlers

// Task 1.2 — golden baseline for csv_utility.go ExportResultsCSV and ExportEssayResults
// (all three format branches: csv, xlsx, pdf), recorded on the same dataset as task 1.1.
//
// CSV is byte-stable, so it is recorded verbatim. XLSX and PDF are NOT byte-stable — the XLSX
// zip carries per-build metadata and the PDF embeds time.Now() in both its header line and its
// /CreationDate — so what gets recorded is the EXTRACTED STRUCTURE: sheet/row/cell values and
// column widths for XLSX, per-page text runs for PDF. Byte comparison is deliberately not
// attempted on those two.

import (
	"bytes"
	"compress/zlib"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// ExportResultsCSV
// ---------------------------------------------------------------------------

// TestGoldenExportResultsCSV records the full CSV body of ExportResultsCSV: the header row, the
// column order, the two-decimal score formatting, and the row order produced by
// ORDER BY k.nama_kelas ASC, p.no_id ASC.
func TestGoldenExportResultsCSV(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/export/results", ExportResultsCSV)

	assertGolden(t, "export_results.csv", getOK(t, app, "/api/export/results"))
}

// ---------------------------------------------------------------------------
// ExportEssayResults — csv branch
// ---------------------------------------------------------------------------

// TestGoldenExportEssayResultsCSV records the csv branch verbatim.
func TestGoldenExportEssayResultsCSV(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/export/essay/:format", ExportEssayResults)

	assertGolden(t, "export_essay_results.csv", getOK(t, app, "/api/export/essay/csv"))
}

// ---------------------------------------------------------------------------
// ExportEssayResults — xlsx branch
// ---------------------------------------------------------------------------

type goldenXLSXColWidth struct {
	Column string  `json:"column"`
	Width  float64 `json:"width"`
}

type goldenXLSXSheet struct {
	Name      string               `json:"name"`
	Rows      [][]string           `json:"rows"`
	ColWidths []goldenXLSXColWidth `json:"col_widths"`
}

type goldenXLSXWorkbook struct {
	SheetOrder []string          `json:"sheet_order"`
	Sheets     []goldenXLSXSheet `json:"sheets"`
}

// TestGoldenExportEssayResultsXLSX records the extracted workbook structure: sheet name and
// order, every cell value in row order, and the column widths the handler sets. The zip bytes
// themselves are not stable across builds, so they are not compared.
func TestGoldenExportEssayResultsXLSX(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/export/essay/:format", ExportEssayResults)

	raw := getOK(t, app, "/api/export/essay/xlsx")
	assertGoldenJSON(t, "export_essay_results_xlsx_structure.json", extractXLSX(t, raw))
}

func extractXLSX(t *testing.T, raw []byte) goldenXLSXWorkbook {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer f.Close() //nolint:errcheck

	wb := goldenXLSXWorkbook{SheetOrder: f.GetSheetList()}
	for _, name := range wb.SheetOrder {
		rows, err := f.GetRows(name)
		if err != nil {
			t.Fatalf("get rows of sheet %q: %v", name, err)
		}
		sheet := goldenXLSXSheet{Name: name, Rows: rows}
		// Widths are part of the "format" the preservation clause covers; record the columns the
		// handler explicitly sizes (A..I).
		for _, col := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"} {
			w, err := f.GetColWidth(name, col)
			if err != nil {
				t.Fatalf("get col width %s!%s: %v", name, col, err)
			}
			sheet.ColWidths = append(sheet.ColWidths, goldenXLSXColWidth{Column: col, Width: w})
		}
		wb.Sheets = append(wb.Sheets, sheet)
	}
	return wb
}

// ---------------------------------------------------------------------------
// ExportEssayResults — pdf branch
// ---------------------------------------------------------------------------

type goldenPDFPage struct {
	Page     int      `json:"page"`
	TextRuns []string `json:"text_runs"`
	RunCount int      `json:"run_count"`
}

type goldenPDFDocument struct {
	PageCount int             `json:"page_count"`
	Pages     []goldenPDFPage `json:"pages"`
}

// pdfTimestampPlaceholder replaces the one line the handler renders from time.Now(). Everything
// else in the PDF text is derived from the seeded data and is stable.
const (
	pdfTimestampPrefix      = "Dicetak otomatis pada: "
	pdfTimestampPlaceholder = pdfTimestampPrefix + "<NORMALIZED-BY-GOLDEN-RECORDER>"
)

// TestGoldenExportEssayResultsPDF records the per-page text runs of the pdf branch. The PDF is
// not byte-stable (time.Now() in the header line, /CreationDate in the trailer), so the golden
// holds extracted text with the single wall-clock line normalized to a placeholder.
func TestGoldenExportEssayResultsPDF(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/export/essay/:format", ExportEssayResults)

	raw := getOK(t, app, "/api/export/essay/pdf")
	if !bytes.HasPrefix(raw, []byte("%PDF-")) {
		t.Fatalf("response is not a PDF (prefix %q)", truncateForLog(raw[:min(16, len(raw))]))
	}
	assertGoldenJSON(t, "export_essay_results_pdf_structure.json", extractPDF(t, raw))
}

// pdfContentStreamRe matches the object dictionary gofpdf writes immediately before each page's
// compressed content stream (fpdf.go putpages: `<</Filter /FlateDecode /Length %d>>` then
// `stream`). Page content objects are emitted in page order, so match order == page order.
var pdfContentStreamRe = regexp.MustCompile(`<</Filter /FlateDecode /Length (\d+)>>\nstream\n`)

// pdfShowTextRe pulls the operand out of a `(...)Tj` text-showing operator. gofpdf's CellFormat
// emits `(%s)Tj` with no separating space while Text() emits `(%s) Tj`, so the space is optional.
// One operator is written per line, so the greedy group safely spans an escaped `\)`.
var pdfShowTextRe = regexp.MustCompile(`\((.*)\)\s*Tj`)

func extractPDF(t *testing.T, raw []byte) goldenPDFDocument {
	t.Helper()

	matches := pdfContentStreamRe.FindAllSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		t.Fatal("no FlateDecode content stream found in PDF; the gofpdf output layout changed")
	}

	doc := goldenPDFDocument{PageCount: len(matches)}
	for i, m := range matches {
		length, err := strconv.Atoi(string(raw[m[2]:m[3]]))
		if err != nil {
			t.Fatalf("page %d: parse /Length: %v", i+1, err)
		}
		start := m[1] // first byte after "stream\n"
		if start+length > len(raw) {
			t.Fatalf("page %d: /Length %d overruns the %d-byte document", i+1, length, len(raw))
		}
		zr, err := zlib.NewReader(bytes.NewReader(raw[start : start+length]))
		if err != nil {
			t.Fatalf("page %d: open zlib stream: %v", i+1, err)
		}
		content, err := io.ReadAll(zr)
		_ = zr.Close()
		if err != nil {
			t.Fatalf("page %d: inflate content stream: %v", i+1, err)
		}

		page := goldenPDFPage{Page: i + 1}
		for _, line := range strings.Split(string(content), "\n") {
			sub := pdfShowTextRe.FindStringSubmatch(line)
			if sub == nil {
				continue
			}
			text := unescapePDFString(sub[1])
			if strings.HasPrefix(text, pdfTimestampPrefix) {
				text = pdfTimestampPlaceholder
			}
			page.TextRuns = append(page.TextRuns, text)
		}
		page.RunCount = len(page.TextRuns)
		doc.Pages = append(doc.Pages, page)
	}

	// Guard against silently recording an empty baseline: if gofpdf ever changes its text
	// operator syntax the extractor would produce zero runs and the golden would prove nothing.
	var total int
	for _, p := range doc.Pages {
		total += p.RunCount
	}
	if total == 0 {
		t.Fatal("extracted 0 text runs from the PDF; the text-showing operator syntax changed and the extractor must be updated before this golden is trustworthy")
	}
	return doc
}

// unescapePDFString reverses gofpdf's escape(): `\` -> `\\`, `(` -> `\(`, `)` -> `\)`,
// CR -> `\r`.
func unescapePDFString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'r':
			b.WriteByte('\r')
		default: // covers \\ , \( , \)
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Response headers for all four export branches
// ---------------------------------------------------------------------------

type goldenExportHeaders struct {
	Route              string `json:"route"`
	Status             int    `json:"status"`
	ContentType        string `json:"content_type"`
	ContentDisposition string `json:"content_disposition"`
}

// TestGoldenExportResponseHeaders records the Content-Type and Content-Disposition of every
// export branch. Clause 3.13 covers the export "format", and the download filename plus MIME
// type is the part of that format a client actually sees.
func TestGoldenExportResponseHeaders(t *testing.T) {
	app, _ := newGoldenApp(t, "admin")
	app.Get("/api/export/results", ExportResultsCSV)
	app.Get("/api/export/essay/:format", ExportEssayResults)

	routes := []string{
		"/api/export/results",
		"/api/export/essay/csv",
		"/api/export/essay/xlsx",
		"/api/export/essay/pdf",
	}
	var recorded []goldenExportHeaders
	for _, route := range routes {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, route, nil), -1)
		if err != nil {
			t.Fatalf("app.Test GET %s: %v", route, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		recorded = append(recorded, goldenExportHeaders{
			Route:              route,
			Status:             resp.StatusCode,
			ContentType:        resp.Header.Get("Content-Type"),
			ContentDisposition: resp.Header.Get("Content-Disposition"),
		})
	}
	assertGoldenJSON(t, "export_response_headers.json", recorded)
}

// TestGoldenExportOrderingIsNotFullyPinnedBySQL documents an ordering hazard for task 6.29.
//
// Both export queries end in `ORDER BY k.nama_kelas ASC, p.no_id ASC`. For ExportResultsCSV that
// is a total order on the seeded data (one hasil_tes row per student). For ExportEssayResults it
// is NOT: a student with several essay detail rows produces several output rows that all share
// the same (nama_kelas, no_id) key, so their relative order is decided by the query plan, not by
// SQL. The seed contains exactly such a student (peserta 102 -> details 5004 and 5005), which is
// why this is called out rather than assumed away.
func TestGoldenExportOrderingIsNotFullyPinnedBySQL(t *testing.T) {
	_, database := newGoldenApp(t, "admin")

	var ties int
	err := database.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT ht.peserta_id
			FROM hasil_tes_detail hd
			JOIN hasil_tes ht ON hd.hasil_tes_id = ht.id
			WHERE ht.tenant_id = 1 AND hd.question_type = 'essayQuestion'
			GROUP BY ht.peserta_id
			HAVING COUNT(*) > 1
		)`).Scan(&ties)
	if err != nil {
		t.Fatalf("count essay ordering ties: %v", err)
	}
	if ties == 0 {
		t.Fatal("seed no longer contains a student with multiple essay rows; the ExportEssayResults ordering hazard is no longer covered")
	}
	t.Logf("ExportEssayResults: %d student(s) produce rows that tie on (nama_kelas, no_id) — their relative order is NOT pinned by SQL; task 6.29 must not treat that ordering as a preserved contract", ties)
}
