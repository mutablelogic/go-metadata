package playlist

import (
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

func Test_playlist_hint(t *testing.T) {
	// A playlist with entries has a hint of its Markdown, and one without
	// any has none
	got := playlistMetadata(t, "audio/x-mpegurl", "#EXTM3U\n#EXTINF:123,Artist - Title\nsong.mp3\n")
	if hints, _ := got["metadata:hint"].([]string); len(hints) != 1 || hints[0] != metadata.HintMarkdown {
		t.Errorf("expected the markdown hint, got %v", got["metadata:hint"])
	}
	got = playlistMetadata(t, "audio/x-mpegurl", "#EXTM3U\n")
	if _, exists := got["metadata:hint"]; exists {
		t.Errorf("expected no hint, got %v", got["metadata:hint"])
	}
}
