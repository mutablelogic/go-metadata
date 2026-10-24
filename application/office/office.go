// Package office registers handlers of office documents, which are Word,
// Excel and PowerPoint documents, and OpenDocument text documents,
// spreadsheets and presentations, which are zip archives of XML, and which
// return their properties, their thumbnail, and their content as Markdown
package office

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"maps"
	"mime"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	title "github.com/mutablelogic/go-metadata/internal/title"
	zipdoc "github.com/mutablelogic/go-metadata/internal/zipdoc"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// coreProperties are the properties of an Office Open XML document, such as
// a Word document, in docProps/core.xml
type coreProperties struct {
	Title       string `xml:"http://purl.org/dc/elements/1.1/ title"`
	Subject     string `xml:"http://purl.org/dc/elements/1.1/ subject"`
	Creator     string `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Description string `xml:"http://purl.org/dc/elements/1.1/ description"`
	Created     string `xml:"http://purl.org/dc/terms/ created"`
}

// packageRelationships are the relationships of an Office Open XML
// document's package, in _rels/.rels, such as to its thumbnail
type packageRelationships struct {
	Relationships []struct {
		Type       string `xml:"Type,attr"`
		Target     string `xml:"Target,attr"`
		TargetMode string `xml:"TargetMode,attr"`
	} `xml:"Relationship"`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// corePropertiesPath is the path of the properties within the document
	corePropertiesPath = "docProps/core.xml"

	// packageRelationshipsPath is the path of the package's relationships
	packageRelationshipsPath = "_rels/.rels"

	// thumbnailRelationship is the type of the relationship to the
	// document's thumbnail, and thumbnailPath its usual path
	thumbnailRelationship = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail"
	thumbnailPath         = "docProps/thumbnail.jpeg"

	// thumbnailKey is the key of the thumbnail of a document, which is a
	// picture of its first page, sheet or slide
	thumbnailKey = "thumbnail"
)

var (
	// maxDocumentSize is the largest document which is read into memory,
	// when it can't be read in place. A larger document has only the title
	// from its name.
	maxDocumentSize int64 = 64 << 20
)

// officeTypes are the content types of documents which are zip archives, by
// their extension, which are detected as zip archives, and refined by their
// extension
var officeTypes = map[string]string{
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".odt":  "application/vnd.oasis.opendocument.text",
	".ods":  "application/vnd.oasis.opendocument.spreadsheet",
	".odp":  "application/vnd.oasis.opendocument.presentation",
	".ott":  "application/vnd.oasis.opendocument.text-template",
	".ots":  "application/vnd.oasis.opendocument.spreadsheet-template",
	".otp":  "application/vnd.oasis.opendocument.presentation-template",
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	for ext, contentType := range officeTypes {
		if err := mime.AddExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	// Word, Excel and PowerPoint documents
	metadata.AddHandler(regexp.MustCompile(`^application/vnd\.openxmlformats-officedocument\.(?:wordprocessingml\.document|spreadsheetml\.sheet|presentationml\.presentation)$`), "office", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		meta, err := officeMetadata(r, o)
		return append(meta, metadata.KindMetadata(officeKind(o.ContentType()))...), err
	}, dc.Namespace, metadata.ArtworkNamespace, metadata.MarkdownNamespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// officeMetadata returns the properties of an Office Open XML document as
// dc:title, dc:subject, dc:creator, dc:description and dc:date, when they're
// set, its thumbnail, which is a picture of its first page or slide, as
// artwork:thumbnail, when artwork is requested, and it has one, and its
// content as Markdown, when it's requested, a part at a time (see
// metadata.WithRange). A title which is the name of the file the document
// was made from isn't returned, so the title is from the document's own
// name.
func officeMetadata(r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	// A document is a zip archive, which is read in place, or from memory,
	// unless it's too large, when it has only the title from its name
	archive, ok, err := zipdoc.Open(r, maxDocumentSize)
	if errors.Is(err, zip.ErrFormat) || errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, metadata.ErrBadParameter.Withf("not an office document: %v", err)
	} else if err != nil || !ok {
		return nil, err
	}

	// Read the properties, when there are any
	var props coreProperties
	if err := zipdoc.ReadXML(archive, corePropertiesPath, &props); err != nil && !errors.Is(err, metadata.ErrNotFound) {
		return nil, err
	}
	if title.Untitled(props.Title) {
		props.Title = ""
	}

	// Return the properties which are set
	entries := make(map[string]metadata.Metadata)
	for _, p := range []struct{ key, value string }{
		{dc.Title, props.Title},
		{dc.Subject, props.Subject},
		{dc.Creator, props.Creator},
		{dc.Description, props.Description},
	} {
		addEntry(entries, dc.Namespace, p.key, strings.TrimSpace(p.value))
	}
	if created, err := time.Parse(time.RFC3339, strings.TrimSpace(props.Created)); err == nil {
		addEntry(entries, dc.Namespace, dc.Date, created)
	}

	// The thumbnail, when artwork is requested
	if o.HasNamespace(metadata.ArtworkNamespace) {
		if thumbnail := zipdoc.Image(archive, officeThumbnailPath(archive), thumbnailKey, o); thumbnail != nil {
			entries[metadata.ArtworkNamespace+string(metadata.Separator)+thumbnailKey] = thumbnail
		}
	}

	// The content as Markdown, when it's requested
	if err := addMarkdown(entries, archive, o, officeConverter(archive)); err != nil {
		return nil, err
	}
	return slices.Collect(maps.Values(entries)), nil
}

// officeConverter returns the function which converts the document to
// Markdown, by its parts, or nil when its content isn't converted
func officeConverter(archive *zip.Reader) func(*zip.Reader, io.StringWriter) error {
	if _, err := fs.Stat(archive, docxDocumentPath); err == nil {
		return docxMarkdown
	} else if _, err := fs.Stat(archive, pptxPresentationPath); err == nil {
		return pptxMarkdown
	} else if _, err := fs.Stat(archive, xlsxWorkbookPath); err == nil {
		return xlsxMarkdown
	}
	return nil
}

// officeThumbnailPath returns the path of an Office Open XML document's
// thumbnail, which its package's relationships have, relative to the
// package, such as ./docProps/thumbnail.jpeg, or else its usual path. A
// thumbnail which is external to the package isn't read.
func officeThumbnailPath(archive *zip.Reader) string {
	var rels packageRelationships
	if err := zipdoc.ReadXML(archive, packageRelationshipsPath, &rels); err == nil {
		for _, rel := range rels.Relationships {
			if rel.Type == thumbnailRelationship && rel.Target != "" && rel.TargetMode != "External" {
				return path.Clean(strings.TrimPrefix(rel.Target, "/"))
			}
		}
	}
	return thumbnailPath
}

// addEntry adds the metadata to the entries, by its key, when its value
// isn't an empty string
func addEntry(entries map[string]metadata.Metadata, ns, key string, value any) {
	if s, ok := value.(string); ok && s == "" {
		return
	}
	if entry := meta.New(ns, key, value); entry != nil {
		entries[ns+string(metadata.Separator)+key] = entry
	}
}

// officeKind returns the kind of an Office or OpenDocument document of the
// content type, which is a spreadsheet, a presentation, or else a document
func officeKind(contentType string) string {
	switch {
	case strings.Contains(contentType, "spreadsheet"):
		return dc.KindSpreadsheet
	case strings.Contains(contentType, "presentation"):
		return dc.KindPresentation
	default:
		return dc.KindDocument
	}
}
