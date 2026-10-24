package tiff_test

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"os"
	"testing"

	// Packages
	tiff "github.com/mutablelogic/go-metadata/internal/tiff"
)

const (
	TEST_DIR = "../../testfiles/"
)

func Test_tiff_000(t *testing.T) {
	// The orientation of an image, which is 1 without one
	for name, want := range map[string]int{
		"PXL_20250102_172041079.MP.jpg": 1,
		"canon_makernote_variant_1.jpg": 6,
		"sample.jpg":                    1,
		"sample.txt":                    1,
	} {
		data, err := os.ReadFile(TEST_DIR + name)
		if err != nil {
			t.Fatal(err)
		}
		if got := tiff.Orientation(data); got != want {
			t.Errorf("%s: expected %d, got %d", name, want, got)
		}
	}
}

func Test_tiff_001(t *testing.T) {
	// The thumbnail of a photo is a JPEG image, and there's none without one
	data, err := os.ReadFile(TEST_DIR + "PXL_20250102_172041079.MP.jpg")
	if err != nil {
		t.Fatal(err)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(tiff.Thumbnail(data)))
	if err != nil {
		t.Fatal(err)
	} else if format != "jpeg" || config.Width != 283 || config.Height != 504 {
		t.Errorf("expected a 283x504 jpeg, got a %dx%d %s", config.Width, config.Height, format)
	}
	for _, data := range [][]byte{nil, []byte("Exif\x00\x00II*\x00"), []byte("Exif\x00\x00II*\x00\xff\xff\xff\xff")} {
		if got := tiff.Thumbnail(data); got != nil {
			t.Errorf("%q: expected no thumbnail, got %d bytes", data, len(got))
		}
		if got := tiff.Orientation(data); got != 1 {
			t.Errorf("%q: expected 1, got %d", data, got)
		}
	}
}

func Test_tiff_002(t *testing.T) {
	// The TIFF data of a RAW image has a TIFF header, and data which isn't
	// TIFF data is nil
	for _, test := range []struct {
		data, want string
	}{
		{"II*\x00\x08\x00\x00\x00", "II*\x00\x08\x00\x00\x00"},
		{"MM\x00*\x00\x00\x00\x08", "MM\x00*\x00\x00\x00\x08"},
		{"IIRO\x08\x00\x00\x00", "II*\x00\x08\x00\x00\x00"},
		{"IIU\x00\x08\x00\x00\x00", "II*\x00\x08\x00\x00\x00"},
		{"MMOR\x00\x00\x00\x08", "MM\x00*\x00\x00\x00\x08"},
		{"FUJIFILMCCD-RAW", ""},
		{"II*", ""},
	} {
		if got := string(tiff.Normalize([]byte(test.data))); got != test.want {
			t.Errorf("%q: expected %q, got %q", test.data, test.want, got)
		}
	}

	// The orientation of an Olympus RAW image, once it's normalized
	data, err := os.ReadFile(TEST_DIR + "RAW_OLYMPUS_E3.ORF")
	if err != nil {
		t.Fatal(err)
	}
	if got := tiff.Orientation(tiff.Normalize(data)); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}
