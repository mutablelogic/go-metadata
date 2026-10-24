package xmp

import (
	"strconv"
	"strings"
	"sync"
	"time"

	// Packages
	rational "github.com/mutablelogic/go-metadata/pkg/rational"
)

// ValueType describes the scalar value type expected for an XMP tag.
type ValueType uint8

const (
	ValueTypeUnknown ValueType = iota
	ValueTypeTime
	ValueTypeDuration
	ValueTypeBoolean
	ValueTypeRational
	ValueTypeGPSCoord
)

var (
	// tagTypes are the types of the properties, in "prefix:name" form, of
	// the schemas of XMP itself. The packages of other schemas register the
	// types of their properties, such as the exif and tiff schemas in the
	// exif package (see RegisterValueType).
	tagTypeMu sync.RWMutex
	tagTypes  = map[string]ValueType{
		"xmp:CreateDate":   ValueTypeTime,
		"xmp:ModifyDate":   ValueTypeTime,
		"xmp:MetadataDate": ValueTypeTime,
		"xmpRights:Marked": ValueTypeBoolean,
	}
)

// RegisterValueType registers or overrides the scalar type for a specific
// XMP key in "prefix:name" form, which the package of a schema calls for
// the properties of its schema, such as exif:FNumber.
func RegisterValueType(key string, typ ValueType) {
	if key == "" {
		return
	}
	tagTypeMu.Lock()
	defer tagTypeMu.Unlock()
	tagTypes[key] = typ
}

// ValueTypeForKey returns the registered scalar type for key.
func ValueTypeForKey(key string) ValueType {
	tagTypeMu.RLock()
	defer tagTypeMu.RUnlock()
	if typ, ok := tagTypes[key]; ok {
		return typ
	}
	return ValueTypeUnknown
}

// timeLayouts are the forms of an XMP date, which is a subset of ISO 8601:
// a year, month or day, or a time to the minute or second, with a fraction
// of a second, and with a time zone, or without one, when it's in UTC
var timeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"2006-01",
	"2006",
}

// parseTimeValue returns an XMP date, in any of its forms (see timeLayouts).
// A fraction of a second is accepted after the seconds of each form.
func parseTimeValue(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := timeLayouts
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseDurationValue(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(f * float64(time.Second)), true
}

func parseBoolValue(s string) (bool, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false, false
	}
	switch s {
	case "yes", "y":
		return true, true
	case "no", "n":
		return false, true
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, false
	}
	return b, true
}

func parseRationalValue(s string) (rational.Rational, bool) {
	r, err := rational.Parse(s)
	return r, err == nil
}

func parseGPSCoordValue(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, false
	}

	sign := 1.0
	if strings.HasSuffix(s, "S") || strings.HasSuffix(s, "W") {
		sign = -1
		s = strings.TrimSpace(s[:len(s)-1])
	} else if strings.HasSuffix(s, "N") || strings.HasSuffix(s, "E") {
		s = strings.TrimSpace(s[:len(s)-1])
	}

	parts := splitGPSParts(s)
	if len(parts) == 0 {
		return 0, false
	}
	if len(parts) == 1 {
		f, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, false
		}
		if f < 0 {
			sign = 1
		}
		return f * sign, true
	}

	deg, ok := parseGPSPart(parts[0])
	if !ok {
		return 0, false
	}
	min, ok := parseGPSPart(parts[1])
	if !ok {
		return 0, false
	}
	sec := 0.0
	if len(parts) >= 3 {
		if sec, ok = parseGPSPart(parts[2]); !ok {
			return 0, false
		}
	}

	value := deg + (min / 60.0) + (sec / 3600.0)
	return value * sign, true
}

func splitGPSParts(s string) []string {
	if strings.Contains(s, ",") {
		raw := strings.Split(s, ",")
		parts := make([]string, 0, len(raw))
		for _, p := range raw {
			p = strings.TrimSpace(p)
			if p != "" {
				parts = append(parts, p)
			}
		}
		return parts
	}
	return strings.Fields(s)
}

func parseGPSPart(s string) (float64, bool) {
	if r, ok := parseRationalValue(s); ok {
		return r.Float64()
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
