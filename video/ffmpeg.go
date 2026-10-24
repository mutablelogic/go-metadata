//go:build cgo

package video

import (
	// The handlers of videos, which use ffmpeg, so need cgo
	_ "github.com/mutablelogic/go-metadata/video/ffmpeg"
)
