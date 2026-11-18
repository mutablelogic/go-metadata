package video

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
	// reContentType is the content type of video, which the fallback reads
	reContentType = regexp.MustCompile(`^video/.+$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// The kind of video, of its content type, which is returned when nothing
// else returns one, such as without cgo, when ffmpeg doesn't read it, or
// when it fails to, so it isn't unknown.
func init() {
	metadata.AddHandlerWithPriority(reContentType, "video-kind", metadata.FallbackPriority, func(context.Context, io.Reader, *metadata.Opts) ([]metadata.Metadata, error) {
		return metadata.KindMetadata(dc.KindVideo), nil
	}, metadata.MetadataNamespace, dc.Namespace)
}
