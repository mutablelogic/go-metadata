// Package kind registers the kind of audio, of its content type, which is
// returned when nothing else returns one, such as without cgo, when ffmpeg
// doesn't read it, or when it fails to, so it isn't unknown. It's pure Go,
// so a program which isn't built with cgo, or ffmpeg, can import it, as the
// audio package, which imports it, isn't.
package kind

import (
	"context"
	"io"
	"regexp"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// reContentType is the content type of audio, which the fallback reads
	reContentType = regexp.MustCompile(`^audio/.+$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// The kind of audio, of its content type, which is returned when nothing
// else returns one, such as without cgo, when ffmpeg doesn't read it, or
// when it fails to, so it isn't unknown. A playlist, whose content type is
// audio's too, has its own.
func init() {
	metadata.AddHandlerWithPriority(reContentType, "audio-kind", metadata.FallbackPriority, func(context.Context, io.Reader, *metadata.Opts) ([]metadata.Metadata, error) {
		return metadata.KindMetadata(dc.KindAudio), nil
	}, metadata.MetadataNamespace, dc.Namespace)
}
