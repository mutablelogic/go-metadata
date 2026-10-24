//go:build cgo

package image

import (
	// The handlers of HEIF images, which use libheif, so need cgo
	_ "github.com/mutablelogic/go-metadata/image/heif"
)
