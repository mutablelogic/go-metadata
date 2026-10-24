package geo_test

import (
	"maps"
	"testing"

	// Packages
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

func Test_xmp_000(t *testing.T) {
	// A position in the exif namespace is the GPS properties of the exif
	// schema of XMP, with an altitude, below sea level, when there's one
	for _, test := range []struct {
		ll   geo.LatLong
		want map[string]string
	}{
		{geo.LatLong{Lat: 53.234219, Long: 10.206025}, map[string]string{
			"exif:GPSLatitude":  "53,14.053140N",
			"exif:GPSLongitude": "10,12.361500E",
		}},
		{geo.LatLong{Lat: -33.8568, Long: -151.2153}.WithAlt(35.682), map[string]string{
			"exif:GPSLatitude":    "33,51.408000S",
			"exif:GPSLongitude":   "151,12.918000W",
			"exif:GPSAltitude":    "3568/100",
			"exif:GPSAltitudeRef": "0",
		}},
		{geo.LatLong{Lat: 31.5, Long: 35.5}.WithAlt(-430), map[string]string{
			"exif:GPSLatitude":    "31,30.000000N",
			"exif:GPSLongitude":   "35,30.000000E",
			"exif:GPSAltitude":    "43000/100",
			"exif:GPSAltitudeRef": "1",
		}},

		// Minutes which round to 60 carry into the degrees
		{geo.LatLong{Lat: 59.9999999999, Long: 0}, map[string]string{
			"exif:GPSLatitude":  "60,0.000000N",
			"exif:GPSLongitude": "0,0.000000E",
		}},
	} {
		if got := test.ll.MarshalXMP("exif:GPSPosition"); !maps.Equal(got, test.want) {
			t.Errorf("%v: expected %v, got %v", test.ll, test.want, got)
		}
	}

	// In another namespace, or off the Earth, it's none, so it's its text
	if got := (geo.LatLong{Lat: 1, Long: 2}).MarshalXMP("dc:coverage"); got != nil {
		t.Errorf("expected none, got %v", got)
	}
	if got := (geo.LatLong{Lat: 91}).MarshalXMP("exif:GPSPosition"); got != nil {
		t.Errorf("expected none, got %v", got)
	}
}
