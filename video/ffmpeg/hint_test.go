//go:build cgo

package ffmpeg_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

// hints returns the hints of the file, which are what to read of it next
func hints(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, meta, err := metadata.Extract(context.Background(), f, metadata.WithContentType(contentTypeForFile(t, path)), metadata.WithNamespace(metadata.MetadataNamespace))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == metadata.MetadataNamespace && m.Key() == metadata.HintKey {
			hints, _ := m.Any().([]string)
			return hints
		}
	}
	return nil
}

func Test_hint_000(t *testing.T) {
	// The hints of what to read of a file next, from what's read of it
	for _, test := range []struct {
		name  string
		hints []string
	}{
		{"sample.mp4", []string{"artwork"}},
	} {
		if got := hints(t, filepath.Join(TEST_DIR, test.name)); !slices.Equal(got, test.hints) {
			t.Errorf("%s: expected %q, got %q", test.name, test.hints, got)
		}
	}
}
