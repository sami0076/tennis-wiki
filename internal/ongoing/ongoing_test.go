package ongoing

import "testing"

func TestRoundCodeReadsTheWTAFilesWords(t *testing.T) {
	for in, want := range map[string]string{
		"Quarterfinals": "QF",
		"Semifinals":    "SF",
		"Final":         "F",
		"Round of 16":   "R16",
		"R32":           "R32",
		"QF":            "QF",
		"RR":            "RR",
	} {
		if got := roundCode(in); got != want {
			t.Errorf("roundCode(%q) = %q, want %q", in, got, want)
		}
	}
}
