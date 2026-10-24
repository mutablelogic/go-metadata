package raster

import (
	"context"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// noContentNote is the markdown:note of an image, as it has no text
	noContentNote = "An image has no content as Markdown, as it isn't text. Read its metadata, such as its size, or its artwork, instead."
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// A note that an image has no content as Markdown, only when the
	// markdown namespace is requested
	metadata.AddHandler(reContentType, Namespace+"-markdown", func(_ context.Context, _ io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		return []metadata.Metadata{meta.New(metadata.MarkdownNamespace, "note", noContentNote)}, nil
	}, metadata.MarkdownNamespace)
}
