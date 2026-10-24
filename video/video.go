// Package video registers the handlers of videos, and of their subtitles,
// and the lookup of videos on TMDB, so importing it anonymously registers
// them all. The handlers of videos
// use ffmpeg, so are only registered when built with cgo.
package video

import (
	// Handlers
	_ "github.com/mutablelogic/go-metadata/video/subtitle"
	_ "github.com/mutablelogic/go-metadata/video/tmdb"
)
