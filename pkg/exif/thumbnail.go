//go:build cgo

package exif

import (
	// Packages
	tiff "github.com/mutablelogic/go-metadata/internal/tiff"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Thumbnail returns the thumbnail which is embedded in the EXIF data, which
// is usually a small JPEG image, such as 160x120, or nil when there isn't
// one (see tiff.Thumbnail). libexif reads the thumbnail, but doesn't expose
// it, or keep the tags of where it is, so they're read from the data.
func (e *EXIF) Thumbnail() []byte {
	return tiff.Thumbnail(e.raw)
}

// Orientation returns the orientation of the image, from 1 to 8, as EXIF
// defines it, or 1 when it doesn't have one (see tiff.Orientation)
func (e *EXIF) Orientation() int {
	return tiff.Orientation(e.raw)
}
