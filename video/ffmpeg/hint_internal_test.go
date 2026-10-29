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
	// A video's hints: its artwork, when it has a video stream, unless it's
	// protected, when it's only when it has cover art, its place, when it has
	// a position, and that it's TV, when it's an episode of a show, by its
	// tags, or its name
	position, err := geo.New(52.52, 13.405)
	if err != nil {
		t.Fatal(err)
	}
	codec := meta.New(VideoNamespace, "codec", "h264")
	drm := meta.New(VideoNamespace, "drm", true)
	for _, test := range []struct {
		name  string
		meta  []metadata.Metadata
		path  string
		cover bool
		hints []string
	}{
		{"audio only", nil, "film.mkv", false, nil},
		{"video", []metadata.Metadata{codec}, "film.mkv", false, []string{metadata.HintArtwork}},
		{"protected", []metadata.Metadata{codec, drm}, "film.m4v", false, nil},
		{"protected with a cover", []metadata.Metadata{codec, drm}, "film.m4v", true, []string{metadata.HintArtwork}},
		{"position", []metadata.Metadata{codec, meta.New(VideoNamespace, locationKey, position)}, "", false, []string{metadata.HintArtwork, metadata.HintGeo}},
		{"show tag", []metadata.Metadata{meta.New(VideoNamespace, "show", "Seinfeld")}, "", false, []string{metadata.HintTV}},
		{"episode name", []metadata.Metadata{codec}, "/tv/Seinfeld/Seinfeld.S05E12.mkv", false, []string{metadata.HintArtwork, metadata.HintTV}},
	} {
		if got := videoHints(test.meta, test.path, test.cover); !slices.Equal(got, test.hints) {
			t.Errorf("%s: expected %q, got %q", test.name, test.hints, got)
		}
	}
}

func Test_isProtected(t *testing.T) {
	// A stream is protected when its codec tag is FairPlay's, drmi for video
	// and drms for audio, which ffmpeg reads as a little-endian fourcc
	fourcc := func(s string) uint32 {
		return uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
	}
	for _, tag := range []string{"drmi", "drms"} {
		if !isProtected(fourcc(tag)) {
			t.Errorf("expected %s to be protected", tag)
		}
	}
	for _, tag := range []string{"avc1", "mp4a"} {
		if isProtected(fourcc(tag)) {
			t.Errorf("expected %s not to be protected", tag)
		}
	}
}
