package kind_test

import (
	"os"
	"path/filepath"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/audio/kind"
	_ "github.com/mutablelogic/go-metadata/audio/playlist"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	testassert "github.com/mutablelogic/go-metadata/internal/testassert"
)

// kind returns the kind, and DCMI type, of a file
func kind(t *testing.T, path string) (string, string) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	_, entries, _ := metadata.Extract(t.Context(), f)
	var kind, typ string
	for _, entry := range entries {
		switch {
		case entry.NS() == metadata.MetadataNamespace && entry.Key() == metadata.KindKey:
			kind = entry.Value()
		case entry.NS() == dc.Namespace && entry.Key() == dc.Type:
			typ = entry.Value()
		}
	}
	return kind, typ
}

func Test_FallbackKind(t *testing.T) {
	// Audio has a kind, which isn't unknown, of ffmpeg, or of its content
	// type, without it, and a DCMI type
	for _, name := range []string{"sample.mp3", "jfk.wav"} {
		kind, typ := kind(t, filepath.Join("..", "..", "testfiles", name))
		assert.True(t, kind != dc.KindUnknown, name)
		assert.NotEmpty(t, kind, name)
		assert.NotEmpty(t, typ, name)
	}

	// A playlist, whose content type is audio's, has its own
	path := filepath.Join(t.TempDir(), "list.m3u")
	require.NoError(t, os.WriteFile(path, []byte("#EXTM3U\n#EXTINF:1,One\none.mp3\n"), 0o644))
	kind, _ := kind(t, path)
	assert.Equal(t, dc.KindData, kind)
}

var (
	assert  = testassert.Assert
	require = testassert.Require
)
