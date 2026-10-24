package geo

import (
	"regexp"
	"strconv"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// reISO6709 matches a position in ISO 6709, in decimal degrees, which is
// the latitude, longitude and optionally the altitude, each with its sign,
// such as +52.4158+013.5896+035.667/, as a QuickTime video's location is
var reISO6709 = regexp.MustCompile(`^([+-]\d{1,2}(?:\.\d+)?)([+-]\d{1,3}(?:\.\d+)?)([+-]\d+(?:\.\d+)?)?(?:CRS[^/]*)?/?$`)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ParseISO6709 returns a position from text in ISO 6709, in decimal
// degrees, such as +52.4158+013.5896+035.667/, whose altitude, in metres,
// is optional, or an error when it isn't one, or it isn't on the Earth
func ParseISO6709(text string) (LatLong, error) {
	match := reISO6709.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return LatLong{}, metadata.ErrBadParameter.Withf("position %q: expected ISO 6709", text)
	}
	lat, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return LatLong{}, metadata.ErrBadParameter.Withf("position %q: latitude: %v", text, err)
	}
	long, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return LatLong{}, metadata.ErrBadParameter.Withf("position %q: longitude: %v", text, err)
	}
	ll := LatLong{Lat: lat, Long: long}
	if match[3] != "" {
		alt, err := strconv.ParseFloat(match[3], 64)
		if err != nil {
			return LatLong{}, metadata.ErrBadParameter.Withf("position %q: altitude: %v", text, err)
		}
		ll = ll.WithAlt(alt)
	}
	if err := ll.Validate(); err != nil {
		return LatLong{}, err
	}
	return ll, nil
}
