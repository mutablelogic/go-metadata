package chromaprint

import (
	"errors"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// MaxDuration is the most of audio which is fingerprinted, which is enough
// for AcoustID to match it
const MaxDuration = 120 * time.Second

var (
	// ErrNoAudio is returned by Fingerprint for audio which has no samples
	// to fingerprint
	ErrNoAudio = errors.New("chromaprint: no audio to fingerprint")

	// ErrNotEnabled is returned by Fingerprint when it's built without cgo
	// or the chromaprint tag, as audio is only fingerprinted with them
	ErrNotEnabled = errors.New("chromaprint: not enabled, as it's built without cgo or the chromaprint tag")
)
