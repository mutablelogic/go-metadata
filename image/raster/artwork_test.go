package raster_test

import (
	"bytes"
	"context"
	"image"
	"os"
	"path/filepath"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/image/raster"
)

// Test_artwork_000 checks that requesting the "artwork" namespace returns a
// single, valid artwork entry.
func Test_artwork_000(t *testing.T) {
	path := filepath.Join(TEST_DIR, "sample.jpg")
	contentType := contentTypeForFile(t, path)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	meta, err := getMetadata(context.Background(), f, contentType, metadata.WithNamespace("artwork"))
	if err != nil {
		t.Fatal(err)
	}
	if len(meta) != 1 {
		t.Fatalf("expected 1 artwork entry, got %d", len(meta))
	}
	assertArtwork(t, meta[0])
}

// Test_artwork_001 checks that namespaces other than "artwork" (including
// no namespace filter at all) don't trigger artwork extraction.
func Test_artwork_001(t *testing.T) {
	for _, namespace := range []string{"", "tiff", "image"} {
		t.Run(namespace, func(t *testing.T) {
			path := filepath.Join(TEST_DIR, "sample.jpg")
			contentType := contentTypeForFile(t, path)

			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			var opts []metadata.Option
			if namespace != "" {
				opts = append(opts, metadata.WithNamespace(namespace))
			}

			meta, err := getMetadata(context.Background(), f, contentType, opts...)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range meta {
				if key(m) == "artwork:thumbnail" {
					t.Fatalf("did not expect artwork:thumbnail for namespace %q", namespace)
				}
			}
		})
	}
}

// Test_artwork_002 checks that an already-jpeg image near the target width
// is returned unchanged (the fast path), byte-for-byte identical to the
// source file, rather than being needlessly re-encoded.
func Test_artwork_002(t *testing.T) {
	path := filepath.Join(TEST_DIR, "sample.jpg")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contentType := contentTypeForFile(t, path)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	meta, err := getMetadata(context.Background(), f, contentType, metadata.WithNamespace("artwork"))
	if err != nil {
		t.Fatal(err)
	}
	if len(meta) != 1 {
		t.Fatalf("expected 1 artwork entry, got %d", len(meta))
	}
	if meta[0].Value() != "image/jpeg" {
		t.Errorf("Value() = %q, want %q", meta[0].Value(), "image/jpeg")
	}
	if !bytes.Equal(meta[0].Bytes(), want) {
		t.Error("expected the fast path to return the original file bytes unchanged")
	}
}

// Test_artwork_003 checks that an image larger than the largest image
// dimension is resized down to it, preserving aspect ratio, and re-encoded
// as PNG, as it's a PNG image.
func Test_artwork_003(t *testing.T) {
	path := filepath.Join(TEST_DIR, "sample.png")
	contentType := contentTypeForFile(t, path)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	meta, err := getMetadata(context.Background(), f, contentType, metadata.WithNamespace("artwork"), metadata.WithMaxImageDimension(640))
	if err != nil {
		t.Fatal(err)
	}
	if len(meta) != 1 {
		t.Fatalf("expected 1 artwork entry, got %d", len(meta))
	}
	if meta[0].Value() != "image/png" {
		t.Errorf("Value() = %q, want %q", meta[0].Value(), "image/png")
	}

	img := assertArtwork(t, meta[0])
	if got := img.Bounds().Dx(); got > 640 {
		t.Errorf("width = %d, want <= 640", got)
	}
	// sample.png is 1280x720 (16:9); the resize should preserve that ratio
	if got, want := img.Bounds().Dy(), img.Bounds().Dx()*720/1280; got != want {
		t.Errorf("height = %d, want %d (aspect ratio not preserved)", got, want)
	}
}

// Test_artwork_004 checks that small non-jpeg/png images (gif, bmp, tiff)
// are still re-encoded to PNG, even though they're already smaller than the
// largest image dimension and don't need resizing.
func Test_artwork_004(t *testing.T) {
	for _, file := range []string{"sample.gif", "sample.bmp", "sample.tiff"} {
		t.Run(file, func(t *testing.T) {
			path := filepath.Join(TEST_DIR, file)
			contentType := contentTypeForFile(t, path)

			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			meta, err := getMetadata(context.Background(), f, contentType, metadata.WithNamespace("artwork"))
			if err != nil {
				t.Fatal(err)
			}
			if len(meta) != 1 {
				t.Fatalf("expected 1 artwork entry, got %d", len(meta))
			}
			if meta[0].Value() != "image/png" {
				t.Errorf("Value() = %q, want %q", meta[0].Value(), "image/png")
			}
			assertArtwork(t, meta[0])
		})
	}
}

// assertArtwork checks that an artwork metadata entry has a decodable,
// non-empty Bytes() payload and a non-nil Image() with matching bounds,
// and returns the decoded image.
func assertArtwork(t *testing.T, m metadata.Metadata) image.Image {
	t.Helper()

	if key(m) != "artwork:thumbnail" {
		t.Errorf("Key() = %q, want %q", key(m), "artwork:thumbnail")
	}

	data := m.Bytes()
	if len(data) == 0 {
		t.Fatal("Bytes() is empty")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Bytes() did not decode: %v", err)
	}

	img := m.Image()
	if img == nil {
		t.Fatal("Image() returned nil")
	}
	if img.Bounds() != decoded.Bounds() {
		t.Errorf("Image().Bounds() = %v, decoded Bytes() bounds = %v", img.Bounds(), decoded.Bounds())
	}
	return img
}

func Test_artwork_orientation_000(t *testing.T) {
	// The artwork of a photo is transformed by its orientation, as its EXIF
	// data isn't kept when it's re-encoded, so a portrait photo which is
	// rotated 90° is landscape
	data, err := os.ReadFile(filepath.Join(TEST_DIR, "PXL_20250102_172041079.MP.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	entry := []byte{0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00}
	i := bytes.Index(data, entry)
	if i < 0 {
		t.Fatal("expected the orientation entry")
	}
	original := data
	data = bytes.Clone(data)
	data[i+8] = 6

	meta, err := getMetadata(context.Background(), bytes.NewReader(data), "image/jpeg", metadata.WithNamespace(metadata.ArtworkNamespace))
	if err != nil {
		t.Fatal(err)
	} else if len(meta) != 1 {
		t.Fatalf("expected one artwork, got %d", len(meta))
	}
	img, _, err := image.Decode(bytes.NewReader(meta[0].Bytes()))
	if err != nil {
		t.Fatal(err)
	} else if img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 576 {
		t.Errorf("expected 1024x576, got %v", img.Bounds())
	}

	// The width and height are as it's displayed, too, and as it's stored
	// without the orientation
	for _, test := range []struct {
		data          []byte
		width, height string
	}{
		{data, "4032", "2268"},
		{original, "2268", "4032"},
	} {
		meta, err := getMetadata(context.Background(), bytes.NewReader(test.data), "image/jpeg")
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, m := range meta {
			got[key(m)] = m.Value()
		}
		if got["image:width"] != test.width || got["image:height"] != test.height {
			t.Errorf("expected %sx%s, got %sx%s", test.width, test.height, got["image:width"], got["image:height"])
		}
	}
}
