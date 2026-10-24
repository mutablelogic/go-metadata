package postscript

import (
	"context"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// noContentNote is the markdown:note of a document, as its content isn't
// read
const noContentNote = "The content of this document can't be read yet, as it's PostScript, which is a program that draws its pages, rather than text."

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// A note that the content of the document can't be read, only when the
	// markdown namespace is requested
	metadata.AddHandler(reContentType, Namespace+"-markdown", func(_ context.Context, _ io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		return []metadata.Metadata{meta.New(metadata.MarkdownNamespace, "note", noContentNote)}, nil
	}, metadata.MarkdownNamespace)
}
