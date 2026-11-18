package video_test

import (
	"os"
	"path/filepath"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	testassert "github.com/mutablelogic/go-metadata/internal/testassert"
	_ "github.com/mutablelogic/go-metadata/video"
)

// kind returns the kind of a file
func kind(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	_, entries, _ := metadata.Extract(t.Context(), f)
	for _, entry := range entries {
		if entry.NS() == metadata.MetadataNamespace && entry.Key() == metadata.KindKey {
			return entry.Value()
		}
	}
	return ""
}

func Test_FallbackKind(t *testing.T) {
	// Video has a kind, which isn't unknown, of ffmpeg, or of its content
	// type, without it
	k := kind(t, filepath.Join("..", "testfiles", "sample.mp4"))
	assert.True(t, k != dc.KindUnknown)
	assert.NotEmpty(t, k)
}

var (
	assert  = testassert.Assert
	require = testassert.Require
)
