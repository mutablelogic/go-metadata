package text

import (
	"bytes"
	"io"
	"unicode/utf8"

	// Packages
	charmap "golang.org/x/text/encoding/charmap"
	unicode "golang.org/x/text/encoding/unicode"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// MaxBytes is the most of a text document which is read, as the
	// document is decoded, or parsed, in memory. A larger document is read
	// up to this size.
	MaxBytes = 16 << 20
)

var (
	// The byte order marks of UTF-8 and UTF-16
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Read returns at most MaxBytes of the text as UTF-8, from the character set
// of its byte order mark, or UTF-8 when it's valid UTF-8, or else
// Windows-1252
func Read(r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes))
	if err != nil {
		return "", err
	}
	return Decode(data, len(data) == MaxBytes)
}

// Decode returns the text as UTF-8, from the character set of its byte order
// mark, or UTF-8 when it's valid UTF-8, or else Windows-1252. When the text
// was cut off, a character cut off at its end is removed first, so it isn't
// read as invalid UTF-8.
func Decode(data []byte, cut bool) (string, error) {
	switch {
	case bytes.HasPrefix(data, bomUTF8):
		data = data[len(bomUTF8):]
	case bytes.HasPrefix(data, bomUTF16LE), bytes.HasPrefix(data, bomUTF16BE):
		text, err := unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder().Bytes(data)
		return string(text), err
	}
	if cut {
		data = data[:validEnd(data)]
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	text, err := charmap.Windows1252.NewDecoder().Bytes(data)
	return string(text), err
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// validEnd returns the end of the data without a character which is cut off
// at its end, which is that character's start
func validEnd(data []byte) int {
	for i := len(data) - 1; i >= 0 && i >= len(data)-utf8.UTFMax; i-- {
		if utf8.RuneStart(data[i]) {
			if utf8.FullRune(data[i:]) {
				return len(data)
			}
			return i
		}
	}
	return len(data)
}
