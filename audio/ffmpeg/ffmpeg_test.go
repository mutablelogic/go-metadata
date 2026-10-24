//go:build cgo

package ffmpeg_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/audio/ffmpeg"
)

// audioMeta returns the metadata of the audio in the test files, by key
func audioMeta(t *testing.T, name string, opts ...metadata.Option) map[string]metadata.Metadata {
	t.Helper()
	path := filepath.Join(TEST_DIR, name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	meta, err := getMetadata(context.Background(), f, contentTypeForFile(t, path), opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]metadata.Metadata, len(meta))
	for _, m := range meta {
		result[key(m)] = m
	}
	return result
}

func Test_ffmpeg_000(t *testing.T) {
	// The duration, codec and kind of audio
	for name, want := range map[string]map[string]string{
		"sample.mp3": {"audio:codec": "mp3", "audio:sample-rate": "44100", "audio:channels": "2"},
		"jfk.wav":    {"audio:codec": "pcm_s16le", "audio:sample-rate": "16000", "audio:channels": "1"},
	} {
		got := audioMeta(t, name)
		want["metadata:kind"], want["dc:type"] = "audio", "Sound"
		for key, value := range want {
			if m := got[key]; m == nil || m.Value() != value {
				t.Errorf("%s: %s: expected %q, got %v", name, key, value, m)
			}
		}
		if d, ok := got["audio:duration"].Any().(time.Duration); !ok || d <= 0 {
			t.Errorf("%s: expected a duration, got %v", name, got["audio:duration"])
		}
	}
}

func Test_ffmpeg_001(t *testing.T) {
	// The cover art of audio is its artwork, the first as artwork:cover,
	// and the others numbered, and audio without any has none
	for name, want := range map[string][]string{
		"sample_with_artwork.mp3":       {"artwork:cover"},
		"sample_with_multi_artwork.mp3": {"artwork:cover", "artwork:cover-2"},
		"sample.mp3":                    nil,
	} {
		got := audioMeta(t, name, metadata.WithNamespace(metadata.ArtworkNamespace))
		if len(got) != len(want) {
			t.Errorf("%s: expected %v, got %v", name, want, got)
		}
		for _, key := range want {
			if m := got[key]; m == nil || m.Image() == nil {
				t.Errorf("%s: expected the image %s", name, key)
			}
		}
	}
}

func Test_ffmpeg_002(t *testing.T) {
	// A playlist, which has an audio type but is text, isn't read as audio,
	// but as a playlist,
	// and data which isn't audio has its kind, and a note, and no more,
	// which isn't an error
	for _, contentType := range []string{"audio/x-mpegurl", "audio/mpegurl", "audio/x-scpls"} {
		meta, err := getMetadata(context.Background(), strings.NewReader("#EXTM3U\n#EXTINF:123,Artist - Title\nmusic/track01.mp3\n"), contentType)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range meta {
			if m.NS() == "audio" || (key(m) == "metadata:kind" && m.Value() == "audio") {
				t.Errorf("%s: expected no metadata of audio, got %s", contentType, key(m))
			}
		}
	}
	meta, err := getMetadata(context.Background(), strings.NewReader("not audio"), "audio/mpeg")
	if err != nil {
		t.Fatal(err)
	}
	var note bool
	for _, m := range meta {
		switch key(m) {
		case "metadata:kind", "dc:type":
		case "audio:note":
			note = true
		default:
			t.Errorf("expected no metadata, got %s", key(m))
		}
	}
	if !note {
		t.Error("expected a note")
	}
}
