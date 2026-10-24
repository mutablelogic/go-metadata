//go:build cgo

package photoshop

import (
	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	imageexif "github.com/mutablelogic/go-metadata/image/exif"
	exif "github.com/mutablelogic/go-metadata/pkg/exif"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// exifHeader is before the TIFF data of EXIF metadata in a JPEG image,
// which libexif reads it after, but which the EXIF resource of a document
// doesn't have
const exifHeader = "Exif\x00\x00"

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The EXIF metadata of a document, as the JPEG handler returns it
	exifMetadata = func(data []byte) []metadata.Metadata {
		f, err := exif.Parse(append([]byte(exifHeader), data...))
		if err != nil {
			return nil
		}
		defer f.Close()
		return imageexif.TagsMetadata(f.Tags())
	}
}
