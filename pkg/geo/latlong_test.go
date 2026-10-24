package geo_test

import (
	"encoding/json"
	"math"
	"testing"

	// Packages
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

func Test_latlong_000(t *testing.T) {
	// Text is the latitude and longitude, separated by a comma, with or
	// without spaces
	for _, test := range []struct {
		text string
		want geo.LatLong
	}{
		{"52.472697,13.337869", geo.LatLong{Lat: 52.472697, Long: 13.337869}},
		{" -33.8568 , 151.2153 ", geo.LatLong{Lat: -33.8568, Long: 151.2153}},
		{"0,0", geo.LatLong{}},
		{"90,-180", geo.LatLong{Lat: 90, Long: -180}},
		{"52.472697,13.337869,91.5", geo.LatLong{Lat: 52.472697, Long: 13.337869}.WithAlt(91.5)},
		{"31.5, 35.5, -430", geo.LatLong{Lat: 31.5, Long: 35.5}.WithAlt(-430)},
		{"0,0,0", geo.LatLong{}.WithAlt(0)},
	} {
		got, err := geo.Parse(test.text)
		if err != nil {
			t.Errorf("%q: %v", test.text, err)
		} else if !got.Equal(test.want) {
			t.Errorf("%q: expected %v, got %v", test.text, test.want, got)
		}
	}
}

func Test_latlong_001(t *testing.T) {
	// Text which isn't a position, or is off the Earth, is an error
	for _, text := range []string{"", "52.47", "52.47,", ",13.34", "north,east", "90.1,0", "0,180.1", "NaN,0", "1,2,3,4", "1,2,", "1,2,high", "1,2,Inf"} {
		if got, err := geo.Parse(text); err == nil {
			t.Errorf("%q: expected an error, got %v", text, got)
		}
	}
}

func Test_latlong_002(t *testing.T) {
	// A position is the same as text, and back, to a hundredth of a second,
	// and exactly in decimal degrees
	for _, ll := range []geo.LatLong{
		{Lat: 52.47269722222222, Long: 13.337869444444445},
		{Lat: -33.8568, Long: -151.2153},
		{},
		geo.LatLong{Lat: 52.47269722222222, Long: 13.337869444444445}.WithAlt(91.5),
		geo.LatLong{}.WithAlt(0),
	} {
		text, err := ll.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var got geo.LatLong
		if err := got.UnmarshalText(text); err != nil {
			t.Errorf("%s: %v", text, err)
		} else if !near(got, ll) {
			t.Errorf("%s: expected %v, got %v", text, ll, got)
		}
		if err := got.UnmarshalText([]byte(ll.Decimal())); err != nil {
			t.Errorf("%s: %v", ll.Decimal(), err)
		} else if !got.Equal(ll) {
			t.Errorf("%s: expected %v, got %v", ll.Decimal(), ll, got)
		}
	}
}

func Test_latlong_003(t *testing.T) {
	// A position is an object in JSON, and back
	ll := geo.LatLong{Lat: -33.8568, Long: 151.2153}
	data, err := json.Marshal(ll)
	if err != nil {
		t.Fatal(err)
	} else if string(data) != `{"lat":-33.8568,"long":151.2153}` {
		t.Errorf("expected an object, got %s", data)
	}
	var got geo.LatLong
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	} else if !got.Equal(ll) {
		t.Errorf("expected %v, got %v", ll, got)
	}

	// With an altitude, which is in the JSON only when there's one
	ll = ll.WithAlt(58)
	if data, err := json.Marshal(ll); err != nil {
		t.Fatal(err)
	} else if string(data) != `{"lat":-33.8568,"long":151.2153,"alt":58}` {
		t.Errorf("expected an object with alt, got %s", data)
	} else if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	} else if !got.Equal(ll) {
		t.Errorf("expected %v, got %v", ll, got)
	}

	// Unmarshalling a position without an altitude clears it
	if err := json.Unmarshal([]byte(`{"lat":1,"long":2}`), &got); err != nil {
		t.Fatal(err)
	} else if got.Alt != nil {
		t.Errorf("expected no altitude, got %v", got)
	}

	// It's the same within another value, and as a pointer
	var v struct {
		Position *geo.LatLong `json:"position"`
	}
	if err := json.Unmarshal([]byte(`{"position":{"lat":1.5,"long":-2.5}}`), &v); err != nil {
		t.Fatal(err)
	} else if v.Position == nil || !v.Position.Equal(geo.LatLong{Lat: 1.5, Long: -2.5}) {
		t.Errorf("expected 1.5,-2.5, got %v", v.Position)
	}
}

func Test_latlong_004(t *testing.T) {
	// JSON can be a string of the text
	var got geo.LatLong
	if err := json.Unmarshal([]byte(`"52.472697, 13.337869, 91.5"`), &got); err != nil {
		t.Fatal(err)
	} else if !got.Equal(geo.LatLong{Lat: 52.472697, Long: 13.337869}.WithAlt(91.5)) {
		t.Errorf("expected 52.472697,13.337869,91.5, got %v", got)
	}
}

func Test_latlong_005(t *testing.T) {
	// JSON which isn't a position, is missing either part, or is off the
	// Earth, is an error
	for _, data := range []string{`{}`, `{"lat":1}`, `{"long":1}`, `{"lat":91,"long":0}`, `{"lat":"1","long":2}`, `{"lat":1,"long":2,"alt":"high"}`, `"1"`, `[1,2]`, `null`} {
		var got geo.LatLong
		if err := json.Unmarshal([]byte(data), &got); err == nil {
			t.Errorf("%s: expected an error, got %v", data, got)
		}
	}

	// A position which is off the Earth, or isn't a number, can't be
	// marshalled
	for _, ll := range []geo.LatLong{{Lat: 91}, {Long: -181}, {Lat: math.NaN()}, geo.LatLong{}.WithAlt(math.Inf(1))} {
		if _, err := json.Marshal(ll); err == nil {
			t.Errorf("%v: expected an error", ll)
		}
		if _, err := ll.MarshalText(); err == nil {
			t.Errorf("%v: expected an error", ll)
		}
	}
}

func Test_latlong_006(t *testing.T) {
	// Positions are equal with the same altitude, or neither with one, and
	// WithAlt doesn't change the position it's called on
	ll := geo.LatLong{Lat: 1, Long: 2}
	alt := ll.WithAlt(3)
	if ll.Alt != nil {
		t.Error("expected WithAlt to return a copy")
	}
	if ll.Equal(alt) || alt.Equal(ll) || !alt.Equal(ll.WithAlt(3)) || alt.Equal(ll.WithAlt(4)) || !ll.Equal(geo.LatLong{Lat: 1, Long: 2}) {
		t.Error("expected positions to be equal only with the same altitude")
	}
}

func Test_latlong_007(t *testing.T) {
	// New checks the position is on the Earth
	if _, err := geo.New(-90, 180); err != nil {
		t.Error(err)
	}
	if _, err := geo.New(-90.0001, 0); err == nil {
		t.Error("expected an error")
	}
}
