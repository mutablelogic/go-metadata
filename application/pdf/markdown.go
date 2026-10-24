package pdf

import (
	"context"
	"errors"
	"io"
	"strings"

	// Packages
	textpdf "github.com/giraffesyo/pdf"
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	itext "github.com/mutablelogic/go-metadata/internal/text"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// pageBreak separates the text of the pages in the Markdown
	pageBreak = "\n\n---\n\n"

	// noTextNote is the markdown:note of a document which has no text
	noTextNote = "This PDF document has no text to extract, as its pages are images, such as scanned pages, which need OCR to be read."
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The text of the pages as Markdown, only when the markdown namespace is
	// requested
	metadata.AddHandler(reContentType, Namespace+"-markdown", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.MarkdownNamespace) {
			return nil, nil
		}
		return pdfMarkdown(ctx, r, o)
	}, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// pdfMarkdown returns the text of the pages of a PDF document as Markdown,
// in reading order, as it's shown, with a rule between the pages, without
// the pages which have no text, such as scanned pages, and without headers,
// footers and page numbers which are marked as such, or a markdown:note
// which says so when no page has text. The pages are read one at a time, until the Markdown in the
// range is read. A document which needs
// a password, without one, has no Markdown, and with a wrong one, is an
// error. A document larger than maxDocumentSize has no Markdown.
func pdfMarkdown(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	ra, size, ok, err := readerAt(r)
	if err != nil || !ok {
		return nil, err
	}

	w := imarkdown.NewWriter(o)
	first := true
	opts := textpdf.Options{Password: o.Password(), IgnoreArtifacts: true}
	_, err = textpdf.ExtractPages(ctx, ra, size, opts, func(page textpdf.Page) error {
		text := strings.TrimSpace(page.Text())
		if text == "" {
			return nil
		}
		if !first {
			if _, err := w.WriteString(pageBreak); err != nil {
				return err
			}
		}
		first = false
		return itext.WriteMarkdown(w, text)
	})
	if errors.Is(err, textpdf.ErrPasswordRequired) {
		return nil, encryptedError{}.wrongPassword(o.Password())
	} else if first && err == nil {
		// No page has text, such as a scanned document
		return []metadata.Metadata{meta.New(metadata.MarkdownNamespace, "note", noTextNote)}, nil
	}
	return w.Metadata(err)
}

// readerAt returns the document to read at any offset, and its size, or
// false when it's larger than maxDocumentSize
func readerAt(r io.Reader) (io.ReaderAt, int64, bool, error) {
	rs, ok, err := readSeeker(r)
	if err != nil || !ok {
		return nil, 0, false, err
	}
	ra, ok := rs.(io.ReaderAt)
	if !ok {
		return nil, 0, false, metadata.ErrInternalError.With("the pdf document can't be read at an offset")
	}
	size, err := rs.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, 0, false, err
	}
	return ra, size, true, nil
}
