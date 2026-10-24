//go:build cgo

package raw

import (
	"fmt"
	"strings"
	"time"

	// Packages
	libraw "github.com/mutablelogic/go-media/sys/libraw"
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	exif "github.com/mutablelogic/go-metadata/pkg/exif"
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// newMetadata returns the camera and shooting parameters of the RAW image,
// in the namespaces of EXIF metadata, tiff for the camera and exif for the
// shooting parameters,
// each formatted as text, such as "f/2.0" or "1/320", with its value as a
// number, or a time, when it's set
func newMetadata(r *RAW) []metadata.Metadata {
	iparams := libraw.Libraw_get_iparams(r.data)
	other := libraw.Libraw_get_imgother(r.data)

	var result []metadata.Metadata
	add := func(ns, key, val string, typed any) {
		if val != "" {
			result = append(result, meta.NewText(ns, key, typed, val))
		}
	}
	text := func(ns, key, val string) {
		val = strings.TrimSpace(val)
		add(ns, key, val, val)
	}

	text(exif.TIFFNamespace, "Make", libraw.IParams_make(iparams))
	text(exif.TIFFNamespace, "Model", libraw.IParams_model(iparams))
	text(exif.TIFFNamespace, "Software", libraw.IParams_software(iparams))

	if iso := libraw.ImgOther_iso_speed(other); iso > 0 {
		add(exif.EXIFNamespace, "ISOSpeedRatings", fmt.Sprintf("%.0f", iso), float64(iso))
	}
	if s := libraw.ImgOther_shutter(other); s > 0 {
		add(exif.EXIFNamespace, "ExposureTime", shutterStr(s), float64(s))
	}
	if ap := libraw.ImgOther_aperture(other); ap > 0 {
		add(exif.EXIFNamespace, "FNumber", fmt.Sprintf("f/%.1f", ap), float64(ap))
	}
	if fl := libraw.ImgOther_focal_len(other); fl > 0 {
		add(exif.EXIFNamespace, "FocalLength", fmt.Sprintf("%.0fmm", fl), float64(fl))
	}
	if ts := libraw.ImgOther_timestamp(other); ts > 0 {
		// libraw derives timestamp via mktime() on the EXIF wall-clock
		// string (which carries no timezone), so the epoch is only
		// meaningful when re-localized on the same host that parsed it.
		// Recover the wall-clock fields via time.Local (the inverse of
		// libraw's mktime) and relabel them as UTC, rather than
		// converting, so the result doesn't depend on the host timezone.
		local := time.Unix(ts, 0)
		t := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), 0, time.UTC)
		add(exif.EXIFNamespace, "DateTimeOriginal", t.Format(time.RFC3339), t)
	}
	text(dc.Namespace, dc.Description, libraw.ImgOther_desc(other))
	text(dc.Namespace, dc.Creator, libraw.ImgOther_artist(other))

	return result
}

func shutterStr(s float32) string {
	if s >= 1 {
		return fmt.Sprintf("%.1fs", s)
	}
	return fmt.Sprintf("1/%.0f", 1.0/s)
}
