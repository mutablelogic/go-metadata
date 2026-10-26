// Package iwork registers a handler of documents of Apple's iWork
// applications, Pages, Keynote and Numbers, which are zip archives, which
// returns their kind, the properties of a document from iWork '09, such as
// its title and authors, the template and version of a document from iWork
// 2013 or later, and their previews, as artwork
package iwork

import (
	"bytes"
	"context"
	"io"
	"mime"
	"regexp"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	iwork "github.com/mutablelogic/go-metadata/pkg/iwork"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the properties of a document which
	// aren't Dublin Core
	Namespace = "iwork"

	// previewKey is the key of the preview of a document, as artwork
	previewKey = "preview"
)

var (
	reContentType = regexp.MustCompile(`^application/vnd\.apple\.(?:pages|keynote|numbers)$`)

	// iworkTypes are the content types of documents, by their extension,
	// which refine a zip archive
	iworkTypes = map[string]string{
		".pages":   iwork.ContentTypePages,
		".key":     iwork.ContentTypeKeynote,
		".numbers": iwork.ContentTypeNumbers,
	}

	// iworkKinds are the kinds of documents, by their content type
	iworkKinds = map[string]string{
		iwork.ContentTypePages:   dc.KindDocument,
		iwork.ContentTypeKeynote: dc.KindPresentation,
		iwork.ContentTypeNumbers: dc.KindSpreadsheet,
	}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	for ext, contentType := range iworkTypes {
		if err := mime.AddExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	// The kind of a document, its properties, the artwork hint, when it has
	// a preview, and its preview, as artwork, only when the artwork
	// namespace is requested
	metadata.AddHandler(reContentType, Namespace, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(iworkKinds[o.ContentType()])
		ra, size, err := readerAt(r)
		if err != nil {
			return kind, err
		}
		doc, err := iwork.Parse(ra, size)
		if err != nil {
			return kind, metadata.ErrBadParameter.With(err)
		}
		result := append(documentMetadata(doc), kind...)
		if doc.Preview != nil {
			// A document with a preview has artwork, but its content can't
			// be read as Markdown yet
			result = append(result, metadata.HintMetadata(metadata.HintArtwork)...)
		}
		if o.HasNamespace(metadata.ArtworkNamespace) && doc.Preview != nil {
			if preview := artwork.Image(doc.Preview, previewKey, o); preview != nil {
				result = append(result, preview)
			}
		}
		return result, nil
	}, Namespace, dc.Namespace, metadata.MetadataNamespace, metadata.ArtworkNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// documentMetadata returns the properties of a document: its title, authors,
// comment, copyright, and when it was made, as Dublin Core, and its
// keywords, projects, template, and the version of the application which
// last saved it
func documentMetadata(doc *iwork.Document) []metadata.Metadata {
	var result []metadata.Metadata
	add := func(ns, key string, value any) {
		if entry := meta.New(ns, key, value); entry != nil {
			result = append(result, entry)
		}
	}
	list := func(key string, values []string) {
		if len(values) > 0 {
			result = append(result, meta.NewText(Namespace, key, values, strings.Join(values, ", ")))
		}
	}
	if doc.Title != "" {
		add(dc.Namespace, dc.Title, doc.Title)
	}
	if len(doc.Authors) > 0 {
		add(dc.Namespace, dc.Creator, strings.Join(doc.Authors, ", "))
	}
	if doc.Comment != "" {
		add(dc.Namespace, dc.Description, doc.Comment)
	}
	if doc.Copyright != "" {
		add(dc.Namespace, dc.Rights, doc.Copyright)
	}
	if !doc.Created.IsZero() {
		add(dc.Namespace, dc.Date, doc.Created)
	}
	list("keywords", doc.Keywords)
	list("projects", doc.Projects)
	if doc.Template != "" {
		add(Namespace, "template", doc.Template)
	}
	if doc.Version != "" {
		add(Namespace, "version", doc.Version)
	}
	return result
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
