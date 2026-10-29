//go:build cgo && libfvad

package audio

import (
	// The handler which classifies audio, which uses go-media's segmenter,
	// so needs cgo and libfvad
	_ "github.com/mutablelogic/go-metadata/audio/classify"
)
