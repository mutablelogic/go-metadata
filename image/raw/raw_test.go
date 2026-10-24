//go:build cgo

package raw_test

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
	_ "github.com/mutablelogic/go-metadata/image/raster"
	_ "github.com/mutablelogic/go-metadata/image/raw"
)

const testRAW = "RAW_OLYMPUS_E3.ORF"

// rawMeta returns the metadata of the RAW image, by key, and the number of
// each key
func rawMeta(t *testing.T, data []byte, contentType string, opts ...metadata.Option) (map[string]metadata.Metadata, map[string]int) {
	t.Helper()
	meta, err := getMetadata(context.Background(), bytes.NewReader(data), contentType, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result, count := make(map[string]metadata.Metadata, len(meta)), make(map[string]int, len(meta))
	for _, m := range meta {
		result[key(m)] = m
		count[key(m)]++
	}
	return result, count
}

func Test_raw_000(t *testing.T) {
	// The EXIF metadata of a RAW image, from the image itself, its format
	// and size, and its kind, once
	path := filepath.Join(TEST_DIR, testRAW)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contentType := contentTypeForFile(t, path)
	if contentType != "image/x-olympus-orf" {
		t.Fatalf("expected image/x-olympus-orf, got %q", contentType)
	}
	got, count := rawMeta(t, data, contentType)
	for key, want := range map[string]string{
		"tiff:Make":             "OLYMPUS IMAGING CORP.",
		"tiff:Model":            "E-3",
		"exif:FNumber":          "f/2.0",
		"exif:ExposureTime":     "1/320 sec.",
		"exif:DateTimeOriginal": "2008-12-19T12:29:40Z",
		"dc:date":               "2008-12-19T12:29:40Z",
		"image:format":          "orf",
		"image:width":           "3720",
		"image:height":          "2800",
		"metadata:kind":         "image",
		"dc:type":               "StillImage",
	} {
		if m, exists := got[key]; !exists {
			t.Errorf("expected %s", key)
		} else if m.Value() != want {
			t.Errorf("%s: expected %q, got %q", key, want, m.Value())
		} else if count[key] != 1 {
			t.Errorf("%s: expected once, got %d times", key, count[key])
		}
	}
	if v, ok := got["dc:date"].Any().(time.Time); !ok || !v.Equal(time.Date(2008, 12, 19, 12, 29, 40, 0, time.UTC)) {
		t.Errorf("dc:date: expected a time, got %v", got["dc:date"].Any())
	}
}

func Test_raw_001(t *testing.T) {
	// The artwork of a RAW image is its preview, at most the largest
	// dimension, and a JPEG image, as the preview is
	data, err := os.ReadFile(filepath.Join(TEST_DIR, testRAW))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := rawMeta(t, data, "image/x-olympus-orf", metadata.WithNamespace(metadata.ArtworkNamespace))
	if len(got) != 1 || got["artwork:thumbnail"] == nil {
		t.Fatalf("expected artwork:thumbnail, got %v", got)
	}
	img, format, err := image.Decode(bytes.NewReader(got["artwork:thumbnail"].Bytes()))
	if err != nil {
		t.Fatal(err)
	} else if format != "jpeg" || img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 768 {
		t.Errorf("expected a 1024x768 jpeg, got a %v %s", img.Bounds(), format)
	}
}

func Test_raw_002(t *testing.T) {
	// Data which isn't a RAW image has no metadata from it, which isn't an
	// error
	got, _ := rawMeta(t, []byte("not a raw image"), "image/x-olympus-orf")
	for k := range got {
		if k != "metadata:kind" && k != "dc:type" {
			t.Errorf("expected no metadata, got %s", k)
		}
	}
}
