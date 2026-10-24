// Package byterange parses a range of bytes, such as of the Markdown of a
// document, which is read a part at a time
package byterange

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DefaultLimit is the number of bytes of a range which is empty: the
	// first kilobyte
	DefaultLimit = 1024
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Parse returns the byte offset and limit of a range of bytes, such as of
// the Markdown of a document,
// which is from:to, from the byte up to the byte, from:, from the byte to
// the end, :to, from the start up to the byte, or :, all of it. A limit of
// zero is to the end. An empty range is the first kilobyte. A range which
// ends holds at least a whole character.
func Parse(r string) (uint64, uint64, error) {
	if r == "" {
		return 0, DefaultLimit, nil
	}
	fromStr, toStr, ok := strings.Cut(r, ":")
	if !ok {
		return 0, 0, fmt.Errorf("invalid range %q, which is from:to, from:, :to or :", r)
	}
	var from, to uint64
	var err error
	if fromStr != "" {
		if from, err = strconv.ParseUint(fromStr, 10, 64); err != nil {
			return 0, 0, fmt.Errorf("invalid range %q, with a start which isn't a byte", r)
		}
	}
	if toStr == "" {
		return from, 0, nil
	}
	if to, err = strconv.ParseUint(toStr, 10, 64); err != nil {
		return 0, 0, fmt.Errorf("invalid range %q, with an end which isn't a byte", r)
	} else if to < from+utf8.UTFMax {
		return 0, 0, fmt.Errorf("invalid range %q, which ends before it starts, or is shorter than a character can be, %d bytes", r, utf8.UTFMax)
	}
	return from, to - from, nil
}
