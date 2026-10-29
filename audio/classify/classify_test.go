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

// classOf returns the audio metadata of a file, by key, with the options
func classOf(t *testing.T, path, contentType string, opts ...metadata.Option) map[string]metadata.Metadata {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, md, err := metadata.Extract(context.Background(), f, append(opts, metadata.WithContentType(contentType))...)
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
	got := classOf(t, "../../testfiles/jfk.wav", "audio/wav", metadata.WithClassifyAudio())
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
	got := classOf(t, writeSilence(t, 3), "audio/wav", metadata.WithClassifyAudio())
	if got["class"] == nil || got["class"].Value() != "silence" {
		t.Fatalf("expected silence, got %v", got["class"])
	}
	if silence, ok := got["silence"].Any().(float64); !ok || silence != 1 {
		t.Errorf("expected all silence, got %v", got["silence"])
	}
}

func Test_classify_002(t *testing.T) {
	// Audio isn't classified unless it's asked for
	got := classOf(t, "../../testfiles/jfk.wav", "audio/wav")
	for _, key := range []string{"class", "speech", "music", "silence"} {
		if got[key] != nil {
			t.Errorf("%s: expected nothing, got %v", key, got[key])
		}
	}
	if got["duration"] == nil {
		t.Error("expected the audio to be read")
	}
}

// hintsOf returns the hints of a file, with the options
func hintsOf(t *testing.T, path, contentType string, opts ...metadata.Option) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, md, err := metadata.Extract(context.Background(), f, append(opts, metadata.WithContentType(contentType))...)
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

func Test_classify_003(t *testing.T) {
	// Speech which is classified is transcribed, and silence isn't, and
	// audio which isn't classified is transcribed when it isn't tagged as
	// a recording
	for _, test := range []struct {
		name string
		path string
		opts []metadata.Option
		want bool
	}{
		{"speech", "../../testfiles/jfk.wav", []metadata.Option{metadata.WithClassifyAudio()}, true},
		{"silence", writeSilence(t, 3), []metadata.Option{metadata.WithClassifyAudio()}, false},
		{"not classified", "../../testfiles/jfk.wav", nil, true},
		{"silence, not classified", writeSilence(t, 3), nil, true},
	} {
		hints := hintsOf(t, test.path, "audio/wav", test.opts...)
		if got := slices.Contains(hints, metadata.HintTranscription); got != test.want {
			t.Errorf("%s: expected transcription %v, got hints %q", test.name, test.want, hints)
		}
	}
}
