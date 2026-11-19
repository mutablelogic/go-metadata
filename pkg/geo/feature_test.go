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

func Test_feature_000(t *testing.T) {
	// A collection's features are read with their geometries, names and
	// identifiers, and one of a type which a Geometry isn't, such as a
	// LineString, or none, is read without one
	data := []byte(`{"type":"FeatureCollection","features":[
		{"type":"Feature","id":"park","geometry":{"type":"Polygon","coordinates":[[[0,0],[2,0],[2,1],[0,1],[0,0]]]},"properties":{"name":" The Park "}},
		{"type":"Feature","id":7,"geometry":{"type":"Point","coordinates":[13.34,52.47,91.5]},"properties":{"title":"Gate"}},
		{"type":"Feature","geometry":{"type":"LineString","coordinates":[[0,0],[1,1]]},"properties":null},
		{"type":"Feature","geometry":null,"properties":{"name":"Nowhere"}}
	]}`)
	features, err := geo.ParseGeoJSON(data)
	if err != nil {
		t.Fatal(err)
	} else if len(features) != 4 {
		t.Fatalf("got %d features, want 4", len(features))
	}
	if got := features[0].Name(); got != "The Park" {
		t.Errorf("name: got %q", got)
	} else if features[0].ID != "park" || features[1].ID != float64(7) {
		t.Errorf("ids: got %v and %v", features[0].ID, features[1].ID)
	} else if got := features[1].Name(); got != "Gate" {
		t.Errorf("name of its title: got %q", got)
	}
	if g := features[0].Geometry; g == nil || g.Type != geo.GeometryPolygon {
		t.Errorf("polygon: got %+v", g)
	}
	if g, want := features[1].Geometry, geo.NewPoint(52.47, 13.34).WithAlt(91.5); g == nil || !reflect.DeepEqual(*g, want) {
		t.Errorf("point: got %+v, want %+v", g, want)
	}
	if features[2].Geometry != nil || features[3].Geometry != nil {
		t.Errorf("without geometries: got %+v and %+v", features[2].Geometry, features[3].Geometry)
	}
	c := geo.FeatureCollection{Features: features}
	if got := c.Geometries(); len(got) != 2 {
		t.Errorf("geometries: got %d, want 2", len(got))
	}

	// It's written again, as GeoJSON, which is read as it was
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var read geo.FeatureCollection
	if err := json.Unmarshal(out, &read); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(read, c) {
		t.Errorf("read again: got %+v, want %+v", read, c)
	}
}

func Test_feature_001(t *testing.T) {
	// A feature, or a geometry, is GeoJSON of one feature
	features, err := geo.ParseGeoJSON([]byte(`{"type":"Feature","geometry":{"type":"Point","coordinates":[1,2]},"properties":{"name":"A"}}`))
	if err != nil || len(features) != 1 || features[0].Name() != "A" {
		t.Errorf("feature: got %+v, %v", features, err)
	}
	features, err = geo.ParseGeoJSON([]byte(`{"type":"Point","coordinates":[1,2]}`))
	if err != nil || len(features) != 1 || features[0].Geometry == nil || features[0].Properties != nil {
		t.Errorf("geometry: got %+v, %v", features, err)
	}

	// A feature without a geometry, or properties, is written with nulls
	out, err := json.Marshal(geo.Feature{})
	if err != nil {
		t.Fatal(err)
	} else if string(out) != `{"type":"Feature","geometry":null,"properties":null}` {
		t.Errorf("got %s", out)
	}
	out, err = json.Marshal(geo.FeatureCollection{})
	if err != nil {
		t.Fatal(err)
	} else if string(out) != `{"type":"FeatureCollection","features":[]}` {
		t.Errorf("got %s", out)
	}

	// What isn't GeoJSON, or a geometry of a type a Geometry isn't, which
	// isn't a feature's, is bad
	for _, data := range []string{
		`not json`,
		`{"type":"LineString","coordinates":[[0,0],[1,1]]}`,
		`{"type":"FeatureCollection","features":[{"type":"Thing"}]}`,
		`{"type":"Feature","geometry":{"type":"Point","coordinates":[1]}}`,
	} {
		if _, err := geo.ParseGeoJSON([]byte(data)); !errors.Is(err, metadata.ErrBadParameter) {
			t.Errorf("%s: got %v, want ErrBadParameter", data, err)
		}
	}
}
