//go:build cgo && libfvad

package classify_test

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"

	// Handlers
	_ "github.com/mutablelogic/go-metadata/audio/classify"
	_ "github.com/mutablelogic/go-metadata/audio/ffmpeg"
)

// classOf returns the audio metadata of a file, by key
func classOf(t *testing.T, path, contentType string) map[string]metadata.Metadata {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, md, err := metadata.Extract(context.Background(), f, metadata.WithContentType(contentType))
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]metadata.Metadata)
	for _, m := range md {
		if m.NS() == "audio" {
			result[m.Key()] = m
		}
	}
	return result
}

// writeSilence writes a WAV file of seconds of silence, at 16kHz in one
// channel
func writeSilence(t *testing.T, seconds int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "silence.wav")
	n := uint32(seconds * 16000 * 2)
	header := []any{
		[4]byte{'R', 'I', 'F', 'F'}, 36 + n, [4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '}, uint32(16), uint16(1), uint16(1), uint32(16000), uint32(32000), uint16(2), uint16(16),
		[4]byte{'d', 'a', 't', 'a'}, n,
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, v := range header {
		if err := binary.Write(f, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.Write(make([]byte, n)); err != nil {
		t.Fatal(err)
	}
	return path
}

func Test_classify_000(t *testing.T) {
	// A speech, with pauses
	got := classOf(t, "../../testfiles/jfk.wav", "audio/wav")
	if got["class"] == nil || got["class"].Value() != "speech" {
		t.Fatalf("expected speech, got %v", got["class"])
	}
	if speech, ok := got["speech"].Any().(float64); !ok || speech < 0.7 {
		t.Errorf("expected mostly speech, got %v", got["speech"])
	}
	if silence, ok := got["silence"].Any().(float64); !ok || silence <= 0 || silence > 0.3 {
		t.Errorf("expected some silence, got %v", got["silence"])
	}
}

func Test_classify_001(t *testing.T) {
	// Silence
	got := classOf(t, writeSilence(t, 3), "audio/wav")
	if got["class"] == nil || got["class"].Value() != "silence" {
		t.Fatalf("expected silence, got %v", got["class"])
	}
	if silence, ok := got["silence"].Any().(float64); !ok || silence != 1 {
		t.Errorf("expected all silence, got %v", got["silence"])
	}
}

// hintsOf returns the hints of a file
func hintsOf(t *testing.T, path, contentType string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, md, err := metadata.Extract(context.Background(), f, metadata.WithContentType(contentType))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range md {
		if m.NS() == metadata.MetadataNamespace && m.Key() == metadata.HintKey {
			return strings.Split(m.Value(), ",")
		}
	}
	return nil
}

func Test_classify_002(t *testing.T) {
	// Speech is transcribed, and silence isn't
	for _, test := range []struct {
		name string
		path string
		want bool
	}{
		{"speech", "../../testfiles/jfk.wav", true},
		{"silence", writeSilence(t, 3), false},
	} {
		hints := hintsOf(t, test.path, "audio/wav")
		if got := slices.Contains(hints, metadata.HintTranscription); got != test.want {
			t.Errorf("%s: expected transcription %v, got hints %q", test.name, test.want, hints)
		}
	}
}

func Test_classify_003(t *testing.T) {
	// What isn't audio isn't classified, which isn't an error
	_, md, err := metadata.Extract(context.Background(), strings.NewReader("not audio"), metadata.WithContentType("audio/wav"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range md {
		if m.NS() == "audio" && m.Key() == "class" {
			t.Errorf("expected no class, got %v", m)
		}
	}
}

// withTag writes a copy of an MP3 file with an ID3 tag of a user-defined
// text frame, TXXX, with a description and a value, as MusicBrainz Picard
// writes its tags
func withTag(t *testing.T, path, description, value string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	frame := append([]byte{0}, []byte(description+"\x00"+value)...)
	var tag []byte
	tag = append(tag, "TXXX"...)
	tag = binary.BigEndian.AppendUint32(tag, uint32(len(frame)))
	tag = append(tag, 0, 0)
	tag = append(tag, frame...)
	n := len(tag)
	header := []byte{'I', 'D', '3', 3, 0, 0, byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)}
	result := filepath.Join(t.TempDir(), "tagged.mp3")
	if err := os.WriteFile(result, append(append(header, tag...), data...), 0o644); err != nil {
		t.Fatal(err)
	}
	return result
}

func Test_classify_004(t *testing.T) {
	// Audio with a MusicBrainz release type isn't classified, as it's a
	// known release, and without one it is
	got := classOf(t, "../../testfiles/sample.mp3", "audio/mpeg")
	if got["class"] == nil {
		t.Fatal("expected a class")
	}
	got = classOf(t, withTag(t, "../../testfiles/sample.mp3", "MusicBrainz Album Type", "album"), "audio/mpeg")
	if got["class"] != nil {
		t.Errorf("expected no class, got %v", got["class"])
	}
	if got["duration"] == nil {
		t.Error("expected the audio to be read")
	}
}

func Test_classify_005(t *testing.T) {
	// Speech which is tagged as a recording, such as a song with prominent
	// vocals, isn't transcribed, by each tag which identifies it, and speech
	// with other tags is
	for _, test := range []struct {
		description string
		key         string
		want        bool
	}{
		{"MusicBrainz Track Id", "musicbrainz:recording-id", false},
		{"MusicBrainz Release Track Id", "musicbrainz:track-id", false},
		{"MusicBrainz Album Id", "musicbrainz:release-id", false},
		{"MusicBrainz Release Group Id", "musicbrainz:release-group-id", false},
		{"MusicBrainz Artist Id", "musicbrainz:artist-id", false},
		{"MusicBrainz Work Id", "musicbrainz:work-id", false},
		{"Acoustid Id", "acoustid:id", false},
		{"MusicBrainz Album Status", "musicbrainz:release-status", true},
		{"Mood", "audio:mood", true},
	} {
		path := withTag(t, "../../testfiles/sample.mp3", test.description, "c0ffee00-0000-4000-8000-000000000000")
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		_, md, err := metadata.Extract(context.Background(), f, metadata.WithContentType("audio/mpeg"))
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		var tagged, classified bool
		var hints []string
		for _, m := range md {
			switch {
			case m.NS()+":"+m.Key() == test.key:
				tagged = true
			case m.NS() == "audio" && m.Key() == "class":
				classified = true
			case m.NS() == metadata.MetadataNamespace && m.Key() == metadata.HintKey:
				hints = strings.Split(m.Value(), ",")
			}
		}
		if !tagged || !classified {
			t.Errorf("%s: expected %s, and a class, got %v and %v", test.description, test.key, tagged, classified)
			continue
		}
		if got := slices.Contains(hints, metadata.HintTranscription); got != test.want {
			t.Errorf("%s: expected transcription %v, got hints %q", test.description, test.want, hints)
		}
	}
}
