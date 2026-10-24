package html

import (
	"context"
	"io"

	// Packages
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	converter "github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The content of the document as Markdown, only when the markdown
	// namespace is requested, as converting the document is expensive
	metadata.AddHandler(reContentType, "html-markdown", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		doc, err := parse(r)
		if err != nil {
			return nil, err
		}
		data, err := htmltomarkdown.ConvertNode(doc, converter.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		w := imarkdown.NewWriter(o)
		_, err = w.Write(data)
		return w.Metadata(err)
	}, metadata.MarkdownNamespace)
}
