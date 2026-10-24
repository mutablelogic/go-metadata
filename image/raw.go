//go:build cgo

package image

import (
	// The handlers of RAW images, which use libraw and libexif, so need cgo
	_ "github.com/mutablelogic/go-metadata/image/raw"
)
