package isbn

import "testing"

func Test_isbn_000(t *testing.T) {
	// An ISBN of ten or thirteen digits, whose check digit is right, with or
	// without hyphens, spaces, and urn:isbn: or ISBN before it
	for value, want := range map[string]string{
		"9780306406157":              "9780306406157",
		"urn:isbn:978-1-5098-6807-0": "9781509868070",
		"ISBN 0-306-40615-2":         "0306406152",
		"080442957X":                 "080442957X",
		"9781509868075":              "",
		"4006381333931":              "",
		"NOID":                       "",
	} {
		got, ok := Parse(value)
		if (want == "") == ok || (ok && got != want) {
			t.Errorf("%q: expected %q, got %q %v", value, want, got, ok)
		}
	}
}
