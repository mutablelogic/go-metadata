//go:build cgo

package heif_test

import (
	"bytes"
	"context"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/image/heif"
	_ "github.com/mutablelogic/go-metadata/image/raster"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

// heifMeta returns the metadata of the image in the test files, by key
func heifMeta(t *testing.T, name string, opts ...metadata.Option) map[string]metadata.Metadata {
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

func Test_heif_000(t *testing.T) {
	// The EXIF metadata of a HEIC photo, as for a JPEG photo, and its format
	// and size, which is after it's rotated, from the raster images' handler
	got := heifMeta(t, "photo.HEIC")
	for key, want := range map[string]string{
		"tiff:Make":             "Apple",
		"tiff:Model":            "iPhone 11 Pro",
		"exif:FNumber":          "f/1.8",
		"exif:DateTimeOriginal": "2022-09-19T10:20:02+01:00",
		"dc:date":               "2022-09-19T10:20:02+01:00",
		"exif:GPSPosition":      `51°32'09.71"N 0°08'39.53"W 35.68m`,
		"image:format":          "heif",
		"image:width":           "3024",
		"image:height":          "4032",
		"dc:type":               "StillImage",
		"metadata:kind":         "image",
	} {
		if m, exists := got[key]; !exists {
			t.Errorf("expected %s", key)
		} else if m.Value() != want {
			t.Errorf("%s: expected %q, got %q", key, want, m.Value())
		}
	}
	if v, ok := got["dc:date"].Any().(time.Time); !ok || !v.Equal(time.Date(2022, 9, 19, 9, 20, 2, 0, time.UTC)) {
		t.Errorf("dc:date: expected a time, got %v", got["dc:date"].Any())
	}
	if v, ok := got["dc:coverage"].Any().(geo.LatLong); !ok || v.Long >= 0 {
		t.Errorf("dc:coverage: expected a position in the west, got %v", got["dc:coverage"].Any())
	}
	for _, key := range []string{"exif:GPSLatitude", "exif:GPSLongitudeRef", "exif:OffsetTimeOriginal"} {
		if _, exists := got[key]; exists {
			t.Errorf("expected no %s", key)
		}
	}
}

func Test_heif_001(t *testing.T) {
	// The artwork of a HEIC photo is the thumbnail embedded in it, rather
	// than the photo itself, at most the largest dimension
	for _, size := range []int{1024, 64} {
		got := heifMeta(t, "photo.HEIC", metadata.WithNamespace(metadata.ArtworkNamespace), metadata.WithMaxImageDimension(size))
		if len(got) != 1 || got["artwork:thumbnail"] == nil {
			t.Fatalf("expected artwork:thumbnail, got %v", got)
		}
		img, _, err := image.Decode(bytes.NewReader(got["artwork:thumbnail"].Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		bounds := img.Bounds()
		t.Logf("at most %d: %dx%d", size, bounds.Dx(), bounds.Dy())
		if max(bounds.Dx(), bounds.Dy()) > size || bounds.Dx() > bounds.Dy() {
			t.Errorf("expected a portrait thumbnail at most %d, got %dx%d", size, bounds.Dx(), bounds.Dy())
		} else if size == 1024 && max(bounds.Dx(), bounds.Dy()) > 512 {
			t.Errorf("expected the thumbnail rather than the photo, got %dx%d", bounds.Dx(), bounds.Dy())
		}
	}
}

func Test_heif_002(t *testing.T) {
	// Data which isn't a HEIF image has no metadata from it, which isn't an
	// error
	meta, err := getMetadata(context.Background(), bytes.NewReader([]byte("not a heif image")), "image/heic")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == "exif" || m.NS() == "tiff" || m.NS() == "image" {
			t.Errorf("expected no metadata, got %s", key(m))
		}
	}
}
