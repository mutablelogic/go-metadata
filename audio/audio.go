// Package audio registers the handlers of audio, and of playlists, and the
// lookup of audio on AcoustID, so importing it anonymously registers them
// all. The handlers of audio use ffmpeg, so are only registered when built
// with cgo, and, without it, or when ffmpeg can't read it, the kind of audio
// is of its content type.
package audio

import (
	// Handlers
	_ "github.com/mutablelogic/go-metadata/audio/acoustid"
	_ "github.com/mutablelogic/go-metadata/audio/kind"
	_ "github.com/mutablelogic/go-metadata/audio/playlist"
)
