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
	// when it has no MusicBrainz or AcoustID tags, unless it's protected with
	// DRM or classified, or whatever its tags when it's tagged as spoken
	// word, unless it's protected, and its release, by its MusicBrainz release
	// type, or as an album, when it has an album's name and no release type
	for _, test := range []struct {
		name       string
		entries    []entry
		cover      bool
		drm        bool
		classified bool
		hints      []string
	}{
		{"untagged", nil, false, false, false, []string{metadata.HintTranscription}},
		{"protected", nil, false, true, false, nil},
		{"classified", nil, false, false, true, nil},
		{"protected, with an album", []entry{{AudioNamespace, "album", "Abbey Road"}}, true, true, false, []string{metadata.HintArtwork, metadata.HintAlbum}},
		{"cover", nil, true, false, false, []string{metadata.HintArtwork, metadata.HintTranscription}},
		{"album", []entry{{AudioNamespace, "album", "Abbey Road"}}, false, false, false, []string{metadata.HintTranscription, metadata.HintAlbum}},
		{"empty album", []entry{{AudioNamespace, "album", " "}}, false, false, false, []string{metadata.HintTranscription}},
		{"recording", []entry{{MusicBrainzNamespace, recordingIDKey, "id"}}, false, false, false, nil},
		{"acoustid", []entry{{AcoustIDNamespace, "id", "id"}}, false, false, false, nil},
		{"release", []entry{{MusicBrainzNamespace, "release-id", "id"}}, false, false, false, nil},
		{"release group", []entry{{MusicBrainzNamespace, "release-group-id", "id"}}, true, false, false, []string{metadata.HintArtwork}},
		{"album release", []entry{{MusicBrainzNamespace, "release-type", "album"}}, false, false, false, []string{metadata.HintAlbum}},
		{"compilation album", []entry{{MusicBrainzNamespace, "release-type", "Album; Compilation"}}, false, false, false, []string{metadata.HintAlbum}},
		{"single", []entry{{MusicBrainzNamespace, "release-type", "single"}, {AudioNamespace, "album", "Love Me Do"}}, false, false, false, []string{metadata.HintSingle}},
		{"ep", []entry{{MusicBrainzNamespace, "release-type", "EP"}}, false, false, false, []string{metadata.HintEP}},
		{"broadcast", []entry{{MusicBrainzNamespace, "release-type", "broadcast"}}, false, false, false, []string{metadata.HintBroadcast}},
		{"other", []entry{{MusicBrainzNamespace, "release-type", "other"}}, false, false, false, []string{metadata.HintOther}},
		{"audiobook", []entry{{MusicBrainzNamespace, recordingIDKey, "id"}, {MusicBrainzNamespace, "release-type", "Other; Audiobook"}}, false, false, true, []string{metadata.HintTranscription, metadata.HintOther}},
		{"spoken word", []entry{{MusicBrainzNamespace, "release-type", "album/spokenword"}}, false, false, true, []string{metadata.HintTranscription, metadata.HintAlbum}},
		{"podcast genre", []entry{{AcoustIDNamespace, "id", "id"}, {AudioNamespace, "genre", "Podcast"}}, false, false, true, []string{metadata.HintTranscription}},
		{"spoken word genre", []entry{{AudioNamespace, "genre", "Comedy; Spoken Word"}}, false, false, true, []string{metadata.HintTranscription}},
		{"audiobook genre", []entry{{AudioNamespace, "genre", "Audio-Book"}}, false, false, false, []string{metadata.HintTranscription}},
		{"protected audiobook", []entry{{AudioNamespace, "genre", "Audiobook"}}, false, true, false, nil},
		{"music genre", []entry{{MusicBrainzNamespace, recordingIDKey, "id"}, {AudioNamespace, "genre", "Hip Hop/Rap"}}, false, false, false, nil},
		{"unknown release type", []entry{{MusicBrainzNamespace, "release-type", "bootleg"}, {AudioNamespace, "album", "Live"}}, false, false, false, nil},
	} {
		if got := audioHints(test.entries, test.cover, test.drm, test.classified); !slices.Equal(got, test.hints) {
			t.Errorf("%s: expected %q, got %q", test.name, test.hints, got)
		}
	}
}

func Test_isProtected(t *testing.T) {
	// Audio is protected when its codec tag is FairPlay's, drms, which ffmpeg
	// reads as a little-endian fourcc, and not when it's AAC's, mp4a
	fourcc := func(s string) uint32 {
		return uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
	}
	if !isProtected(fourcc("drms")) {
		t.Error("expected drms to be protected")
	}
	if isProtected(fourcc("mp4a")) || isProtected(0) {
		t.Error("expected mp4a, and no tag, not to be protected")
	}
}
