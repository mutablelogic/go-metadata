// Package epub registers handlers of EPUB books, which return their
// metadata, their cover, and their text as Markdown
package epub

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"maps"
	"mime"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	isbns "github.com/mutablelogic/go-metadata/internal/isbn"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	title "github.com/mutablelogic/go-metadata/internal/title"
	zipdoc "github.com/mutablelogic/go-metadata/internal/zipdoc"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// epubContainer is an EPUB's META-INF/container.xml, which has the path of
// its package document
type epubContainer struct {
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

// epubPackage is an EPUB's package document, which has its metadata, in
// Dublin Core, and its files, in its manifest
type epubPackage struct {
	Metadata struct {
		Titles       []epubElement `xml:"http://purl.org/dc/elements/1.1/ title"`
		Creators     []epubElement `xml:"http://purl.org/dc/elements/1.1/ creator"`
		Subjects     []epubElement `xml:"http://purl.org/dc/elements/1.1/ subject"`
		Descriptions []epubElement `xml:"http://purl.org/dc/elements/1.1/ description"`
		Publishers   []epubElement `xml:"http://purl.org/dc/elements/1.1/ publisher"`
		Languages    []epubElement `xml:"http://purl.org/dc/elements/1.1/ language"`
		Dates        []epubElement `xml:"http://purl.org/dc/elements/1.1/ date"`
		Identifiers  []epubElement `xml:"http://purl.org/dc/elements/1.1/ identifier"`
		Metas        []struct {
			Name     string `xml:"name,attr"`
			Content  string `xml:"content,attr"`
			Property string `xml:"property,attr"`
			Refines  string `xml:"refines,attr"`
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef  string `xml:"idref,attr"`
		Linear string `xml:"linear,attr"`
	} `xml:"spine>itemref"`
}

// epubElement is a Dublin Core element of an EPUB's metadata, with its
// identifier, which EPUB 3 refines, and the role, scheme and event which
// EPUB 2 has as attributes
type epubElement struct {
	ID     string `xml:"id,attr"`
	Role   string `xml:"http://www.idpf.org/2007/opf role,attr"`
	Scheme string `xml:"http://www.idpf.org/2007/opf scheme,attr"`
	Event  string `xml:"http://www.idpf.org/2007/opf event,attr"`
	Value  string `xml:",chardata"`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// epubContainerPath is the path of an EPUB's container, which has the
	// path of its package document
	epubContainerPath = "META-INF/container.xml"

	// epubType is the type of an EPUB book, and epubPackageType of its
	// package document
	epubType        = "application/epub+zip"
	epubPackageType = "application/oebps-package+xml"

	// coverKey is the key of the cover of a book, as artwork
	coverKey = "cover"
)

// maxBookSize is the largest book which is read into memory, when it can't
// be read in place. A larger book has only the title from its name.
var maxBookSize int64 = 64 << 20

// epubDates are the layouts of an EPUB's dates, which are W3C dates, from a
// year to a time
var epubDates = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02", "2006-01", "2006"}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// EPUB books are zip archives, which are refined by their extension
	if err := mime.AddExtensionType(".epub", epubType); err != nil {
		panic(err)
	}

	// EPUB books, whose cover is returned only when artwork is requested, and
	// text only when Markdown is
	metadata.AddHandler(regexp.MustCompile(`^`+regexp.QuoteMeta(epubType)+`$`), "epub", func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		meta, err := epubMetadata(ctx, r, o)
		return append(meta, metadata.KindMetadata(dc.KindBook)...), err
	}, dc.Namespace, metadata.ArtworkNamespace, metadata.MarkdownNamespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// epubMetadata returns the metadata of an EPUB book: its title, authors,
// subjects, description, publisher, language, date and ISBN, as dc:title,
// dc:creator, dc:subject, dc:description, dc:publisher, dc:language, dc:date
// and dc:identifier, when they're set, its cover as artwork:cover, when
// artwork is requested, its text as Markdown, when it's requested, a part
// at a time (see metadata.WithRange), and the hints of a book with text, and
// with a cover. A title which isn't a title,
// such as the name of a file, isn't returned, so the title is from the
// book's own name.
func epubMetadata(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	// A book is a zip archive, which is read in place, or from memory,
	// unless it's too large, when it has only the title from its name
	archive, ok, err := zipdoc.Open(r, maxBookSize)
	if errors.Is(err, zip.ErrFormat) || errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, metadata.ErrBadParameter.Withf("not an epub book: %v", err)
	} else if err != nil || !ok {
		return nil, err
	}

	// The package document, from the container
	var container epubContainer
	if err := zipdoc.ReadXML(archive, epubContainerPath, &container); err != nil {
		return nil, metadata.ErrBadParameter.Withf("not an epub book: %v", err)
	}
	// The package document is the first rootfile which is one, which is the
	// default rendition of a book with several, or else the first rootfile
	packagePath := ""
	for _, rootfile := range container.Rootfiles {
		if rootfile.MediaType == epubPackageType {
			packagePath = rootfile.FullPath
			break
		} else if packagePath == "" {
			packagePath = rootfile.FullPath
		}
	}
	if packagePath == "" {
		return nil, metadata.ErrBadParameter.Withf("not an epub book: %s has no package document", epubContainerPath)
	}
	var pkg epubPackage
	if err := zipdoc.ReadXML(archive, packagePath, &pkg); err != nil {
		return nil, metadata.ErrBadParameter.Withf("not an epub book: %v", err)
	}

	// The metadata which is set
	md := pkg.Metadata
	bookTitle := firstValue(md.Titles)
	if title.Untitled(bookTitle) {
		bookTitle = ""
	}
	entries := make(map[string]metadata.Metadata)
	add := func(key string, value any) {
		if entry := meta.New(dc.Namespace, key, value); entry != nil {
			entries[dc.Namespace+string(metadata.Separator)+key] = entry
		}
	}
	for _, p := range []struct{ key, value string }{
		{dc.Title, bookTitle},
		{dc.Creator, strings.Join(pkg.authors(), ", ")},
		{dc.Subject, strings.Join(values(md.Subjects), ", ")},
		{dc.Description, itext.PlainText(firstValue(md.Descriptions))},
		{dc.Publisher, firstValue(md.Publishers)},
		{dc.Language, firstValue(md.Languages)},
		{dc.Identifier, isbn(md.Identifiers)},
	} {
		if value := strings.TrimSpace(p.value); value != "" {
			add(p.key, value)
		}
	}
	if date, ok := publicationDate(md.Dates); ok {
		add(dc.Date, date)
	}

	// The hints of a book with text in its spine, and with a cover in its
	// archive
	var hints []string
	if len(pkg.Spine) > 0 {
		hints = append(hints, metadata.HintMarkdown)
	}
	if coverPath := pkg.coverPath(path.Dir(packagePath)); coverPath != "" {
		if _, err := fs.Stat(archive, coverPath); err == nil {
			hints = append(hints, metadata.HintArtwork)
		}
	}

	// The cover, when artwork is requested
	if o.HasNamespace(metadata.ArtworkNamespace) {
		if cover := pkg.cover(archive, path.Dir(packagePath), o); cover != nil {
			entries[metadata.ArtworkNamespace+string(metadata.Separator)+coverKey] = cover
		}
	}

	// The text, when Markdown is requested
	if o.HasNamespace(metadata.MarkdownNamespace) {
		w := imarkdown.NewWriter(o)
		parts, err := w.Metadata(pkg.markdown(ctx, archive, path.Dir(packagePath), w))
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			entries[part.NS()+string(metadata.Separator)+part.Key()] = part
		}
	}
	return append(slices.Collect(maps.Values(entries)), metadata.HintMetadata(hints...)...), nil
}

