// Package plain registers a handler of plain text, which returns its
// content as Markdown, decoded from its character set, and of text which no
// other handler reads, as plain text
package plain

import (
	"context"
	"io"
	"regexp"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	itext "github.com/mutablelogic/go-metadata/internal/text"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	reContentType = regexp.MustCompile(`^text/plain$`)

	// reText matches any type of text
	reText = regexp.MustCompile(`^text/`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The content of the document as Markdown, only when the markdown
	// namespace is requested
	metadata.AddHandler(reContentType, "plain", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		result := append(metadata.KindMetadata(dc.KindDocument), hints(r)...)
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return result, nil
		}
		meta, err := markdown(r, o)
		return append(meta, result...), err
	}, metadata.MarkdownNamespace, metadata.MetadataNamespace, dc.Namespace)

	// The content of text which no other handler reads, such as text/x-log,
	// as plain text, only when the markdown namespace is requested, and the
	// hint that it has text, with the least priority, so it's only the
	// content of text without another handler
	metadata.AddHandlerWithPriority(reText, "plain-text", metadata.DefaultPriority-1, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if len(metadata.GetHandlers(o.ContentType())) > 1 {
			return nil, nil
		}
		result := hints(r)
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return result, nil
		}
		meta, err := markdown(r, o)
		return append(meta, result...), err
	}, metadata.MarkdownNamespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// hints returns the hint that the text can be read as Markdown, when it has
// text, which is read without moving r
func hints(r io.Reader) []metadata.Metadata {
	if itext.HasText(r) {
		return metadata.HintMetadata(metadata.HintMarkdown)
	}
	return nil
}

// markdown returns the part of the text as Markdown in the range, which is
// read as it's converted, until the part is converted, so a text of any size
// is read a part at a time
func markdown(r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	w := imarkdown.NewWriter(o)
	return w.Metadata(itext.WriteMarkdownFrom(w, itext.NewReader(r)))
}
