//go:build cgo

package ffmpeg

import (
	"slices"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

func Test_hint_video(t *testing.T) {
	// A video's hints: its artwork, when it has a video stream, its place,
	// when it has a position, and its collection, when it's an episode of a
	// show, by its tags, or its name
	position, err := geo.New(52.52, 13.405)
	if err != nil {
		t.Fatal(err)
	}
	codec := meta.New(VideoNamespace, "codec", "h264")
	for _, test := range []struct {
		name  string
		meta  []metadata.Metadata
		path  string
		hints []string
	}{
		{"audio only", nil, "film.mkv", nil},
		{"video", []metadata.Metadata{codec}, "film.mkv", []string{metadata.HintArtwork}},
		{"position", []metadata.Metadata{codec, meta.New(VideoNamespace, locationKey, position)}, "", []string{metadata.HintArtwork, metadata.HintGeo}},
		{"show tag", []metadata.Metadata{meta.New(VideoNamespace, "show", "Seinfeld")}, "", []string{metadata.HintCollection}},
		{"episode name", []metadata.Metadata{codec}, "/tv/Seinfeld/Seinfeld.S05E12.mkv", []string{metadata.HintArtwork, metadata.HintCollection}},
	} {
		if got := videoHints(test.meta, test.path); !slices.Equal(got, test.hints) {
			t.Errorf("%s: expected %q, got %q", test.name, test.hints, got)
		}
	}
}
