package exif_test

import (
	"testing"

	// Packages
	_ "github.com/mutablelogic/go-metadata/pkg/exif"
	xmp "github.com/mutablelogic/go-metadata/pkg/xmp"
)

func Test_xmp_000(t *testing.T) {
	// The types of the properties of the exif and tiff schemas of XMP are
	// registered when the package is imported, with or without cgo, and
	// exif:Flash is a struct, whose exif:Fired is a boolean
	for key, want := range map[string]xmp.ValueType{
		"exif:FNumber":          xmp.ValueTypeRational,
		"exif:GPSAltitude":      xmp.ValueTypeRational,
		"exif:GPSLatitude":      xmp.ValueTypeGPSCoord,
		"exif:DateTimeOriginal": xmp.ValueTypeTime,
		"tiff:DateTime":         xmp.ValueTypeTime,
		"exif:Fired":            xmp.ValueTypeBoolean,
		"exif:Flash":            xmp.ValueTypeUnknown,
	} {
		if got := xmp.ValueTypeForKey(key); got != want {
			t.Errorf("%s: expected %v, got %v", key, want, got)
		}
	}
}
