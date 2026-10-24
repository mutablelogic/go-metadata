// Package image registers the handlers of images, such as raster images and
// SVG images, and EXIF metadata when built with cgo, so importing it
// anonymously registers them all
package image

import (
	// Handlers
	_ "github.com/mutablelogic/go-metadata/image/raster"
	_ "github.com/mutablelogic/go-metadata/image/svg"
)
