//go:build !(cgo && chromaprint)

package chromaprint

import (
	"context"
	"errors"
	"io"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Enabled is false, as audio is only fingerprinted when it's built with cgo
// and the chromaprint tag
const Enabled = false

// ErrNotEnabled is returned by Fingerprint, as audio is only fingerprinted
// when it's built with cgo and the chromaprint tag
var ErrNotEnabled = errors.New("chromaprint: not enabled, as it's built without cgo or the chromaprint tag")

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Fingerprint returns ErrNotEnabled, as audio is only fingerprinted when
// it's built with cgo and the chromaprint tag
func Fingerprint(context.Context, io.Reader) (string, time.Duration, error) {
	return "", 0, ErrNotEnabled
}
