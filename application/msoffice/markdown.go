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

// noContentNotes are the markdown:note of a document, by its content type,
// as its content isn't read
var noContentNotes = map[string]string{
	"application/msword":            "The content of this document can't be read yet, as it's in the binary format of Microsoft Office from before 2007, rather than the format of a .docx document.",
	"application/vnd.ms-excel":      "The content of this spreadsheet can't be read yet, as it's in the binary format of Microsoft Office from before 2007, rather than the format of a .xlsx spreadsheet.",
	"application/vnd.ms-powerpoint": "The content of this presentation can't be read yet, as it's in the binary format of Microsoft Office from before 2007, rather than the format of a .pptx presentation.",
	"application/vnd.visio":         "The content of this drawing can't be read yet, as it's in the binary format of Visio from before 2013, rather than the format of a .vsdx drawing.",
	"application/vnd.ms-project":    "The content of this plan can't be read yet, as it's in the binary format of Microsoft Project.",
	"application/vnd.ms-works":      "The content of this document can't be read yet, as it's in the binary format of Microsoft Works.",
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// A note that the content of the document can't be read, only when the
	// markdown namespace is requested
	metadata.AddHandler(reContentType, Namespace+"-markdown", func(_ context.Context, _ io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		return []metadata.Metadata{meta.New(metadata.MarkdownNamespace, "note", noContentNotes[o.ContentType()])}, nil
	}, metadata.MarkdownNamespace)
}
