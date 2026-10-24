package archive

import (
	"context"
	"fmt"
	"io"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	itext "github.com/mutablelogic/go-metadata/internal/text"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// tocHeader is the header of the table of contents of an archive
	tocHeader = "| Name | Size | Modified |\n| --- | ---: | --- |\n"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The table of contents of the archive, as Markdown, only when the
	// markdown namespace is requested
	metadata.AddHandler(reContentType, Namespace+"-markdown", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		return archiveMarkdown(ctx, r, o)
	}, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// archiveMarkdown returns the table of contents of the archive, as a
// Markdown table of the name of each entry, its size in bytes, and when it
// was modified, in the order they're in the archive, with a slash after the
// name of a directory, which has no size. The entries are read until the
// Markdown in the range is read.
func archiveMarkdown(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	w := imarkdown.NewWriter(o)
	if _, err := w.WriteString(tocHeader); err != nil {
		return w.Metadata(err)
	}
	info, err := walk(ctx, r, o.ContentType(), func(e entry) error {
		_, err := w.WriteString(tocRow(e))
		return err
	})
	if info.format == "" {
		return nil, err
	}
	return w.Metadata(err)
}

// tocRow returns the row of the table of contents of an entry
func tocRow(e entry) string {
	name, size := e.name, fmt.Sprint(e.size)
	if e.dir {
		if name != "" && name[len(name)-1] != '/' {
			name += "/"
		}
		size = ""
	}
	var modified string
	if !e.modified.IsZero() {
		modified = e.modified.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf("| %s | %s | %s |\n", itext.Escape(name), size, modified)
}
