//go:build cgo

package photoshop_test

import (
	"path/filepath"
	"slices"
	"testing"
)

func Test_hint_exif_000(t *testing.T) {
	// A document with a thumbnail, and a position in its EXIF, has artwork
	// and places
	if got, want := hints(t, filepath.Join(TEST_DIR, "sample.psd")), []string{"artwork", "geo"}; !slices.Equal(got, want) {
		t.Errorf("sample.psd: expected %q, got %q", want, got)
	}
}
