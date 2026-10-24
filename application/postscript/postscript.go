// Package postscript registers a handler of PostScript documents and EPS
// files, such as .ps, .eps and older Illustrator .ai files, which returns
// their kind, what their comments say about them, such as their title, who
// they were made for, when, and their pages and size, and the preview of an
// EPS file, as artwork
package postscript

import (
	"bytes"
	"context"
	"image/png"
	"io"
	"math"
	"mime"
	"regexp"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	tiff "github.com/mutablelogic/go-metadata/internal/tiff"
	title "github.com/mutablelogic/go-metadata/internal/title"
	postscript "github.com/mutablelogic/go-metadata/pkg/postscript"

	// Image decoders, for an EPS file's TIFF preview
	_ "golang.org/x/image/tiff"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the properties of a document which
	// aren't Dublin Core
	Namespace = "postscript"

	// previewKey is the key of the preview of an EPS file, as artwork
	previewKey = "preview"
)

var (
	reContentType = regexp.MustCompile(`^(?:application/(?:postscript|eps|x-eps)|image/(?:eps|x-eps))$`)

	// extensions are the extensions of PostScript documents and EPS files,
	// which are detected by their extension when they have a DOS EPS
	// header, which isn't text. An .ai file from Illustrator 9 or later is
	// PDF, which is detected as PDF.
	extensions = []string{".ps", ".eps", ".epsf", ".epsi", ".ai"}

	// reFileName matches a title which is the name of the file the document
	// was made from, such as kddj.dvi, which dvips gives a document, rather
	// than its title
	reFileName = regexp.MustCompile(`(?i)^[^\s/\\:]+\.(?:dvi|e?ps|epsf|ai|tex|docx?|pdf|indd|qxd|p65|pm\d?|cdr|fh\d+)$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	for _, ext := range extensions {
		if err := mime.AddExtensionType(ext, postscript.ContentType); err != nil {
			panic(err)
		}
	}

	// The kind of a PostScript document, what its comments say about it, and
	// the preview of an EPS file, as artwork, only when the artwork
	// namespace is requested
	metadata.AddHandler(reContentType, Namespace, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		ra, size, err := readerAt(r)
		if err != nil {
			return nil, err
		}
		doc, err := postscript.Parse(ra, size)
		if err != nil {
			return nil, metadata.ErrBadParameter.With(err)
		}
		result := append(documentMetadata(doc), metadata.KindMetadata(kind(doc))...)
		if o.HasNamespace(metadata.ArtworkNamespace) && doc.Preview != nil {
			if preview := previewArtwork(doc.Preview, o); preview != nil {
				result = append(result, preview)
			}
		}
		return result, nil
	}, Namespace, dc.Namespace, metadata.MetadataNamespace, metadata.ArtworkNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// kind returns the kind of a document, which is a drawing for an EPS file,
// or a document from Illustrator, and otherwise a document
func kind(doc *postscript.Document) string {
	if doc.EPS != "" || strings.Contains(doc.Creator, "Illustrator") {
		return dc.KindDrawing
	}
	return dc.KindDocument
}

// documentMetadata returns the metadata of a document: its title, unless
// it's the name of the file it was made from, who it was made for, as its
// creator, and when, as Dublin Core, and the application which made it, its
// pages, the size of paper it's for, its width and height in millimetres,
// from its bounding box, the fonts it uses, and its language level
func documentMetadata(doc *postscript.Document) []metadata.Metadata {
	var result []metadata.Metadata
	add := func(ns, key string, value any) {
		if entry := meta.New(ns, key, value); entry != nil {
			result = append(result, entry)
		}
	}
	if doc.Title != "" && !title.Untitled(doc.Title) && !reFileName.MatchString(doc.Title) {
		add(dc.Namespace, dc.Title, doc.Title)
	}
	if doc.For != "" {
		add(dc.Namespace, dc.Creator, doc.For)
	}
	if !doc.Created.IsZero() {
		add(dc.Namespace, dc.Date, doc.Created)
	}
	if doc.Creator != "" {
		add(Namespace, "application", doc.Creator)
	}
	if doc.Pages > 0 {
		add(Namespace, "pages", doc.Pages)
	}
	if paper := doc.Paper(); paper != "" {
		add(Namespace, "paper", paper)
	}
	if box := doc.BoundingBox; box != nil && box.Width() > 0 && box.Height() > 0 {
		add(Namespace, "width-mm", millimetres(box.Width()))
		add(Namespace, "height-mm", millimetres(box.Height()))
	}
	if len(doc.Fonts) > 0 {
		result = append(result, meta.NewText(Namespace, "fonts", doc.Fonts, strings.Join(doc.Fonts, ", ")))
	}
	if doc.LanguageLevel > 0 {
		add(Namespace, "language-level", doc.LanguageLevel)
	}
	return result
}

// previewArtwork returns the TIFF preview of an EPS file as artwork, or nil
// when it can't be read. A palette image with an alpha channel, as
// Illustrator writes, is read first, as golang.org/x/image/tiff reads it
// wrongly, without an error. Another image's tags are sorted, when it can't
// be read as it is, as some applications don't sort them.
func previewArtwork(data []byte, o *metadata.Opts) metadata.Metadata {
	if img := tiff.PaletteAlpha(data); img != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil
		}
		return artwork.Image(buf.Bytes(), previewKey, o)
	}
	if preview := artwork.Image(data, previewKey, o); preview != nil {
		return preview
	}
	return artwork.Image(tiff.SortTags(data), previewKey, o)
}

// millimetres returns a length in points, a 72nd of an inch, in
// millimetres, to a tenth of a millimetre
func millimetres(points float64) float64 {
	return math.Round(points*25.4/72*10) / 10
}

// readerAt returns the document to read at any offset, and its size, which
// is the reader itself when it can be, as Extract's readers can, and
// otherwise the document read into memory
func readerAt(r io.Reader) (io.ReaderAt, int64, error) {
	if ra, ok := r.(interface {
		io.ReaderAt
		Size() int64
	}); ok {
		return ra, ra.Size(), nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, err
	}
	return bytes.NewReader(data), int64(len(data)), nil
}
