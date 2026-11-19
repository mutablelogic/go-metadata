package geo

import (
	"bytes"
	"encoding/json"
	"strings"

	// Packages
	errs "github.com/mutablelogic/go-metadata/internal/errs"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Feature is a feature of GeoJSON (RFC 7946): a geometry, such as a
// region, with its properties, such as its name, and its identifier, when
// it has one. Its geometry is nil when it has none, or when it's of a type
// which a Geometry isn't, such as a LineString, which is read, without it.
type Feature struct {
	ID         any            // A string, or a number, as a json.Number, so it's exact, or nil
	Geometry   *Geometry      // Its geometry, or nil
	Properties map[string]any // Its properties, or nil
}

// FeatureCollection is a collection of features of GeoJSON
type FeatureCollection struct {
	Features []Feature
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	typeFeature           = "Feature"
	typeFeatureCollection = "FeatureCollection"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// ParseGeoJSON returns the features of GeoJSON, which is a feature
// collection, a feature, or a geometry, which is a feature without
// properties, or ErrBadParameter when it isn't GeoJSON
func ParseGeoJSON(data []byte) ([]Feature, error) {
	var v struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, errs.BadParameter("GeoJSON: %v", err)
	}
	switch v.Type {
	case typeFeatureCollection:
		var c FeatureCollection
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		return c.Features, nil
	case typeFeature:
		var f Feature
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		return []Feature{f}, nil
	default:
		f, err := featureGeometry(data)
		if err != nil {
			return nil, err
		}
		return []Feature{f}, nil
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the name of the feature, of its name, or title, property,
// or empty when it has neither
func (f Feature) Name() string {
	for _, key := range []string{"name", "title"} {
		if s, ok := f.Properties[key].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// Geometries returns the geometries of the features which have them
func (c FeatureCollection) Geometries() []Geometry {
	result := make([]Geometry, 0, len(c.Features))
	for _, f := range c.Features {
		if f.Geometry != nil {
			result = append(result, *f.Geometry)
		}
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// JSON

type featureJSON struct {
	Type       string          `json:"type"`
	ID         json.RawMessage `json:"id,omitempty"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}

type featureCollectionJSON struct {
	Type     string            `json:"type"`
	Features []json.RawMessage `json:"features"`
}

// MarshalJSON returns the feature as GeoJSON, whose geometry and properties
// are null when it has none
func (f Feature) MarshalJSON() ([]byte, error) {
	v := featureJSON{Type: typeFeature, Geometry: json.RawMessage("null"), Properties: f.Properties}
	if f.ID != nil {
		data, err := json.Marshal(f.ID)
		if err != nil {
			return nil, err
		}
		v.ID = data
	}
	if f.Geometry != nil {
		data, err := json.Marshal(f.Geometry)
		if err != nil {
			return nil, err
		}
		v.Geometry = data
	}
	return json.Marshal(v)
}

// UnmarshalJSON reads the feature of GeoJSON, without its geometry when
// it's of a type which a Geometry isn't, such as a LineString
func (f *Feature) UnmarshalJSON(data []byte) error {
	var v featureJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return errs.BadParameter("GeoJSON feature: %v", err)
	} else if v.Type != typeFeature {
		return errs.BadParameter("GeoJSON feature: of type %q, rather than a Feature", v.Type)
	}
	id, err := readID(v.ID)
	if err != nil {
		return err
	}
	result := Feature{ID: id, Properties: v.Properties}
	if g, err := readGeometry(v.Geometry); err != nil {
		return err
	} else {
		result.Geometry = g
	}
	*f = result
	return nil
}

// MarshalJSON returns the collection as GeoJSON
func (c FeatureCollection) MarshalJSON() ([]byte, error) {
	features := c.Features
	if features == nil {
		features = []Feature{}
	}
	return json.Marshal(struct {
		Type     string    `json:"type"`
		Features []Feature `json:"features"`
	}{Type: typeFeatureCollection, Features: features})
}

// UnmarshalJSON reads the collection of GeoJSON
func (c *FeatureCollection) UnmarshalJSON(data []byte) error {
	var v featureCollectionJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return errs.BadParameter("GeoJSON feature collection: %v", err)
	} else if v.Type != typeFeatureCollection {
		return errs.BadParameter("GeoJSON feature collection: of type %q, rather than a FeatureCollection", v.Type)
	}
	result := FeatureCollection{Features: make([]Feature, 0, len(v.Features))}
	for _, data := range v.Features {
		var f Feature
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		result.Features = append(result.Features, f)
	}
	*c = result
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// readID returns the identifier of a feature, which is a string, or a
// number, as a json.Number, which is exact, as a float64 of a large integer
// isn't, or nil when it has none
func readID(data json.RawMessage) (any, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var id any
	if err := d.Decode(&id); err != nil {
		return nil, errs.BadParameter("GeoJSON feature: its id: %v", err)
	}
	switch id.(type) {
	case string, json.Number:
		return id, nil
	default:
		return nil, errs.BadParameter("GeoJSON feature: an id of %s, rather than a string or a number", data)
	}
}

// readGeometry returns the geometry of GeoJSON, or nil when it's null, or
// of a type which a Geometry isn't
func readGeometry(data json.RawMessage) (*Geometry, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var v struct {
		Type GeometryType `json:"type"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, errs.BadParameter("GeoJSON geometry: %v", err)
	}
	switch v.Type {
	case GeometryPoint, GeometryPolygon, GeometryMultiPolygon:
		var g Geometry
		if err := json.Unmarshal(data, &g); err != nil {
			return nil, err
		}
		return &g, nil
	default:
		return nil, nil
	}
}

// featureGeometry returns the feature of a geometry, which is GeoJSON
// of a geometry, rather than a feature, or ErrBadParameter when it isn't one
// which a Geometry is
func featureGeometry(data []byte) (Feature, error) {
	var g Geometry
	if err := json.Unmarshal(data, &g); err != nil {
		return Feature{}, err
	}
	return Feature{Geometry: &g}, nil
}
