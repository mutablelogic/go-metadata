//go:build cgo

package audio

import (
	// The handlers of audio, which use ffmpeg, so need cgo
	_ "github.com/mutablelogic/go-metadata/audio/ffmpeg"
)