// authors returns the names of the book's authors: its creators which have
// the role of author, or no role, which is an attribute in EPUB 2, or
// refines the creator in EPUB 3, which can refine it with several roles.
// Creators with only other roles, such as an illustrator or editor, aren't
// its authors, unless all are, such as an anthology's editors, by whom it's
// known.
func (pkg *epubPackage) authors() []string {
	roles := make(map[string][]string)
	for _, m := range pkg.Metadata.Metas {
		if id, found := strings.CutPrefix(m.Refines, "#"); found && m.Property == "role" {
			roles[id] = append(roles[id], strings.TrimSpace(m.Value))
		}
	}
	var authors, all []string
	for _, creator := range pkg.Metadata.Creators {
		name := strings.TrimSpace(creator.Value)
		if name == "" || slices.Contains(all, name) {
			continue
		}
		all = append(all, name)
		creatorRoles := roles[creator.ID]
		if creator.Role != "" {
			creatorRoles = []string{creator.Role}
		}
		if len(creatorRoles) == 0 || slices.ContainsFunc(creatorRoles, func(role string) bool { return strings.EqualFold(role, "aut") }) {
			authors = append(authors, name)
		}
	}
	if len(authors) == 0 {
		return all
	}
	return authors
}

// cover returns the book's cover as artwork:cover, downscaled to fit the
// largest image dimension, or nil when it has none, or it can't be read (see
// coverPath)
func (pkg *epubPackage) cover(archive *zip.Reader, dir string, o *metadata.Opts) metadata.Metadata {
	if coverPath := pkg.coverPath(dir); coverPath != "" {
		return zipdoc.Image(archive, coverPath, coverKey, o)
	}
	return nil
}

