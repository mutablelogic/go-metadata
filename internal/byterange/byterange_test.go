package byterange_test

import (
	"testing"

	// Packages
	byterange "github.com/mutablelogic/go-metadata/internal/byterange"
)

func Test_byterange_000(t *testing.T) {
	// A range is from:to, from:, :to, or : for all of it, and an empty
	// range is the first kilobyte. A range which ends holds at least a
	// whole character.
	for _, test := range []struct {
		r             string
		offset, limit uint64
		err           bool
	}{
		{"", 0, 1024, false},
		{":", 0, 0, false},
		{"100:200", 100, 100, false},
		{"100:", 100, 0, false},
		{":50", 0, 50, false},
		{"100", 0, 0, true},
		{"x:200", 0, 0, true},
		{"100:x", 0, 0, true},
		{"200:100", 0, 0, true},
		{"100:100", 0, 0, true},
		{"100:103", 0, 0, true},
		{"100:104", 100, 4, false},
	} {
		offset, limit, err := byterange.Parse(test.r)
		if (err != nil) != test.err || offset != test.offset || limit != test.limit {
			t.Errorf("%q: expected %d %d %v, got %d %d %v", test.r, test.offset, test.limit, test.err, offset, limit, err)
		}
	}
}
