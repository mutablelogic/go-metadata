//go:build cgo

package exif_test

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/image/exif"
	_ "github.com/mutablelogic/go-metadata/image/raster"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

// exifMeta returns the metadata of the image in the test files, by key
func exifMeta(t *testing.T, name string) map[string]metadata.Metadata {
	t.Helper()
	path := filepath.Join(TEST_DIR, name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	meta, err := getMetadata(context.Background(), f, contentTypeForFile(t, path))
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]metadata.Metadata, len(meta))
	for _, m := range meta {
		result[key(m)] = m
	}
	return result
}

func Test_exif_000(t *testing.T) {
	// The camera, the exposure, and the position of a photo, with its date
	// and time with its UTC offset, as dc:date
	got := exifMeta(t, "PXL_20250102_172041079.MP.jpg")
	for key, want := range map[string]string{
		"tiff:Make":             "Google",
		"tiff:Model":            "Pixel 8",
		"exif:FNumber":          "f/1.7",
		"exif:ExposureTime":     "1/80 sec.",
		"exif:GPSPosition":      `52°28'21.71"N 13°20'16.33"E 91.5m`,
		"exif:DateTimeOriginal": "2025-01-02T18:20:41+01:00",
		"dc:date":               "2025-01-02T18:20:41+01:00",
		"dc:coverage":           `52°28'21.71"N 13°20'16.33"E 91.5m`,
	} {
		if m, exists := got[key]; !exists {
			t.Errorf("expected %s", key)
		} else if m.Value() != want {
			t.Errorf("%s: expected %q, got %q", key, want, m.Value())
		}
	}

	// A date and time is a time, and a rational is a number
	if v, ok := got["dc:date"].Any().(time.Time); !ok || !v.Equal(time.Date(2025, 1, 2, 17, 20, 41, 0, time.UTC)) {
		t.Errorf("dc:date: expected a time, got %v", got["dc:date"].Any())
	}
	if v, ok := got["exif:ExposureTime"].Any().(float64); !ok || v != 0.012445 {
		t.Errorf("exif:ExposureTime: expected 0.012445, got %v", got["exif:ExposureTime"].Any())
	}
	if v, ok := got["exif:GPSPosition"].Any().(geo.LatLong); !ok || math.Abs(v.Lat-52.472697) > 1e-6 || math.Abs(v.Long-13.337869) > 1e-6 || v.Alt == nil || *v.Alt != 91.5 {
		t.Errorf("exif:GPSPosition: expected 52.472697,13.337869,91.5, got %v", got["exif:GPSPosition"].Any())
	}

	// The offsets are merged in, and the position replaces the tags of its
	// latitude, longitude and altitude
	for _, key := range []string{"exif:OffsetTimeOriginal", "exif:GPSLatitude", "exif:GPSLatitudeRef", "exif:GPSLongitude", "exif:GPSLongitudeRef", "exif:GPSAltitude", "exif:GPSAltitudeRef"} {
		if _, exists := got[key]; exists {
			t.Errorf("expected no %s", key)
		}
	}
}

func Test_exif_001(t *testing.T) {
	// A JPEG image without EXIF metadata has none, which isn't an error
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	meta, err := getMetadata(context.Background(), &buf, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == "exif" || m.NS() == "tiff" || key(m) == "dc:date" {
			t.Errorf("expected no EXIF metadata, got %s", key(m))
		}
	}

	// A JPEG image whose EXIF metadata is after its JFIF header has it
	if got := exifMeta(t, "sample.jpg"); got["tiff:Orientation"] == nil {
		t.Error("expected tiff:Orientation")
	}
}

func Test_exif_002(t *testing.T) {
	// Photos with the maker notes of cameras have the make, and the date,
	// in UTC when they have no UTC offset, unless the camera's clock wasn't
	// set, when the dates are unknown and there are none
	files, err := filepath.Glob(filepath.Join(TEST_DIR, "*_makernote_*.jpg"))
	if err != nil {
		t.Fatal(err)
	} else if len(files) == 0 {
		t.Fatal("expected photos with maker notes")
	}
	for _, path := range files {
		name := filepath.Base(path)
		got := exifMeta(t, name)
		if _, exists := got["dc:coverage"]; exists {
			t.Errorf("%s: expected no dc:coverage, without a GPS position", name)
		}
		if m, exists := got["tiff:Make"]; !exists || m.Value() == "" {
			t.Errorf("%s: expected tiff:Make", name)
		}
		if name == "olympus_makernote_variant_4.jpg" {
			for _, key := range []string{"dc:date", "tiff:DateTime", "exif:DateTimeOriginal", "exif:DateTimeDigitized"} {
				if _, exists := got[key]; exists {
					t.Errorf("%s: expected no %s", name, key)
				}
			}
		} else if m, exists := got["dc:date"]; !exists {
			t.Errorf("%s: expected dc:date", name)
		} else if v, ok := m.Any().(time.Time); !ok || v.Location() != time.UTC {
			t.Errorf("%s: expected dc:date in UTC, got %v", name, m.Any())
		}
	}
}

func Test_exif_003(t *testing.T) {
	// Only the namespaces which are requested are returned
	path := filepath.Join(TEST_DIR, "PXL_20250102_172041079.MP.jpg")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	meta, err := getMetadata(context.Background(), f, "image/jpeg", metadata.WithNamespace("tiff"))
	if err != nil {
		t.Fatal(err)
	} else if len(meta) == 0 {
		t.Fatal("expected tiff metadata")
	}
	for _, m := range meta {
		if m.NS() != "tiff" {
			t.Errorf("expected only tiff, got %s", key(m))
		}
	}
}

