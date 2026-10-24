// Package isbn parses International Standard Book Numbers, which identify
// books, such as in their metadata
package isbn

import (
	"regexp"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// isbnRe matches an ISBN, of ten or thirteen digits, with or without
// hyphens or spaces, and urn:isbn: or ISBN before it
var isbnRe = regexp.MustCompile(`(?i)^(?:urn:isbn:|isbn:?\s*)?((?:97[89][-\s]?)?\d{1,5}[-\s]?\d{1,7}[-\s]?\d{1,7}[-\s]?[\dX])$`)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Parse returns the ISBN in the value, as its digits, without urn:isbn: or
// ISBN before it, and without hyphens or spaces, and true, or false when the
// value isn't an ISBN, or its check digit is wrong
func Parse(value string) (string, bool) {
	match := isbnRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return "", false
	}
	digits := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(match[1]))
	return digits, Valid(digits)
}

// Valid returns true when the digits are an ISBN of ten or thirteen
// digits, whose check digit is right. An ISBN of ten digits can end in X.
func Valid(digits string) bool {
	sum := 0
	switch len(digits) {
	case 10:
		for i, r := range digits {
			value := int(r - '0')
			if r == 'X' && i == 9 {
				value = 10
			} else if r < '0' || r > '9' {
				return false
			}
			sum += (10 - i) * value
		}
		return sum%11 == 0
	case 13:
		// An ISBN is a book's EAN, whose prefix is 978 or 979, rather than
		// another product's
		if !strings.HasPrefix(digits, "978") && !strings.HasPrefix(digits, "979") {
			return false
		}
		for i, r := range digits {
			if r < '0' || r > '9' {
				return false
			}
			if i%2 == 0 {
				sum += int(r - '0')
			} else {
				sum += 3 * int(r-'0')
			}
		}
		return sum%10 == 0
	default:
		return false
	}
}
