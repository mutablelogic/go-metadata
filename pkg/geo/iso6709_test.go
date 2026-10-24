package geo_test

import (
	"testing"

	// Packages
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

func Test_iso6709_000(t *testing.T) {
	// A position in ISO 6709 is the latitude, longitude and optionally the
	// altitude, with their signs, and the solidus is optional
	for _, test := range []struct {
		text string
		want geo.LatLong
	}{
		{"+52.4158+013.5896+035.667/", geo.LatLong{Lat: 52.4158, Long: 13.5896}.WithAlt(35.667)},
		{"-33.8568+151.2153/", geo.LatLong{Lat: -33.8568, Long: 151.2153}},
		{"+40.7128-074.0060", geo.LatLong{Lat: 40.7128, Long: -74.006}},
		{"+31.5+035.5-430CRSWGS_84/", geo.LatLong{Lat: 31.5, Long: 35.5}.WithAlt(-430)},
	} {
		got, err := geo.ParseISO6709(test.text)
		if err != nil {
			t.Errorf("%q: %v", test.text, err)
		} else if !got.Equal(test.want) {
			t.Errorf("%q: expected %v, got %v", test.text, test.want, got)
		}
	}
	for _, text := range []string{"", "52.4158,13.5896", "+91.0+000.0/", "+00.0+181.0/"} {
		if got, err := geo.ParseISO6709(text); err == nil {
			t.Errorf("%q: expected an error, got %v", text, got)
		}
	}
}
