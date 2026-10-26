//go:build cgo

package ffmpeg

import (
	"slices"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

func Test_hint_audio(t *testing.T) {
	// Audio's hints: its artwork, when it has cover art, its transcription,
	// when it has no MusicBrainz or AcoustID tags, and its collection, when
	// it has an album, by its name or MusicBrainz identifier
	for _, test := range []struct {
		name    string
		entries []entry
		cover   bool
		hints   []string
	}{
		{"untagged", nil, false, []string{metadata.HintTranscription}},
		{"cover", nil, true, []string{metadata.HintArtwork, metadata.HintTranscription}},
		{"album", []entry{{AudioNamespace, "album", "Abbey Road"}}, false, []string{metadata.HintTranscription, metadata.HintCollection}},
		{"empty album", []entry{{AudioNamespace, "album", " "}}, false, []string{metadata.HintTranscription}},
		{"recording", []entry{{MusicBrainzNamespace, recordingIDKey, "id"}}, false, nil},
		{"acoustid", []entry{{AcoustIDNamespace, "id", "id"}}, false, nil},
		{"release", []entry{{MusicBrainzNamespace, "release-id", "id"}}, false, []string{metadata.HintCollection}},
		{"release group", []entry{{MusicBrainzNamespace, "release-group-id", "id"}}, true, []string{metadata.HintArtwork, metadata.HintCollection}},
	} {
		if got := audioHints(test.entries, test.cover); !slices.Equal(got, test.hints) {
			t.Errorf("%s: expected %q, got %q", test.name, test.hints, got)
		}
	}
}
