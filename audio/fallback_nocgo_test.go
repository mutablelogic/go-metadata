//go:build !cgo

package audio_test

import (
	"path/filepath"
	"testing"

	// Packages
	dc "github.com/mutablelogic/go-metadata/dublincore"
)

func Test_FallbackKindNoCgo(t *testing.T) {
	// Without cgo, audio's kind is of its content type
	kind, typ := kind(t, filepath.Join("..", "testfiles", "sample.mp3"))
	assert.Equal(t, dc.KindAudio, kind)
	assert.Equal(t, dc.TypeOf(dc.KindAudio), typ)
}
