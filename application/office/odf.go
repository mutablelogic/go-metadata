package office

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	title "github.com/mutablelogic/go-metadata/internal/title"
	zipdoc "github.com/mutablelogic/go-metadata/internal/zipdoc"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// odfMeta is the metadata of an OpenDocument document, in meta.xml, whose
// author is its initial creator, as its creator is who last changed it
type odfMeta struct {
	Meta struct {
		Title          string `xml:"http://purl.org/dc/elements/1.1/ title"`
		Subject        string `xml:"http://purl.org/dc/elements/1.1/ subject"`
		Description    string `xml:"http://purl.org/dc/elements/1.1/ description"`
		Creator        string `xml:"http://purl.org/dc/elements/1.1/ creator"`
		Language       string `xml:"http://purl.org/dc/elements/1.1/ language"`
		InitialCreator string `xml:"urn:oasis:names:tc:opendocument:xmlns:meta:1.0 initial-creator"`
		CreationDate   string `xml:"urn:oasis:names:tc:opendocument:xmlns:meta:1.0 creation-date"`
	} `xml:"urn:oasis:names:tc:opendocument:xmlns:office:1.0 meta"`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// odfMetaPath is the path of an OpenDocument document's metadata, and
	// odfThumbnailPath of its thumbnail, which every document has
	odfMetaPath      = "meta.xml"
	odfThumbnailPath = "Thumbnails/thumbnail.png"
)

// odfDates are the layouts of an OpenDocument document's dates, which are
// usually without a time zone
var odfDates = []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02"}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// OpenDocument text documents, spreadsheets and presentations, and their
	// templates, whose types are registered with the office documents'
	metadata.AddHandler(regexp.MustCompile(`^application/vnd\.oasis\.opendocument\.(?:text|spreadsheet|presentation)(?:-template)?$`), "odf", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		meta, err := odfMetadata(r, o)
		return append(meta, metadata.KindMetadata(officeKind(o.ContentType()))...), err
	}, dc.Namespace, metadata.ArtworkNamespace, metadata.MarkdownNamespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// odfMetadata returns the metadata of an OpenDocument document as dc:title,
// dc:subject, dc:description, dc:creator, dc:language and dc:date, when
// they're set, and its thumbnail, which is a picture of its first page,
// sheet or slide, as artwork:thumbnail, when artwork is requested, and its
// content as Markdown, when it's requested, a part at a time (see
// metadata.WithRange), and its hints (see documentHints). A title which is
// the name of the file the document was made from isn't returned, so the
// title is from the document's own name.
func odfMetadata(r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	// A document is a zip archive, which is read in place, or from memory,
	// unless it's too large, when it has only the title from its name
	archive, ok, err := zipdoc.Open(r, maxDocumentSize)
	if errors.Is(err, zip.ErrFormat) || errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, metadata.ErrBadParameter.Withf("not an opendocument document: %v", err)
	} else if err != nil || !ok {
		return nil, err
	}

	// Read the metadata, when there is any
	var doc odfMeta
	if err := zipdoc.ReadXML(archive, odfMetaPath, &doc); err != nil && !errors.Is(err, metadata.ErrNotFound) {
		return nil, err
	}
	props := doc.Meta
	docTitle := props.Title
	if title.Untitled(docTitle) {
		docTitle = ""
	}
	creator := props.InitialCreator
	if strings.TrimSpace(creator) == "" {
		creator = props.Creator
	}

	// Return the metadata which is set
	entries := make(map[string]metadata.Metadata)
	for _, p := range []struct{ key, value string }{
		{dc.Title, docTitle},
		{dc.Subject, props.Subject},
		{dc.Description, props.Description},
		{dc.Creator, creator},
		{dc.Language, props.Language},
	} {
		addEntry(entries, dc.Namespace, p.key, strings.TrimSpace(p.value))
	}
	for _, layout := range odfDates {
		if created, err := time.Parse(layout, strings.TrimSpace(props.CreationDate)); err == nil {
			addEntry(entries, dc.Namespace, dc.Date, created)
			break
		}
	}

	// The thumbnail, when artwork is requested
	if o.HasNamespace(metadata.ArtworkNamespace) {
		if thumbnail := zipdoc.Image(archive, odfThumbnailPath, thumbnailKey, o); thumbnail != nil {
			entries[metadata.ArtworkNamespace+string(metadata.Separator)+thumbnailKey] = thumbnail
		}
	}

	// The content as Markdown, when it's requested
	if err := addMarkdown(entries, archive, o, odfMarkdown); err != nil {
		return nil, err
	}
	_, err = fs.Stat(archive, odfContentPath)
	return append(slices.Collect(maps.Values(entries)), documentHints(archive, err == nil, odfThumbnailPath)...), nil
}
