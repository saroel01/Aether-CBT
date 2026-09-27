package utils

import "testing"

func TestSanitizeFormulaField(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", "   "},
		{"Normal Name", "Normal Name"},
		{"=CMD|'calc.exe'!A0", "'=CMD|'calc.exe'!A0"},
		{"+628123456", "'+628123456"},
		{"-54321", "'-54321"},
		{"@SUM(A1:A10)", "'@SUM(A1:A10)"},
		{"\tTabIndent", "'\tTabIndent"},
		{"\rCarriageReturn", "'\rCarriageReturn"},
		{"  =LeadingSpaces", "'=LeadingSpaces"},
		{"  +628111", "'+628111"},
		{"Already 'quoted", "Already 'quoted"},
		{"Text with = inside is ok", "Text with = inside is ok"},
	}

	for _, tc := range cases {
		got := SanitizeFormulaField(tc.in)
		if got != tc.want {
			t.Errorf("SanitizeFormulaField(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
