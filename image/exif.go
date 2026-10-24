//go:build cgo

package image

import (
	// The handler of EXIF metadata, which uses libexif, so needs cgo
	_ "github.com/mutablelogic/go-metadata/image/exif"
)
