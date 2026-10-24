// Package audio registers the handlers of audio, and of playlists, so
// importing it anonymously registers them all. The handlers of audio use
// ffmpeg, so are only registered when built with cgo.
package audio

import (
	// Handlers
	_ "github.com/mutablelogic/go-metadata/audio/playlist"
)
