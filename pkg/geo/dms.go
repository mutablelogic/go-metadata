package geo

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// dmsSecondDecimals are the decimal places of the seconds of a position
	// in degrees, minutes and seconds, a hundredth of which is about 30cm
	dmsSecondDecimals = 2

	// altitudeUnit follows the altitude of a position in degrees, minutes
	// and seconds, which is rounded to a centimetre
	altitudeUnit  = "m"
	altitudeScale = 100
)

var (
	// reDMS matches a position in degrees, minutes and seconds, which is a
	// latitude and longitude, separated by spaces or a comma, and optionally
	// an altitude in metres, such as 52°28'21.71"N 13°20'16.33"E 91.5m
	reDMS = regexp.MustCompile(`^\s*` + reDMSCoord("NSns") + `\s*,?\s*` + reDMSCoord("EWew") +
		`(?:\s*,?\s*(-?\d+(?:\.\d+)?)\s*` + altitudeUnit + `?)?\s*$`)
)

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// String returns the position in degrees, minutes and seconds, such as
// 52°28'21.71"N 13°20'16.33"E, with the altitude in metres when there is
// one, such as 52°28'21.71"N 13°20'16.33"E 91.5m. The seconds are rounded
// to a hundredth, which is about 30cm, and the altitude to a centimetre.
func (ll LatLong) String() string {
	text := dms(ll.Lat, 'N', 'S') + " " + dms(ll.Long, 'E', 'W')
	if ll.Alt != nil {
		text += " " + strconv.FormatFloat(math.Round(*ll.Alt*altitudeScale)/altitudeScale, 'f', -1, 64) + altitudeUnit
	}
	return text
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// reDMSCoord returns a pattern which matches a latitude or longitude in
// degrees, and optionally minutes and seconds, with its hemisphere, one of
// the hemispheres, before or after it, such as 52°28'21.71"N,
// N 52° 28.362', or -13.3378°
func reDMSCoord(hemispheres string) string {
	return `([` + hemispheres + `])?\s*(-)?(\d+(?:\.\d+)?)\s*[°º]\s*` +
		`(?:(\d+(?:\.\d+)?)\s*['′’]\s*)?` +
		`(?:(\d+(?:\.\d+)?)\s*(?:["″”]|''|′′)\s*)?` +
		`([` + hemispheres + `])?`
}

// dms returns decimal degrees as degrees, minutes and seconds, and the
// hemisphere, pos when it's zero or more, and neg when it's less, rounded
// to the decimal places of the seconds, which carry into the minutes and
// degrees
func dms(value float64, pos, neg rune) string {
	hemisphere := pos
	if value < 0 {
		hemisphere = neg
	}
	scale := math.Pow10(dmsSecondDecimals)
	total := int64(math.Round(math.Abs(value) * 3600 * scale))
	deg := total / int64(3600*scale)
	min := total / int64(60*scale) % 60
	sec := float64(total%int64(60*scale)) / scale
	return fmt.Sprintf("%d°%02d'%0*.*f\"%c", deg, min, dmsSecondDecimals+3, dmsSecondDecimals, sec, hemisphere)
}

// parseDMS returns a position from degrees, minutes and seconds, such as
// 52°28'21.71"N 13°20'16.33"E, or false when the text isn't in degrees
func parseDMS(text string) (LatLong, bool, error) {
	match := reDMS.FindStringSubmatch(text)
	if match == nil {
		return LatLong{}, false, nil
	}
	lat, err := dmsDegrees(match[1:7], "NS", "latitude")
	if err != nil {
		return LatLong{}, true, metadata.ErrBadParameter.Withf("position %q: %v", text, err)
	}
	long, err := dmsDegrees(match[7:13], "EW", "longitude")
	if err != nil {
		return LatLong{}, true, metadata.ErrBadParameter.Withf("position %q: %v", text, err)
	}
	ll := LatLong{Lat: lat, Long: long}
	if match[13] != "" {
		alt, err := strconv.ParseFloat(match[13], 64)
		if err != nil {
			return LatLong{}, true, metadata.ErrBadParameter.Withf("position %q: altitude: %v", text, err)
		}
		ll = ll.WithAlt(alt)
	}
	return ll, true, nil
}

// dmsDegrees returns decimal degrees from the parts of a latitude or
// longitude which reDMSCoord matches: the hemisphere before, the sign, the
// degrees, minutes and seconds, and the hemisphere after. The hemisphere is
// one of hemispheres, the second of which is negative, and there can be
// one, or a sign, but not both. Only the last of the degrees, minutes and
// seconds can have decimals, and minutes and seconds are less than 60.
func dmsDegrees(parts []string, hemispheres, name string) (float64, error) {
	before, sign, deg, min, sec, after := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]

	// The hemisphere, or the sign
	hemisphere := strings.ToUpper(before + after)
	if len(hemisphere) > 1 {
		return 0, fmt.Errorf("%s: more than one hemisphere", name)
	} else if hemisphere != "" && sign != "" {
		return 0, fmt.Errorf("%s: a hemisphere and a sign", name)
	}

	// The degrees, minutes and seconds, of which only the last has decimals
	values := []string{deg, min, sec}
	if sec != "" && min == "" {
		return 0, fmt.Errorf("%s: seconds without minutes", name)
	}
	var result float64
	for i, scale := range []float64{1, 60, 3600} {
		if values[i] == "" {
			continue
		} else if i+1 < len(values) && values[i+1] != "" && strings.Contains(values[i], ".") {
			return 0, fmt.Errorf("%s: decimals before the last part", name)
		}
		v, err := strconv.ParseFloat(values[i], 64)
		if err != nil {
			return 0, fmt.Errorf("%s: %v", name, err)
		} else if i > 0 && v >= 60 {
			return 0, fmt.Errorf("%s: %v is 60 or more", name, v)
		}
		result += v / scale
	}
	if sign != "" || hemisphere == string(hemispheres[1]) {
		result = -result
	}
	return result, nil
}