// coverPath returns the path of the book's cover in its archive, or an
// empty string when it has none: the item of its manifest which is its
// cover image, in EPUB 3, or which its cover meta names, in EPUB 2, or else
// the image whose identifier or path is a cover. Its href is relative to
// the package document's directory.
func (pkg *epubPackage) coverPath(dir string) string {
	href := ""
	for _, item := range pkg.Manifest {
		if slices.Contains(strings.Fields(item.Properties), "cover-image") {
			href = item.Href
			break
		}
	}
	if href == "" {
		for _, m := range pkg.Metadata.Metas {
			if m.Name != "cover" || m.Content == "" {
				continue
			}
			for _, item := range pkg.Manifest {
				if item.ID == m.Content || item.Href == m.Content {
					href = item.Href
				}
			}
		}
	}
	if href == "" {
		for _, item := range pkg.Manifest {
			if strings.HasPrefix(item.MediaType, "image/") && (strings.Contains(strings.ToLower(item.ID), "cover") || strings.Contains(strings.ToLower(item.Href), "cover")) {
				href = item.Href
				break
			}
		}
	}
	if href == "" {
		return ""
	}

	// The cover's path is relative to the package document
	if unescaped, err := url.PathUnescape(href); err == nil {
		href = unescaped
	}
	return path.Join(dir, href)
}

// firstValue returns the first of the elements which has a value
func firstValue(elements []epubElement) string {
	for _, element := range elements {
		if value := strings.TrimSpace(element.Value); value != "" {
			return value
		}
	}
	return ""
}

// values returns the elements' values, without those which are empty, or
// repeated
func values(elements []epubElement) []string {
	var result []string
	for _, element := range elements {
		if value := strings.TrimSpace(element.Value); value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

// publicationDate returns the date the book was published, which is the
// date whose event is its publication, in EPUB 2, or else its first date
func publicationDate(dates []epubElement) (time.Time, bool) {
	ordered := slices.Clone(dates)
	slices.SortStableFunc(ordered, func(a, b epubElement) int {
		return publication(b) - publication(a)
	})
	for _, date := range ordered {
		value := strings.TrimSpace(date.Value)
		for _, layout := range epubDates {
			if t, err := time.Parse(layout, value); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// publication returns 1 when the date is the book's publication, and 0
// otherwise
func publication(date epubElement) int {
	switch strings.ToLower(date.Event) {
	case "publication", "original-publication":
		return 1
	default:
		return 0
	}
}

// isbn returns the book's ISBN, without urn:isbn: or ISBN before it, or an
// empty string when it has none, as other identifiers, such as a UUID or a
// shop's, don't identify the book elsewhere. An ISBN whose check digit is
// wrong isn't one.
func isbn(identifiers []epubElement) string {
	for _, identifier := range identifiers {
		if value, ok := isbns.Parse(identifier.Value); ok {
			return value
		}
	}
	return ""
}
