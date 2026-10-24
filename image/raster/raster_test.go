package raster_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/image/raster"
)

// Test_image_metadata_000 checks that the generic image handler returns the
// correct format, width and height for each of the image formats it
// registers decoders for.
func Test_image_metadata_000(t *testing.T) {
	tests := []struct {
		file   string
		format string
		width  int
		height int
	}{
		{"sample.gif", "gif", 64, 48},
		{"sample.bmp", "bmp", 64, 48},
		{"sample.tiff", "tiff", 64, 48},
		{"sample.jpg", "jpeg", 640, 360},
		{"sample.png", "png", 1280, 720},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			path := filepath.Join(TEST_DIR, test.file)
			contentType := contentTypeForFile(t, path)

			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			meta, err := getMetadata(context.Background(), f, contentType, metadata.WithNamespace("image"))
			if err != nil {
				t.Fatal(err)
			}

			got := make(map[string]string, len(meta))
			for _, m := range meta {
				got[key(m)] = m.Value()
				t.Logf("%s = %v", key(m), m.Value())
			}

			if got["image:format"] != test.format {
				t.Errorf("image:format = %q, want %q", got["image:format"], test.format)
			}
			if want := strconv.Itoa(test.width); got["image:width"] != want {
				t.Errorf("image:width = %q, want %q", got["image:width"], want)
			}
			if want := strconv.Itoa(test.height); got["image:height"] != want {
				t.Errorf("image:height = %q, want %q", got["image:height"], want)
			}
		})
	}
}

// Test_image_metadata_001 checks that an image which there's no decoder for,
// such as an icon, has no image metadata, rather than an error
func Test_image_metadata_001(t *testing.T) {
	icon := strings.NewReader("\x00\x00\x01\x00\x01\x00\x10\x10")
	meta, err := getMetadata(context.Background(), icon, "image/x-icon")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == "image" {
			t.Fatalf("expected no image metadata, got %v", key(m))
		}
	}
}
