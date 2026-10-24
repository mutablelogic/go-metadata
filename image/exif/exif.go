//go:build cgo

// Package exif registers a handler of the EXIF metadata of JPEG images, such
// as the camera, the exposure, when the photo was taken and where, as the
// tiff and exif namespaces, which uses libexif, so needs cgo
package exif

import (
	"context"
	"io"
	"regexp"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	exif "github.com/mutablelogic/go-metadata/pkg/exif"
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
	rational "github.com/mutablelogic/go-metadata/pkg/rational"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// dateTimeTag is an EXIF date and time tag, and the tag of its UTC offset
type dateTimeTag struct {
	ns, key, offsetKey string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// The namespaces of the tags (see the exif package)
	TIFFNamespace = exif.TIFFNamespace
	EXIFNamespace = exif.EXIFNamespace

	// noCopyright is a copyright which isn't set, as libexif formats it
	noCopyright = "[None]"

	// exifDateTime is the format of an EXIF date and time, which can be
	// followed by a UTC offset
	exifDateTime = "2006:01:02 15:04:05"

	// GPSPosition is the key of the GPS position of a photo, a geo.LatLong,
	// which replaces the tags of its latitude, longitude and altitude
	GPSPosition = "GPSPosition"
)

var (
	reContentType = regexp.MustCompile(`^image/jpeg$`)

	// reCopyright matches tiff:Copyright as libexif formats it, which is the
	// copyright of the photographer and of the editor, either of which is
	// noCopyright when it isn't set
	reCopyright = regexp.MustCompile(`^(.*) \(Photographer\) - (.*) \(Editor\)$`)

	// The date and time tags, which are times, with their UTC offset when
	// there is one
	dateTimeTags = []dateTimeTag{
		{TIFFNamespace, "DateTime", "OffsetTime"},
		{EXIFNamespace, "DateTimeOriginal", "OffsetTimeOriginal"},
		{EXIFNamespace, "DateTimeDigitized", "OffsetTimeDigitized"},
	}

	// The tags which the GPS position replaces, the first of which is
	// where it is
	gpsTags = []string{"GPSLatitude", "GPSLatitudeRef", "GPSLongitude", "GPSLongitudeRef", "GPSAltitude", "GPSAltitudeRef"}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	metadata.AddHandler(reContentType, EXIFNamespace, func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		f, err := exif.Read(r)
		if err != nil {
			// A JPEG image without EXIF metadata has none
			return nil, nil
		}
		defer f.Close()
		return TagsMetadata(f.Tags()), nil
	}, TIFFNamespace, EXIFNamespace, dc.Namespace)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// TagsMetadata returns the metadata of the EXIF tags, in their order, as
// libexif formats them, such as "f/2.8", without the spaces cameras pad
// them with, or tags which are empty: a tag of a single rational is
// also a number, a date and time is a time with its UTC offset, without
// the tag of the offset, or none when it's unknown, and the latitude,
// longitude and altitude are a position, exif:GPSPosition, without their
// tags. The time the photo was taken, or failing that digitized or the
// file changed, is dc:date, and the position is dc:coverage.
func TagsMetadata(tags []*exif.Tag) []metadata.Metadata {
	// The tags by namespace and key, as the first of each wins
	index := make(map[string]*exif.Tag, len(tags))
	for _, tag := range tags {
		if _, exists := index[tag.Name()]; !exists {
			index[tag.Name()] = tag
		}
	}
	lookup := func(ns, key string) *exif.Tag {
		return index[ns+string(metadata.Separator)+key]
	}

	// The metadata which replaces tags, and the tags which are dropped
	replace := make(map[string]metadata.Metadata)
	drop := make(map[string]bool)
	for name, tag := range index {
		if v, ok := rationalFloat(tag.Any()); ok {
			replace[name] = meta.NewText(tag.NS(), tag.Key(), v, tag.Value())
		}
	}
	for _, dt := range dateTimeTags {
		tag := lookup(dt.ns, dt.key)
		if tag == nil {
			continue
		}
		var offset string
		if offsetTag := lookup(EXIFNamespace, dt.offsetKey); offsetTag != nil {
			offset = strings.TrimSpace(offsetTag.Value())
		}
		if t, ok := parseDateTime(tag.Value(), offset); ok {
			replace[tag.Name()] = meta.New(dt.ns, dt.key, t)
			if offset != "" {
				drop[EXIFNamespace+string(metadata.Separator)+dt.offsetKey] = true
			}
		} else if unknownDateTime(tag.Value()) {
			drop[tag.Name()] = true
		}
	}
	position, hasPosition := gpsPosition(lookup)
	if hasPosition {
		var where string
		for _, key := range gpsTags {
			if tag := lookup(EXIFNamespace, key); tag != nil {
				if where == "" {
					where = tag.Name()
				} else {
					drop[tag.Name()] = true
				}
			}
		}
		replace[where] = meta.New(EXIFNamespace, GPSPosition, position)
	}

	// The tags, in their order, replaced or dropped
	result := make([]metadata.Metadata, 0, len(index)+1)
	seen := make(map[string]bool, len(index))
	for _, tag := range tags {
		name := tag.Name()
		if seen[name] || drop[name] {
			continue
		}
		seen[name] = true
		if m, exists := replace[name]; exists {
			result = append(result, m)
		} else if value := tagValue(tag); value == "" {
			// A tag which is empty, or only spaces, as cameras often pad text
			// with spaces, is dropped
			continue
		} else if m := meta.New(tag.NS(), tag.Key(), value); m != nil {
			result = append(result, m)
		}
	}

	// The time the photo was taken, as dc:date
	for _, name := range []string{"exif:DateTimeOriginal", "exif:DateTimeDigitized", "tiff:DateTime"} {
		if m, exists := replace[name]; exists {
			if t, ok := m.Any().(time.Time); ok {
				result = append(result, meta.New(dc.Namespace, dc.Date, t))
				break
			}
		}
	}

	// The position where the photo was taken, as dc:coverage
	if hasPosition {
		result = append(result, meta.New(dc.Namespace, dc.Coverage, position))
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// tagValue returns the value of a tag as libexif formats it, without the
// spaces cameras pad it with, and tiff:Copyright as its copyrights, of the
// photographer and the editor, without those which aren't set
func tagValue(tag *exif.Tag) string {
	value := strings.TrimSpace(tag.Value())
	if tag.NS() == TIFFNamespace && tag.Key() == "Copyright" {
		return copyright(value)
	}
	return value
}

// copyright returns tiff:Copyright, as libexif formats it, as the
// copyrights of the photographer and the editor, joined with "; ", without
// those which aren't set, or the value itself when it isn't formatted so
func copyright(value string) string {
	match := reCopyright.FindStringSubmatch(value)
	if match == nil {
		return value
	}
	var parts []string
	for _, part := range match[1:] {
		if part = strings.TrimSpace(part); part != "" && part != noCopyright {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "; ")
}

// parseDateTime returns an EXIF date and time, such as "2024:06:01 12:30:00",
// with its UTC offset, such as "+01:00", or in UTC when it has none
func parseDateTime(value, offset string) (time.Time, bool) {
	layout := exifDateTime
	if offset != "" {
		layout += "-07:00"
		value += offset
	}
	t, err := time.Parse(layout, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// unknownDateTime returns true when an EXIF date and time is unknown, which
// is zeros, such as when a camera's clock isn't set, or spaces and colons
func unknownDateTime(value string) bool {
	return strings.Trim(value, "0: \x00") == ""
}

// gpsPosition returns the position of the GPS tags, which are the latitude
// and longitude in degrees, minutes and seconds, and their hemispheres, S
// and W being negative, and optionally the altitude in metres, and its
// reference, 1 being below sea level, or false when there's no latitude or
// longitude, or it isn't on the Earth. An altitude of zero is none, as
// it's usually what's written when the altitude isn't known.
func gpsPosition(lookup func(ns, key string) *exif.Tag) (geo.LatLong, bool) {
	degrees := func(key, refKey, negRef string) (float64, bool) {
		tag := lookup(EXIFNamespace, key)
		if tag == nil {
			return 0, false
		}
		dms, ok := tag.Any().([]rational.Rational)
		if !ok {
			return 0, false
		}
		deg, ok := dmsDecimal(dms)
		if !ok {
			return 0, false
		}
		if ref := lookup(EXIFNamespace, refKey); ref != nil && strings.EqualFold(strings.TrimSpace(ref.Value()), negRef) {
			deg = -deg
		}
		return deg, true
	}
	lat, ok := degrees("GPSLatitude", "GPSLatitudeRef", "S")
	if !ok {
		return geo.LatLong{}, false
	}
	long, ok := degrees("GPSLongitude", "GPSLongitudeRef", "W")
	if !ok {
		return geo.LatLong{}, false
	}
	position, err := geo.New(lat, long)
	if err != nil {
		return geo.LatLong{}, false
	}
	if tag := lookup(EXIFNamespace, "GPSAltitude"); tag != nil {
		if alt, ok := rationalFloat(tag.Any()); ok && alt != 0 {
			if ref := lookup(EXIFNamespace, "GPSAltitudeRef"); ref != nil {
				if b, ok := ref.Any().(uint8); ok && b == 1 {
					alt = -alt
				}
			}
			position = position.WithAlt(alt)
		}
	}
	return position, true
}

// dmsDecimal returns degrees, minutes and seconds as decimal degrees
func dmsDecimal(dms []rational.Rational) (float64, bool) {
	if len(dms) != 3 {
		return 0, false
	}
	var result float64
	for i, scale := range []float64{1, 60, 3600} {
		v, ok := rationalFloat(dms[i])
		if !ok {
			return 0, false
		}
		result += v / scale
	}
	return result, true
}

// rationalFloat returns a single rational, signed or unsigned, as a number,
// or false when it isn't one, or its denominator is zero
func rationalFloat(value any) (float64, bool) {
	if r, ok := value.(rational.Rational); ok {
		return r.Float64()
	}
	return 0, false
}
