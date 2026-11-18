//go:build !cgo

package video_test

import (
	"path/filepath"
	"testing"

	// Packages
	dc "github.com/mutablelogic/go-metadata/dublincore"
)

func Test_FallbackKindNoCgo(t *testing.T) {
	// Without cgo, video's kind is of its content type
	assert.Equal(t, dc.KindVideo, kind(t, filepath.Join("..", "testfiles", "sample.mp4")))
}
