// Package geo defines a position on the Earth, as a latitude and longitude
// in decimal degrees, and optionally an altitude in metres, which is text in
// degrees, minutes and seconds, such as 52°28'21.71"N 13°20'16.33"E 91.5m,
// or decimal degrees, such as "52.472697,13.337869,91.5", as in a geo URI
// (RFC 5870), and JSON such as {"lat":52.472697,"long":13.337869,"alt":91.5}
package geo

import (
	"bytes"
	"encoding"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// LatLong is a position, as a latitude and longitude in decimal degrees,
// which are negative in the southern and western hemispheres, and an
// altitude in metres above sea level, which is negative below it, or nil
// when it isn't known
type LatLong struct {
	Lat  float64  `json:"lat"`
	Long float64  `json:"long"`
	Alt  *float64 `json:"alt,omitempty"`
}

// latLongJSON is a LatLong in JSON, without its methods
type latLongJSON struct {
	Lat  *float64 `json:"lat"`
	Long *float64 `json:"long"`
	Alt  *float64 `json:"alt,omitempty"`
}

var _ encoding.TextMarshaler = LatLong{}
var _ encoding.TextUnmarshaler = (*LatLong)(nil)
var _ json.Marshaler = LatLong{}
var _ json.Unmarshaler = (*LatLong)(nil)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// separator is between the latitude, longitude and altitude in text
	separator = ","
)

// partNames are the names of the parts of a position in text
var partNames = []string{"latitude", "longitude", "altitude"}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a position without an altitude, or an error when the
// latitude isn't between -90 and 90, or the longitude between -180 and 180
func New(lat, long float64) (LatLong, error) {
	ll := LatLong{Lat: lat, Long: long}
	if err := ll.Validate(); err != nil {
		return LatLong{}, err
	}
	return ll, nil
}

// Parse returns a position from text, which is the latitude, longitude and
// optionally the altitude, separated by commas, in that order, as in ISO
// 6709 and a geo URI (RFC 5870), such as
// "52.472697,13.337869" or "52.472697,13.337869,91.5", with or without
// spaces, or in degrees, minutes and seconds, such as
// 52°28'21.71"N 13°20'16.33"E 91.5m (see DMS)
func Parse(text string) (LatLong, error) {
	if ll, ok, err := parseDMS(text); err != nil {
		return LatLong{}, err
	} else if ok {
		if err := ll.Validate(); err != nil {
			return LatLong{}, err
		}
		return ll, nil
	}
	parts := strings.Split(text, separator)
	if len(parts) != 2 && len(parts) != 3 {
		return LatLong{}, metadata.ErrBadParameter.Withf("position %q: expected latitude,longitude or latitude,longitude,altitude", text)
	}
	values := make([]float64, len(parts))
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return LatLong{}, metadata.ErrBadParameter.Withf("position %q: %s: %v", text, partNames[i], err)
		}
		values[i] = v
	}
	ll := LatLong{Lat: values[0], Long: values[1]}
	if len(values) == 3 {
		ll = ll.WithAlt(values[2])
	}
	if err := ll.Validate(); err != nil {
		return LatLong{}, err
	}
	return ll, nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// Decimal returns the position in decimal degrees, such as
// "52.472697,13.337869", or "52.472697,13.337869,91.5" with an altitude.
// The latitude is first, as in ISO 6709 and a geo URI (RFC 5870), unlike
// GeoJSON, whose longitude is first; text in degrees, minutes and seconds
// (see String) has hemispheres, so isn't ambiguous.
func (ll LatLong) Decimal() string {
	text := strconv.FormatFloat(ll.Lat, 'f', -1, 64) + separator + strconv.FormatFloat(ll.Long, 'f', -1, 64)
	if ll.Alt != nil {
		text += separator + strconv.FormatFloat(*ll.Alt, 'f', -1, 64)
	}
	return text
}

// MarshalText returns the position in degrees, minutes and seconds, such
// as 52°28'21.71"N 13°20'16.33"E, with the altitude when there is one,
// such as 52°28'21.71"N 13°20'16.33"E 91.5m, whose seconds are rounded to
// a hundredth (see String)
func (ll LatLong) MarshalText() ([]byte, error) {
	if err := ll.Validate(); err != nil {
		return nil, err
	}
	return []byte(ll.String()), nil
}

// UnmarshalText sets the position from text, in degrees, minutes and
// seconds, such as 52°28'21.71"N 13°20'16.33"E, or decimal degrees, such as
// "52.472697,13.337869" (see Parse)
func (ll *LatLong) UnmarshalText(text []byte) error {
	v, err := Parse(string(text))
	if err != nil {
		return err
	}
	*ll = v
	return nil
}

// MarshalJSON returns the position as JSON, such as
// {"lat":52.472697,"long":13.337869}, with alt when there's an altitude
func (ll LatLong) MarshalJSON() ([]byte, error) {
	if err := ll.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(latLongJSON{Lat: &ll.Lat, Long: &ll.Long, Alt: ll.Alt})
}

// UnmarshalJSON sets the position from JSON, which is an object with both
// lat and long, and optionally alt, such as
// {"lat":52.472697,"long":13.337869,"alt":91.5}, or a string of its text,
// such as "52.472697,13.337869,91.5"
func (ll *LatLong) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return metadata.ErrBadParameter.Withf("position: %v", err)
		}
		return ll.UnmarshalText([]byte(text))
	}
	var v latLongJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return metadata.ErrBadParameter.Withf("position: %v", err)
	} else if v.Lat == nil || v.Long == nil {
		return metadata.ErrBadParameter.With("position: expected lat and long")
	}
	result := LatLong{Lat: *v.Lat, Long: *v.Long, Alt: v.Alt}
	if err := result.Validate(); err != nil {
		return err
	}
	*ll = result
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithAlt returns the position with an altitude, in metres above sea level,
// which is negative below it
func (ll LatLong) WithAlt(alt float64) LatLong {
	ll.Alt = &alt
	return ll
}

// Equal returns true when the positions are the same, with the same
// altitude, or neither with one
func (ll LatLong) Equal(other LatLong) bool {
	if ll.Lat != other.Lat || ll.Long != other.Long {
		return false
	} else if ll.Alt == nil || other.Alt == nil {
		return ll.Alt == nil && other.Alt == nil
	}
	return *ll.Alt == *other.Alt
}

// Validate returns an error when the latitude isn't between -90 and 90, or
// the longitude between -180 and 180, or either, or the altitude, isn't a
// finite number
func (ll LatLong) Validate() error {
	if math.IsNaN(ll.Lat) || ll.Lat < -90 || ll.Lat > 90 {
		return metadata.ErrBadParameter.Withf("latitude %v: expected -90 to 90", ll.Lat)
	} else if math.IsNaN(ll.Long) || ll.Long < -180 || ll.Long > 180 {
		return metadata.ErrBadParameter.Withf("longitude %v: expected -180 to 180", ll.Long)
	} else if ll.Alt != nil && (math.IsNaN(*ll.Alt) || math.IsInf(*ll.Alt, 0)) {
		return metadata.ErrBadParameter.Withf("altitude %v: expected a number", *ll.Alt)
	}
	return nil
}
