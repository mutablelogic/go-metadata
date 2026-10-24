package geo_test

import (
	"encoding/json"
	"math"
	"testing"

	// Packages
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

// near returns true when the positions are within a hundredth of a second
// of each other, with the same altitude, or neither with one
func near(a, b geo.LatLong) bool {
	const second = 1.0 / 3600 / 100
	if math.Abs(a.Lat-b.Lat) > second || math.Abs(a.Long-b.Long) > second {
		return false
	} else if a.Alt == nil || b.Alt == nil {
		return a.Alt == nil && b.Alt == nil
	}
	return *a.Alt == *b.Alt
}

func Test_dms_000(t *testing.T) {
	// A position is degrees, minutes and seconds, with the hemispheres, and
	// the altitude when there's one
	for _, test := range []struct {
		ll   geo.LatLong
		want string
	}{
		{geo.LatLong{Lat: 52.47269722222222, Long: 13.337869444444445}, `52°28'21.71"N 13°20'16.33"E`},
		{geo.LatLong{Lat: -33.8568, Long: -151.2153}, `33°51'24.48"S 151°12'55.08"W`},
		{geo.LatLong{}, `0°00'00.00"N 0°00'00.00"E`},
		{geo.LatLong{Lat: 1.5, Long: 2.25}.WithAlt(91.5), `1°30'00.00"N 2°15'00.00"E 91.5m`},
		{geo.LatLong{Lat: 31.5, Long: 35.5}.WithAlt(-430), `31°30'00.00"N 35°30'00.00"E -430m`},
		{geo.LatLong{Lat: 1.5, Long: 2.25}.WithAlt(35.68202589489718), `1°30'00.00"N 2°15'00.00"E 35.68m`},

		// The seconds which round to 60 carry into the minutes and degrees
		{geo.LatLong{Lat: 59.999999999, Long: -0.0000001}, `60°00'00.00"N 0°00'00.00"W`},
	} {
		if got := test.ll.String(); got != test.want {
			t.Errorf("%v: expected %s, got %s", test.ll, test.want, got)
		}
	}
}

func Test_dms_001(t *testing.T) {
	// Text in degrees, minutes and seconds is parsed, with the hemispheres
	// before or after, minutes and seconds optional, typographic primes,
	// and a comma or spaces between
	for _, test := range []struct {
		text string
		want geo.LatLong
	}{
		{`52°28'21.71"N 13°20'16.33"E`, geo.LatLong{Lat: 52.47269722, Long: 13.33786944}},
		{`52°28'21.71"N, 13°20'16.33"E`, geo.LatLong{Lat: 52.47269722, Long: 13.33786944}},
		{`52° 28′ 21.71″ N 13° 20′ 16.33″ E`, geo.LatLong{Lat: 52.47269722, Long: 13.33786944}},
		{`N52°28'21.71" E13°20'16.33"`, geo.LatLong{Lat: 52.47269722, Long: 13.33786944}},
		{`33°51'24.48"S 151°12'55.08"W`, geo.LatLong{Lat: -33.8568, Long: -151.2153}},
		{`33°51'24.48"s 151°12'55.08"w`, geo.LatLong{Lat: -33.8568, Long: -151.2153}},
		{`52°28.362'N 13°20.272'E`, geo.LatLong{Lat: 52.4727, Long: 13.337866667}},
		{`52.4727°N 13.3379°E`, geo.LatLong{Lat: 52.4727, Long: 13.3379}},
		{`-33.8568° -151.2153°`, geo.LatLong{Lat: -33.8568, Long: -151.2153}},
		{`52º28'21.71''N 13º20'16.33''E`, geo.LatLong{Lat: 52.47269722, Long: 13.33786944}},
		{`1°30'N 2°15'E 91.5m`, geo.LatLong{Lat: 1.5, Long: 2.25}.WithAlt(91.5)},
		{`31°30'N, 35°30'E, -430 m`, geo.LatLong{Lat: 31.5, Long: 35.5}.WithAlt(-430)},
		{`1°30'N 2°15'E 91.5`, geo.LatLong{Lat: 1.5, Long: 2.25}.WithAlt(91.5)},
	} {
		got, err := geo.Parse(test.text)
		if err != nil {
			t.Errorf("%s: %v", test.text, err)
		} else if !near(got, test.want) {
			t.Errorf("%s: expected %v, got %v", test.text, test.want, got)
		}
	}
}

func Test_dms_002(t *testing.T) {
	// Text which isn't a position in degrees, or is off the Earth, is an
	// error
	for _, text := range []string{
		`52°28'21.71"N`,                    // no longitude
		`52°28'21.71"E 13°20'16.33"N`,      // the hemispheres swapped
		`N52°28'21.71"N 13°20'16.33"E`,     // two hemispheres
		`-52°28'21.71"N 13°20'16.33"E`,     // a hemisphere and a sign
		`52°60'00"N 13°20'16.33"E`,         // 60 minutes
		`52°28'60"N 13°20'16.33"E`,         // 60 seconds
		`52.5°28'N 13°20'E`,                // decimals before the last part
		`52°28.5'21"N 13°20'E`,             // decimals before the last part
		`91°00'00"N 13°20'16.33"E`,         // off the Earth
		`52°28'21.71"N 181°00'00"E`,        // off the Earth
		`52°28'21.71"N 13°20'16.33"E high`, // not an altitude
		`52°28'21.71"N 13°20'16.33"E 1 2`,  // two altitudes
	} {
		if got, err := geo.Parse(text); err == nil {
			t.Errorf("%s: expected an error, got %v", text, got)
		}
	}
}

func Test_dms_003(t *testing.T) {
	// A position is the same in degrees, minutes and seconds, and back, to
	// a hundredth of a second
	for _, ll := range []geo.LatLong{
		{Lat: 52.47269722222222, Long: 13.337869444444445},
		{Lat: -33.8568, Long: -151.2153},
		{Lat: -90, Long: 180},
		{Lat: 0.00001, Long: -0.00001},
		geo.LatLong{Lat: 52.47269722222222, Long: 13.337869444444445}.WithAlt(91.5),
	} {
		got, err := geo.Parse(ll.String())
		if err != nil {
			t.Errorf("%s: %v", ll.String(), err)
		} else if !near(got, ll) {
			t.Errorf("%s: expected %v, got %v", ll.String(), ll, got)
		}
	}
}

func Test_dms_004(t *testing.T) {
	// JSON can be a string in degrees, minutes and seconds, but is
	// marshalled as decimal degrees
	var got geo.LatLong
	if err := json.Unmarshal([]byte(`"1°30'N 2°15'W"`), &got); err != nil {
		t.Fatal(err)
	} else if !got.Equal(geo.LatLong{Lat: 1.5, Long: -2.25}) {
		t.Errorf("expected 1.5,-2.25, got %v", got)
	}
	if data, err := json.Marshal(got); err != nil {
		t.Fatal(err)
	} else if string(data) != `{"lat":1.5,"long":-2.25}` {
		t.Errorf("expected decimal degrees, got %s", data)
	}
}

func Test_dms_005(t *testing.T) {
	// Text is in degrees, minutes and seconds, and decimal degrees on request
	ll := geo.LatLong{Lat: 52.47269722222222, Long: 13.337869444444445}.WithAlt(91.5)
	if text, err := ll.MarshalText(); err != nil {
		t.Fatal(err)
	} else if string(text) != `52°28'21.71"N 13°20'16.33"E 91.5m` {
		t.Errorf("expected degrees, minutes and seconds, got %s", text)
	}
	if got := ll.Decimal(); got != "52.47269722222222,13.337869444444445,91.5" {
		t.Errorf("expected decimal degrees, got %s", got)
	}
}
