package epub

import (
	"archive/zip"
	"context"
	"io"
	"net/url"
	"path"
	"strings"

	// Packages
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	converter "github.com/JohannesKaufmann/html-to-markdown/v2/converter"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// chapterBreak separates the chapters of a book in the Markdown
	chapterBreak = "\n\n---\n\n"

	// maxChapterSize is the largest chapter which is read, in bytes
	maxChapterSize = 16 << 20
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// markdown writes the text of the book as Markdown: the documents of its
// spine, in the order they're read, which aren't auxiliary, such as notes
// which are only linked to, with a rule between them, converted from XHTML.
// A document which can't be read, or has no text, is skipped. It stops when
// a write returns an error. The paths of the documents are relative to the
// package document's directory.
func (pkg *epubPackage) markdown(ctx context.Context, archive *zip.Reader, dir string, w io.StringWriter) error {
	hrefs := make(map[string]string, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		if strings.Contains(item.MediaType, "html") {
			hrefs[item.ID] = item.Href
		}
	}

	first := true
	for _, itemref := range pkg.Spine {
		if err := ctx.Err(); err != nil {
			return err
		}
		href, exists := hrefs[itemref.IDRef]
		if !exists || itemref.Linear == "no" {
			continue
		}
		text := chapter(ctx, archive, dir, href)
		if text == "" {
			continue
		}
		if !first {
			if _, err := w.WriteString(chapterBreak); err != nil {
				return err
			}
		}
		first = false
		if _, err := w.WriteString(text); err != nil {
			return err
		}
	}
	return nil
}

// chapter returns the document of the book as Markdown, without the space at
// its start and end, or an empty string when it can't be read
func chapter(ctx context.Context, archive *zip.Reader, dir, href string) string {
	if unescaped, err := url.PathUnescape(href); err == nil {
		href = unescaped
	}
	f, err := archive.Open(path.Join(dir, href))
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := htmltomarkdown.ConvertReader(io.LimitReader(f, maxChapterSize), converter.WithContext(ctx))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
