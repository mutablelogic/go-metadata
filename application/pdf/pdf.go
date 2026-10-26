// Package pdf registers a handler of PDF documents, which returns their
// properties, from their XMP metadata, or their document information
package pdf

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	title "github.com/mutablelogic/go-metadata/internal/title"
	api "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	model "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	types "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// encryptedError is a document which needs a password to be read, and the
// version from its header
type encryptedError struct {
	version string
}

func (e encryptedError) Error() string {
	return "the pdf document needs a password to be read"
}

// wrongPassword returns an error when there's a password, which is wrong
// for the document, or nil when there's no password, as a document which
// needs one isn't an error
func (e encryptedError) wrongPassword(password string) error {
	if password == "" {
		return nil
	}
	return metadata.ErrBadParameter.With("the password is wrong for the pdf document")
}

// metadata returns the properties of the document which can be known
// without its password: that it's encrypted, as pdf:encrypted, and its
// version, as pdf:version
func (e encryptedError) metadata() []metadata.Metadata {
	result := []metadata.Metadata{meta.New(Namespace, "encrypted", true)}
	if e.version != "" {
		result = append(result, meta.New(Namespace, "version", e.version))
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the properties of a PDF document which
	// aren't Dublin Core
	Namespace = "pdf"
)

var (
	reContentType = regexp.MustCompile(`^application/pdf$`)

	// reHeader matches the header of a PDF document, and its version
	reHeader = regexp.MustCompile(`%PDF-([0-9]+\.[0-9]+)`)
)

// maxDocumentSize is the largest document which is read, as it's validated
// in memory. A larger document has only the title from its name.
var maxDocumentSize int64 = 64 << 20

const (
	// maxFontPages is the most of a document's pages which are read for
	// fonts and images, to know whether it has text, or is scanned
	maxFontPages = 20

	// maxFormDepth is the most forms within forms which are read for fonts
	// and images
	maxFormDepth = 2
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	metadata.AddHandler(reContentType, Namespace, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		meta, hints, err := pdfMetadata(ctx, r, o.Password())
		meta = append(meta, metadata.HintMetadata(hints...)...)
		return append(meta, metadata.KindMetadata(dc.KindDocument)...), err
	}, dc.Namespace, Namespace, metadata.MetadataNamespace)

	// The images of the pages, only when the artwork namespace is requested
	metadata.AddHandler(reContentType, Namespace+"-artwork", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		return pdfArtwork(ctx, r, o)
	}, metadata.ArtworkNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// pdfMetadata returns the properties of a PDF document as dc:title,
// dc:creator, dc:description, dc:subject and dc:date, from its XMP metadata,
// or its document information when they aren't in its XMP metadata, and its
// pages, version, producer and creator as pdf:pages, pdf:version,
// pdf:producer and pdf:creator, and the width and height of its first page,
// in millimetres, as pdf:width-mm and pdf:height-mm. A title which is the name of the file the
// document was made from isn't returned, so the title is from the
// document's own name. A document larger than maxDocumentSize has no
// properties. It returns the hints of a document with pages, which are
// artwork, as its first page is drawn, markdown, when a page has text, and
// ocr, when a page is scanned (see pageContent).
func pdfMetadata(ctx context.Context, r io.Reader, password string) ([]metadata.Metadata, []string, error) {
	doc, err := validate(ctx, r, password)
	var encrypted encryptedError
	if errors.As(err, &encrypted) {
		return encrypted.metadata(), nil, encrypted.wrongPassword(password)
	} else if err != nil || doc == nil {
		return nil, nil, err
	}

	// The properties are from the XMP metadata, or the document information
	info := doc.XRefTable
	var xmp model.Description
	if info.CatalogXMPMeta != nil {
		xmp = info.CatalogXMPMeta.RDF.Description
	}
	docTitle := first(first(xmp.Title.Alt.Entries...), info.Title)
	if title.Untitled(docTitle) {
		docTitle = ""
	}
	properties := []struct {
		ns, key, value string
	}{
		{dc.Namespace, dc.Title, docTitle},
		{dc.Namespace, dc.Creator, first(strings.Join(xmp.Author.Seq.Entries, ", "), info.Author)},
		{dc.Namespace, dc.Description, first(first(xmp.Subject.Alt.Entries...), info.Subject)},
		{dc.Namespace, dc.Subject, first(xmp.Keywords, info.Keywords)},
		{Namespace, "version", info.VersionString()},
		{Namespace, "producer", first(xmp.Producer, info.Producer)},
		{Namespace, "creator", first(xmp.Creator, info.Creator)},
	}

	// Return the properties which are set
	entries := make(map[string]metadata.Metadata)
	add := func(ns, key string, value any) {
		if entry := meta.New(ns, key, value); entry != nil {
			entries[ns+string(metadata.Separator)+key] = entry
		}
	}
	for _, p := range properties {
		if value := strings.TrimSpace(p.value); value != "" {
			add(p.ns, p.key, value)
		}
	}
	if created := time.Time(xmp.CreationDate); !created.IsZero() {
		add(dc.Namespace, dc.Date, created)
	} else if created, ok := types.DateTime(info.CreationDate, true); ok {
		add(dc.Namespace, dc.Date, created)
	}
	if doc.PageCount > 0 {
		add(Namespace, "pages", doc.PageCount)
	}
	if width, height, ok := firstPageSize(ctx, info); ok {
		add(Namespace, "width-mm", millimetres(width))
		add(Namespace, "height-mm", millimetres(height))
	}
	if info.Encrypt != nil {
		add(Namespace, "encrypted", true)
	}

	// The hints of a document with pages
	var hints []string
	if doc.PageCount > 0 {
		hints = append(hints, metadata.HintArtwork)
		text, scanned := pageContent(ctx, info)
		if text {
			hints = append(hints, metadata.HintMarkdown)
		}
		if scanned {
			hints = append(hints, metadata.HintOCR)
		}
	}
	return slices.Collect(maps.Values(entries)), hints, nil
}

// pageContent returns whether one of the document's first maxFontPages pages
// has text, as it uses a font, or a form drawn on it does, as text is drawn
// with fonts, and whether one is scanned, as it draws an image, without a
// font. Only the pages' resources are read, rather than their text.
func pageContent(ctx context.Context, x *model.XRefTable) (text, scanned bool) {
	for page := 1; page <= min(x.PageCount, maxFontPages) && !(text && scanned); page++ {
		// The page's own resources, or those it inherits
		dict, _, attrs, err := x.PageDict(ctx, page, false)
		if err != nil || dict == nil {
			continue
		}
		resources, err := x.DereferenceDict(dict["Resources"])
		if (err != nil || resources == nil) && attrs != nil {
			resources = attrs.Resources
		}
		fonts, images := resourceContent(x, resources, 0)
		if fonts {
			text = true
		} else if images {
			scanned = true
		}
	}
	return text, scanned
}

// resourceContent returns whether the resources have a font, and an image,
// or a form in them, to the depth of maxFormDepth forms within forms, has
// one
func resourceContent(x *model.XRefTable, resources types.Dict, depth int) (fonts, images bool) {
	if resources == nil {
		return false, false
	}
	if dict, err := x.DereferenceDict(resources["Font"]); err == nil && len(dict) > 0 {
		fonts = true
	}
	xobjects, err := x.DereferenceDict(resources["XObject"])
	if err != nil {
		return fonts, images
	}
	for _, object := range xobjects {
		stream, _, err := x.DereferenceStreamDict(object)
		if err != nil || stream == nil || stream.Subtype() == nil {
			continue
		}
		switch *stream.Subtype() {
		case "Image":
			images = true
		case "Form":
			if depth >= maxFormDepth {
				continue
			}
			if formResources, err := x.DereferenceDict(stream.Dict["Resources"]); err == nil {
				formFonts, formImages := resourceContent(x, formResources, depth+1)
				fonts, images = fonts || formFonts, images || formImages
			}
		}
	}
	return fonts, images
}

// validate reads and validates the document, which sets its document
// information, and the thumbnails of its pages, or returns nil when it's
// larger than maxDocumentSize. A document which is encrypted is read with
// the password, as its user or owner password, and is an encryptedError
// when it can't be read without the password, or with it. A document which isn't valid, such as one with a
// malformed name in its document information, is read as it is, with the
// document information which can be read from it. The configuration isn't
// read or written in the user's configuration folder.
func validate(ctx context.Context, r io.Reader, password string) (*model.Context, error) {
	rs, ok, err := readSeeker(r)
	if err != nil || !ok {
		return nil, err
	}
	conf := model.NewStatelessConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	conf.UserPW, conf.OwnerPW = password, password
	doc, err := api.ReadContext(ctx, rs, conf)
	if errors.Is(err, pdfcpu.ErrWrongPassword) {
		return nil, encryptedError{version: headerVersion(rs)}
	} else if err != nil {
		return nil, metadata.ErrBadParameter.Withf("not a pdf document: %v", err)
	}
	if err := api.ValidateContext(ctx, doc); err != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		readInfo(doc.XRefTable)
		if err := doc.EnsurePageCount(); err != nil {
			doc.PageCount = 0
		}
	}
	return doc, nil
}

