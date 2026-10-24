// Package plain registers a handler of plain text, which returns its
// content as Markdown, decoded from its character set
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
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The content of the document as Markdown, only when the markdown
	// namespace is requested
	metadata.AddHandler(reContentType, "plain", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindDocument)
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return kind, nil
		}
		text, err := itext.Read(r)
		if err != nil {
			return kind, err
		}
		w := imarkdown.NewWriter(o)
		meta, err := w.Metadata(itext.WriteMarkdown(w, text))
		return append(meta, kind...), err
	}, metadata.MarkdownNamespace, metadata.MetadataNamespace, dc.Namespace)
}
