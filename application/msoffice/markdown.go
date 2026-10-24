package msoffice

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
	// noContentNote is the markdown:note of a document, as its content isn't
	// read
	noContentNote = "The content of this document can't be read yet, as it's in the binary format of Microsoft Office from before 2007, rather than the format of a .docx, .xlsx or .pptx document."
)

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
