// Package video registers the handlers of videos, and of their subtitles,
// so importing it anonymously registers them all. The handlers of videos
// use ffmpeg, so are only registered when built with cgo.
package video

import (
	// Handlers
	_ "github.com/mutablelogic/go-metadata/video/subtitle"
)
