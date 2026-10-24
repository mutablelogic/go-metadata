//go:build cgo && chromaprint

package acoustid

import (
	// Packages
	chromaprint "github.com/mutablelogic/go-metadata/pkg/chromaprint"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The fingerprint of audio which doesn't have one in its tags, from its
	// audio, with Chromaprint
	fingerprint = chromaprint.Fingerprint
}
