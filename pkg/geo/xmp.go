package geo

import (
	"fmt"
	"math"
	"strings"

	// Packages
	rational "github.com/mutablelogic/go-metadata/pkg/rational"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// exifPrefix is the prefix of the exif schema of XMP, whose GPS
	// properties a position is written as
	exifPrefix = "exif:"

	// xmpMinuteDecimals are the decimal places of the minutes of a GPS
	// coordinate in XMP, a millionth of which is about 2mm
	xmpMinuteDecimals = 6

	// xmpAltitudeScale is the denominator of an altitude in XMP, which is in
	// centimetres
	xmpAltitudeScale = 100
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// MarshalXMP returns the position as the GPS properties of the exif schema
// of XMP, when the key is in the exif namespace, such as exif:GPSPosition:
// exif:GPSLatitude and exif:GPSLongitude, as degrees and minutes, with
// their hemisphere, such as "53,14.053167N", and exif:GPSAltitude, a
// rational in metres, and exif:GPSAltitudeRef, which is 1 below sea level,
// when there's an altitude. It returns none for a key in another namespace,
// such as dc:coverage, which is written as its text (see String).
func (ll LatLong) MarshalXMP(key string) map[string]string {
	if !strings.HasPrefix(key, exifPrefix) || ll.Validate() != nil {
		return nil
	}
	props := map[string]string{
		exifPrefix + "GPSLatitude":  xmpCoordinate(ll.Lat, 'N', 'S'),
		exifPrefix + "GPSLongitude": xmpCoordinate(ll.Long, 'E', 'W'),
	}
	if ll.Alt != nil {
		alt, ref := *ll.Alt, "0"
		if alt < 0 {
			alt, ref = -alt, "1"
		}
		props[exifPrefix+"GPSAltitude"] = rational.New(int64(math.Round(alt*xmpAltitudeScale)), xmpAltitudeScale).String()
		props[exifPrefix+"GPSAltitudeRef"] = ref
	}
	return props
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// xmpCoordinate returns decimal degrees as a GPS coordinate of XMP, which is
// degrees and decimal minutes, and the hemisphere, pos when it's zero or
// more, and neg when it's less, such as "53,14.053167N", rounded to the
// decimal places of the minutes, which carry into the degrees
func xmpCoordinate(value float64, pos, neg rune) string {
	hemisphere := pos
	if value < 0 {
		hemisphere = neg
	}
	scale := math.Pow10(xmpMinuteDecimals)
	total := int64(math.Round(math.Abs(value) * 60 * scale))
	deg := total / int64(60*scale)
	min := float64(total%int64(60*scale)) / scale
	return fmt.Sprintf("%d,%.*f%c", deg, xmpMinuteDecimals, min, hemisphere)
}
