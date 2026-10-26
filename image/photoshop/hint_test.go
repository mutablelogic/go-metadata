package photoshop_test

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
	// A document with a thumbnail has artwork, and its position, which is
	// read from its EXIF, with cgo, is tested with it
	if got := hints(t, filepath.Join(TEST_DIR, "sample.psd")); !slices.Contains(got, "artwork") {
		t.Errorf("sample.psd: expected artwork, got %q", got)
	}
}
