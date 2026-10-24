//go:build cgo

package ffmpeg_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/video/ffmpeg"
)

// videoMeta returns the metadata of the video in the test files, by key
func videoMeta(t *testing.T, name string, opts ...metadata.Option) map[string]metadata.Metadata {
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
	// The title, duration, streams and kind of a video, without a creation
	// time which is the Unix epoch
	got := videoMeta(t, "sample.mp4")
	for key, want := range map[string]string{
		"dc:title":          "Sample From Big Buck Bunny",
		"video:codec":       "h264",
		"video:width":       "1280",
		"video:height":      "720",
		"audio:codec":       "aac",
		"audio:sample_rate": "48000",
		"audio:channels":    "6",
		"metadata:kind":     "video",
		"dc:type":           "MovingImage",
	} {
		if m := got[key]; m == nil || m.Value() != want {
			t.Errorf("%s: expected %q, got %v", key, want, m)
		}
	}
	if d, ok := got["video:duration"].Any().(time.Duration); !ok || d.Round(time.Millisecond) != 5312*time.Millisecond {
		t.Errorf("video:duration: expected 5.312s, got %v", got["video:duration"])
	}
	for _, key := range []string{"dc:date", "video:creation-time"} {
		if got[key] != nil {
			t.Errorf("expected no %s, got %v", key, got[key])
		}
	}
}

func Test_ffmpeg_001(t *testing.T) {
	// The cover art of a video is its artwork, the first as artwork:cover,
	// and the others numbered, and a video without any has none
	for name, want := range map[string][]string{
		"sample_with_artwork.mp4":       {"artwork:cover"},
		"sample_with_multi_artwork.mp4": {"artwork:cover", "artwork:cover-2"},
		"sample.mp4":                    nil,
	} {
		got := videoMeta(t, name, metadata.WithNamespace(metadata.ArtworkNamespace))
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
	// Data which isn't a video has its kind, and no more, which isn't an
	// error
	meta, err := getMetadata(context.Background(), bytesReader("not a video"), "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if key(m) != "metadata:kind" && key(m) != "dc:type" {
			t.Errorf("expected no metadata, got %s", key(m))
		}
	}
}
