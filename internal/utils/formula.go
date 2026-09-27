package utils

import "strings"

// SanitizeFormulaField neutralizes CSV/formula injection for spreadsheet cells (P2-29).
// If a cell begins with '=', '+', '-', '@', '\t', or '\r', prepending a single quote
// prevents spreadsheet software (Excel, LibreOffice, Google Sheets) from executing it.
func SanitizeFormulaField(s string) string {
	trimmed := strings.Trim(s, " ")
	if trimmed == "" {
		return s
	}
	switch trimmed[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + trimmed
	default:
		return trimmed
	}
}