// readInfo sets the document information of a document which wasn't
// validated, from its information dictionary, when it isn't set already.
// Only the entries which are used are read, so a malformed entry which
// isn't used is skipped.
func readInfo(xRefTable *model.XRefTable) {
	if xRefTable.Info == nil {
		return
	}
	d, err := xRefTable.DereferenceDict(*xRefTable.Info)
	if err != nil || d == nil {
		return
	}
	for key, field := range map[string]*string{
		"Title":        &xRefTable.Title,
		"Author":       &xRefTable.Author,
		"Subject":      &xRefTable.Subject,
		"Keywords":     &xRefTable.Keywords,
		"Creator":      &xRefTable.Creator,
		"Producer":     &xRefTable.Producer,
		"CreationDate": &xRefTable.CreationDate,
	} {
		if *field != "" {
			continue
		}
		if obj, found := d.Find(key); found {
			if s, err := xRefTable.DereferenceStringOrHexLiteral(obj, model.V10, nil); err == nil {
				*field = s
			}
		}
	}
}

// firstPageSize returns the width and height of the document's first page,
// in points, a 72nd of an inch, which is its crop box, the area of it which
// is shown, or else its media box, turned by its rotation, or false when it
// doesn't have one
func firstPageSize(ctx context.Context, x *model.XRefTable) (float64, float64, bool) {
	boundaries, err := x.PageBoundaries(ctx, types.IntSet{1: true})
	if err != nil || len(boundaries) == 0 {
		return 0, 0, false
	}
	box := boundaries[0].CropBox()
	if box == nil {
		return 0, 0, false
	}
	width, height := box.Width(), box.Height()
	if boundaries[0].Rot%180 != 0 {
		width, height = height, width
	}
	return width, height, width > 0 && height > 0
}

