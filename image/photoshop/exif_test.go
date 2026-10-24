//go:build cgo

package photoshop_test

import (
	"testing"

	// Packages
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

func Test_photoshop_exif_000(t *testing.T) {
	// The EXIF metadata of a document, as the JPEG handler returns it, with
	// the date and position it was taken
	got := psdMeta(t, sample(t))
	for key, want := range map[string]string{
		"tiff:Make":             "Google",
		"tiff:Model":            "Pixel 8",
		"exif:FNumber":          "f/1.7",
		"exif:DateTimeOriginal": "2025-01-02T18:20:41+01:00",
		"dc:date":               "2025-01-02T18:20:41+01:00",
	} {
		if m, exists := got[key]; !exists {
			t.Errorf("expected %s", key)
		} else if m.Value() != want {
			t.Errorf("%s: expected %q, got %q", key, want, m.Value())
		}
	}
	if _, ok := got["exif:GPSPosition"].Any().(geo.LatLong); !ok {
		t.Errorf("expected exif:GPSPosition, got %v", got["exif:GPSPosition"])
	}
}