func Test_exif_004(t *testing.T) {
	// The artwork of a photo is the thumbnail embedded in its EXIF data,
	// rather than the photo itself, with the raster images' handlers too
	path := filepath.Join(TEST_DIR, "PXL_20250102_172041079.MP.jpg")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	meta, err := getMetadata(context.Background(), f, "image/jpeg", metadata.WithNamespace(metadata.ArtworkNamespace))
	if err != nil {
		t.Fatal(err)
	} else if len(meta) != 1 || key(meta[0]) != "artwork:thumbnail" {
		t.Fatalf("expected artwork:thumbnail, got %v", meta)
	}
	img, _, err := image.Decode(bytes.NewReader(meta[0].Bytes()))
	if err != nil {
		t.Fatal(err)
	} else if img.Bounds().Dx() != 283 || img.Bounds().Dy() != 504 {
		t.Errorf("expected the 283x504 thumbnail, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func Test_exif_007(t *testing.T) {
	// A thumbnail which is smaller than the smallest isn't the artwork, so
	// the photo itself, downscaled, is, which is larger, and zero uses any
	// thumbnail
	path := filepath.Join(TEST_DIR, "PXL_20250102_172041079.MP.jpg")
	for _, test := range []struct {
		min      int
		longest  int
		embedded bool
	}{
		{600, 1024, false},
		{504, 504, true},
		{0, 504, true},
	} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		meta, err := getMetadata(context.Background(), f, "image/jpeg", metadata.WithNamespace(metadata.ArtworkNamespace), metadata.WithMinImageDimension(test.min))
		f.Close()
		if err != nil {
			t.Fatal(err)
		} else if len(meta) != 1 || key(meta[0]) != "artwork:thumbnail" {
			t.Fatalf("min %d: expected artwork:thumbnail, got %v", test.min, meta)
		}
		img, _, err := image.Decode(bytes.NewReader(meta[0].Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if longest := max(img.Bounds().Dx(), img.Bounds().Dy()); longest != test.longest {
			t.Errorf("min %d: expected the longest side to be %d, got %d", test.min, test.longest, longest)
		}
	}

	// The smallest can't be negative
	if _, err := getMetadata(context.Background(), bytes.NewReader(nil), "image/jpeg", metadata.WithMinImageDimension(-1)); err == nil {
		t.Error("expected an error for a negative min image dimension")
	}
}

func Test_exif_005(t *testing.T) {
	// The artwork of a photo without a thumbnail is the photo itself, from
	// the raster images' handler, and a photo whose thumbnail is cut short,
	// and whose image data has been removed, has none
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 640, 480)), nil); err != nil {
		t.Fatal(err)
	}
	stripped, err := os.ReadFile(filepath.Join(TEST_DIR, "canon_makernote_variant_1.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		data  []byte
		width int
	}{
		{"no exif", buf.Bytes(), 640},
		{"cut short", stripped, 0},
	} {
		meta, err := getMetadata(context.Background(), bytes.NewReader(test.data), "image/jpeg", metadata.WithNamespace(metadata.ArtworkNamespace))
		if err != nil {
			t.Fatal(err)
		}
		if test.width == 0 {
			if len(meta) != 0 {
				t.Errorf("%s: expected no artwork, got %d", test.name, len(meta))
			}
			continue
		} else if len(meta) != 1 {
			t.Fatalf("%s: expected one artwork, got %d", test.name, len(meta))
		}
		img, _, err := image.Decode(bytes.NewReader(meta[0].Bytes()))
		if err != nil {
			t.Fatal(err)
		} else if img.Bounds().Dx() != test.width {
			t.Errorf("%s: expected a width of %d, got %d", test.name, test.width, img.Bounds().Dx())
		}
	}
}

func Test_exif_006(t *testing.T) {
	// The thumbnail of a photo is transformed by the orientation of the
	// photo, as it has no EXIF data of its own, so a portrait thumbnail of a
	// photo which is rotated 90° is landscape
	data, err := os.ReadFile(filepath.Join(TEST_DIR, "PXL_20250102_172041079.MP.jpg"))
	if err != nil {
		t.Fatal(err)
	}

	// The entry of IFD0 which is the orientation, 1, as a little-endian
	// SHORT, is set to 6
	entry := []byte{0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00}
	i := bytes.Index(data, entry)
	if i < 0 {
		t.Fatal("expected the orientation entry")
	}
	rotated := bytes.Clone(data)
	rotated[i+8] = 6

	for _, test := range []struct {
		name          string
		data          []byte
		width, height int
	}{
		{"as it is", data, 283, 504},
		{"rotated", rotated, 504, 283},
	} {
		meta, err := getMetadata(context.Background(), bytes.NewReader(test.data), "image/jpeg", metadata.WithNamespace(metadata.ArtworkNamespace))
		if err != nil {
			t.Fatal(err)
		} else if len(meta) != 1 {
			t.Fatalf("%s: expected one artwork, got %d", test.name, len(meta))
		}
		img, _, err := image.Decode(bytes.NewReader(meta[0].Bytes()))
		if err != nil {
			t.Fatal(err)
		} else if img.Bounds().Dx() != test.width || img.Bounds().Dy() != test.height {
			t.Errorf("%s: expected %dx%d, got %v", test.name, test.width, test.height, img.Bounds())
		}
	}
}