// millimetres returns a length in points, a 72nd of an inch, in
// millimetres, to a tenth of a millimetre
func millimetres(points float64) float64 {
	return math.Round(points*25.4/72*10) / 10
}

// headerVersion returns the version of the document from its header, such
// as 1.7 for "%PDF-1.7", or an empty string when it can't be read
func headerVersion(rs io.ReadSeeker) string {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	header := make([]byte, 1024)
	n, _ := io.ReadFull(rs, header)
	if m := reHeader.FindSubmatch(header[:n]); m != nil {
		return string(m[1])
	}
	return ""
}

// readSeeker returns the document to read at any offset, which is r itself
// when it can be, as Extract's readers can, and otherwise the document read
// into memory, or false when the document is larger than maxDocumentSize
func readSeeker(r io.Reader) (io.ReadSeeker, bool, error) {
	if rs, ok := r.(io.ReadSeeker); ok {
		size, err := rs.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, false, err
		} else if size > maxDocumentSize {
			return nil, false, nil
		} else if _, err := rs.Seek(0, io.SeekStart); err != nil {
			return nil, false, err
		}
		return rs, true, nil
	}
	data, err := io.ReadAll(io.LimitReader(r, maxDocumentSize+1))
	if err != nil {
		return nil, false, err
	} else if int64(len(data)) > maxDocumentSize {
		return nil, false, nil
	}
	return bytes.NewReader(data), true, nil
}

// first returns the first value which isn't empty
func first(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
