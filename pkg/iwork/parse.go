package iwork

import (
	"archive/zip"
	"compress/gzip"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// maxEntry is the most of an entry of a document which is read, such as
	// its index.xml, which can be large, or its preview
	maxEntry = 32 << 20

	// The preview of a document from iWork '09, and the build version
	// history of a document from iWork 2013 or later
	thumbnail09    = "QuickLook/Thumbnail.jpg"
	versionHistory = "Metadata/BuildVersionHistory.plist"

	// preview is the preview of a document from iWork 2013 or later
	preview = "preview.jpg"
)

var (
	// ErrNotDocument is returned for a zip archive which isn't an iWork
	// document
	ErrNotDocument = errors.New("not an iWork document")

	// indexes09 are the entries of a document from iWork '09 which are its
	// content and properties: index.xml for Pages and Numbers, and
	// index.apxl for Keynote, which can be compressed
	indexes09 = []string{"index.xml", "index.xml.gz", "index.apxl", "index.apxl.gz"}

	// reTemplate matches the template of a document, and when the document
	// was made from it, such as Template: Blank (2018-02-14 13:01)
	reTemplate = regexp.MustCompile(`^Template:\s*(.*?)\s*\((\d{4}-\d{2}-\d{2} \d{2}:\d{2})\)\s*$`)

	// reVersion matches a build of an application which saved a document,
	// and its version, such as M7.1-5683-2
	reVersion = regexp.MustCompile(`^M(\d+(?:\.\d+)*)-`)
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Parse returns what's known of the iWork document of the size: the
// properties of a document from iWork '09, from its index.xml, which can be
// compressed, or the template and version of a document from iWork 2013 or
// later, from its build version history, and its preview, or ErrNotDocument
// when it isn't an iWork document
func Parse(r io.ReaderAt, size int64) (*Document, error) {
	z, err := zip.NewReader(r, size)
	if err != nil {
		return nil, ErrNotDocument
	}
	entries := make(map[string]*zip.File, len(z.File))
	for _, f := range z.File {
		entries[f.Name] = f
	}

	doc := new(Document)
	switch {
	case index09(entries) != nil:
		if err := doc.read09(index09(entries)); err != nil {
			return nil, err
		}
		doc.Preview = read(entries[thumbnail09])
	case entries[versionHistory] != nil || entries[preview] != nil || hasIndex(z.File):
		doc.readVersionHistory(entries[versionHistory])
		doc.Preview = read(entries[preview])
	default:
		return nil, ErrNotDocument
	}
	return doc, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// hasIndex returns true when the archive has the index of a document from
// iWork 2013 or later, which is its Index folder, or an Index.zip archive
func hasIndex(files []*zip.File) bool {
	for _, f := range files {
		if f.Name == "Index.zip" || strings.HasPrefix(f.Name, "Index/") {
			return true
		}
	}
	return false
}

// read returns the content of an entry, or nil when it's nil, larger than
// maxEntry, or can't be read
func read(f *zip.File) []byte {
	if f == nil || f.UncompressedSize64 > maxEntry {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxEntry))
	if err != nil {
		return nil
	}
	return data
}

// index09 returns the index of a document from iWork '09, or nil when it
// isn't one
func index09(entries map[string]*zip.File) *zip.File {
	for _, name := range indexes09 {
		if f := entries[name]; f != nil {
			return f
		}
	}
	return nil
}

// read09 reads the properties of a document from iWork '09, from the
// metadata element of its index, which is compressed when its name ends
// with .gz
func (doc *Document) read09(f *zip.File) error {
	compressed := strings.HasSuffix(f.Name, ".gz")
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	var r io.Reader = io.LimitReader(rc, maxEntry)
	if compressed {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return ErrNotDocument
		}
		defer gz.Close()
		r = io.LimitReader(gz, maxEntry)
	}
	return doc.readMetadata(r)
}

// readMetadata reads the properties of a document from the metadata element
// of its index.xml, whose properties are each one string, or an array of
// them, such as <sf:title><sf:string sfa:string="Report"/></sf:title>. The
// rest of the index isn't read.
func (doc *Document) readMetadata(r io.Reader) error {
	d := xml.NewDecoder(r)
	d.Strict = false
	inMetadata, property := false, ""
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			// The properties which were read, before the index can't be
			// read
			return nil
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Local == "metadata":
				inMetadata = true
			case !inMetadata:
			case t.Name.Local == "string" && property != "":
				doc.property(property, attribute(t, "string"))
			case t.Name.Local != "array":
				property = t.Name.Local
			}
		case xml.EndElement:
			switch {
			case t.Name.Local == "metadata" && inMetadata:
				return nil
			case t.Name.Local == property:
				property = ""
			}
		}
	}
}

// property sets a property of the document, from one of its strings, which
// isn't set when it's empty
func (doc *Document) property(name, value string) {
	if value = strings.TrimSpace(value); value == "" {
		return
	}
	switch name {
	case "title":
		doc.Title = value
	case "comment":
		doc.Comment = value
	case "copyright":
		doc.Copyright = value
	case "authors":
		doc.Authors = append(doc.Authors, value)
	case "keywords":
		doc.Keywords = append(doc.Keywords, splitKeywords(value)...)
	case "projects":
		doc.Projects = append(doc.Projects, value)
	}
}

// readVersionHistory reads the template a document from iWork 2013 or later
// was made from, and when, and the version of the application which last
// saved it, from its build version history, which is a property list of an
// array of strings, such as Template: Blank (2018-02-14 13:01), and then
// the builds which saved it, such as M7.1-5683-2
func (doc *Document) readVersionHistory(f *zip.File) {
	data := read(f)
	if data == nil {
		return
	}
	var plist struct {
		Strings []string `xml:"array>string"`
	}
	if err := xml.Unmarshal(data, &plist); err != nil {
		return
	}
	for _, s := range plist.Strings {
		s = strings.TrimSpace(s)
		if m := reTemplate.FindStringSubmatch(s); m != nil {
			doc.Template = m[1]
			if created, err := time.Parse("2006-01-02 15:04", m[2]); err == nil {
				doc.Created = created
			}
		} else if m := reVersion.FindStringSubmatch(s); m != nil {
			doc.Version = m[1]
		}
	}
}

// attribute returns the value of an element's attribute, by its name, in
// any namespace
func attribute(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// splitKeywords returns the keywords of a string, which are separated by
// commas, or the string itself
func splitKeywords(value string) []string {
	var result []string
	for _, keyword := range strings.Split(value, ",") {
		if keyword = strings.TrimSpace(keyword); keyword != "" {
			result = append(result, keyword)
		}
	}
	return result
}
