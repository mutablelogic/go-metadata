package geo_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
)

// roundtrip returns the geometry, of its GeoJSON, which it's marshalled as,
// which is the JSON when it isn't empty
func roundtrip(t *testing.T, g geo.Geometry, want string) geo.Geometry {
	t.Helper()
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if want != "" {
		var got, expected any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		} else if err := json.Unmarshal([]byte(want), &expected); err != nil {
			t.Fatal(err)
		} else if !reflect.DeepEqual(got, expected) {
			t.Errorf("JSON: got %s, want %s", data, want)
		}
	}
	var read geo.Geometry
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(read, g) {
		t.Errorf("read again: got %+v, want %+v", read, g)
	}
	return read
}

func Test_geometry_000(t *testing.T) {
	// A point is of its latitude and longitude, which is GeoJSON's
	// longitude, then latitude, and its altitude, when it's known
	point := geo.NewPoint(51.5, -0.12)
	if err := point.Validate(); err != nil {
		t.Fatal(err)
	}
	roundtrip(t, point, `{"type":"Point","coordinates":[-0.12,51.5]}`)
	roundtrip(t, point.WithAlt(30), `{"type":"Point","coordinates":[-0.12,51.5,30]}`)
	if got, want := point.Bounds(), (geo.Bounds{MinLong: -0.12, MinLat: 51.5, MaxLong: -0.12, MaxLat: 51.5}); got != want {
		t.Errorf("bounds: got %+v, want %+v", got, want)
	}
}

func Test_geometry_001(t *testing.T) {
	// A polygon is of its rings, which are closed, and its box is of its
	// boundary, and a multipolygon is of more than one
	square := []geo.Position{{Long: 0, Lat: 0}, {Long: 2, Lat: 0}, {Long: 2, Lat: 1}, {Long: 0, Lat: 1}, {Long: 0, Lat: 0}}
	hole := []geo.Position{{Long: 0.5, Lat: 0.25}, {Long: 1, Lat: 0.25}, {Long: 1, Lat: 0.5}, {Long: 0.5, Lat: 0.25}}
	polygon := geo.NewPolygon(square, hole)
	if err := polygon.Validate(); err != nil {
		t.Fatal(err)
	}
	roundtrip(t, polygon, `{"type":"Polygon","coordinates":[[[0,0],[2,0],[2,1],[0,1],[0,0]],[[0.5,0.25],[1,0.25],[1,0.5],[0.5,0.25]]]}`)
	if got, want := polygon.Bounds(), (geo.Bounds{MinLong: 0, MinLat: 0, MaxLong: 2, MaxLat: 1}); got != want {
		t.Errorf("bounds: got %+v, want %+v", got, want)
	}

	far := []geo.Position{{Long: 10, Lat: -5}, {Long: 11, Lat: -5}, {Long: 11, Lat: -4}, {Long: 10, Lat: -5}}
	multi := geo.NewMultiPolygon([][]geo.Position{square}, [][]geo.Position{far})
	if err := multi.Validate(); err != nil {
		t.Fatal(err)
	}
	roundtrip(t, multi, "")
	if got, want := multi.Bounds(), (geo.Bounds{MinLong: 0, MinLat: -5, MaxLong: 11, MaxLat: 1}); got != want {
		t.Errorf("bounds: got %+v, want %+v", got, want)
	}
}

func Test_geometry_002(t *testing.T) {
	// A geometry which isn't valid, or of another type of GeoJSON's, is
	// go-metadata's ErrBadParameter
	for name, g := range map[string]geo.Geometry{
		"latitude":    geo.NewPoint(91, 0),
		"longitude":   geo.NewPoint(0, -181),
		"open ring":   geo.NewPolygon([]geo.Position{{Long: 0, Lat: 0}, {Long: 1, Lat: 0}, {Long: 1, Lat: 1}, {Long: 0, Lat: 1}}),
		"short ring":  geo.NewPolygon([]geo.Position{{Long: 0, Lat: 0}, {Long: 1, Lat: 0}, {Long: 0, Lat: 0}}),
		"no rings":    geo.NewPolygon(),
		"none":        geo.NewMultiPolygon(),
		"type":        {Type: "LineString"},
		"altitude":    geo.NewPolygon([]geo.Position{{Long: 0, Lat: 0, Alt: ptr(10)}, {Long: 1, Lat: 0}, {Long: 1, Lat: 1}, {Long: 0, Lat: 0, Alt: ptr(20)}}),
		"no altitude": geo.NewPolygon([]geo.Position{{Long: 0, Lat: 0, Alt: ptr(10)}, {Long: 1, Lat: 0}, {Long: 1, Lat: 1}, {Long: 0, Lat: 0}}),
	} {
		if err := g.Validate(); !errors.Is(err, metadata.ErrBadParameter) {
			t.Errorf("%s: got %v, want ErrBadParameter", name, err)
		}
	}
	var read geo.Geometry
	for _, data := range []string{
		`{"type":"LineString","coordinates":[[0,0],[1,1]]}`,
		`{"type":"Point","coordinates":[1]}`,
		`{"type":"Point","coordinates":[null,51.5]}`,
		`{"type":"Point","coordinates":[-0.12,51.5,null]}`,
	} {
		if err := json.Unmarshal([]byte(data), &read); err == nil {
			t.Errorf("%s: read, rather than an error", data)
		}
	}
}

func Test_geometry_003(t *testing.T) {
	// A ring is closed of its altitudes too, when they're the same
	closed := geo.NewPolygon([]geo.Position{{Long: 0, Lat: 0, Alt: ptr(10)}, {Long: 1, Lat: 0}, {Long: 1, Lat: 1}, {Long: 0, Lat: 0, Alt: ptr(10)}})
	if err := closed.Validate(); err != nil {
		t.Error(err)
	}

	// A position is a point, of its altitude when it's known
	alt := 91.5
	if got, want := (geo.LatLong{Lat: 52.47, Long: 13.34, Alt: &alt}).Geometry(), geo.NewPoint(52.47, 13.34).WithAlt(91.5); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got, want := (geo.LatLong{Lat: 52.47, Long: 13.34}).Geometry(), geo.NewPoint(52.47, 13.34); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Its errors are still go-metadata's
	if _, err := geo.Parse("not a position"); !errors.Is(err, metadata.ErrBadParameter) {
		t.Errorf("got %v, want ErrBadParameter", err)
	}
}

// ptr returns a pointer to the value
func ptr(v float64) *float64 {
	return &v
}
