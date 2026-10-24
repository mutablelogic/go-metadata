// Package msoffice registers handlers of the binary Word, Excel and
// PowerPoint documents from before Office 2007, such as .doc, .xls and .ppt
// files, of Visio drawings and Project plans, such as .vsd and .mpp files,
// and of Works documents, such as .wps files, which are compound files,
// which return their properties, and a note that their content can't be
// read yet
package msoffice

import (
	"bytes"
	"context"
	"io"
	"maps"
	"mime"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	title "github.com/mutablelogic/go-metadata/internal/title"
	mscfb "github.com/richardlehane/mscfb"
	msoleps "github.com/richardlehane/msoleps"
	types "github.com/richardlehane/msoleps/types"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the properties of a document which
	// aren't Dublin Core
	Namespace = "msoffice"

	// The streams of a compound file which are its property sets, whose
	// names start with a \x05 character, which the reader removes
	summaryInformation         = "SummaryInformation"
	documentSummaryInformation = "DocumentSummaryInformation"

	// maxPropertySetSize is the largest property set which is read
	maxPropertySetSize = 1 << 20

	// maxDocumentSize is the largest document which is read into memory,
	// when it can't be read in place
	maxDocumentSize = 64 << 20
)

var (
	reContentType = regexp.MustCompile(`^application/(?:msword|vnd\.ms-excel|vnd\.ms-powerpoint|vnd\.visio|vnd\.ms-project|vnd\.ms-works)$`)

	// msofficeKinds are the kinds of documents, by their content type
	msofficeKinds = map[string]string{
		"application/msword":            dc.KindDocument,
		"application/vnd.ms-excel":      dc.KindSpreadsheet,
		"application/vnd.ms-powerpoint": dc.KindPresentation,
		"application/vnd.visio":         dc.KindDrawing,
		"application/vnd.ms-project":    dc.KindDocument,
		"application/vnd.ms-works":      dc.KindDocument,
	}

	// minTime is the earliest time of a property which is set, as a time
	// which isn't set is the start of 1601
	minTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
)

// msofficeTypes are the content types of the documents, by their extension
var msofficeTypes = map[string]string{
	".doc": "application/msword",
	".dot": "application/msword",
	".xls": "application/vnd.ms-excel",
	".xlt": "application/vnd.ms-excel",
	".ppt": "application/vnd.ms-powerpoint",
	".pps": "application/vnd.ms-powerpoint",
	".pot": "application/vnd.ms-powerpoint",
	".vsd": "application/vnd.visio",
	".vss": "application/vnd.visio",
	".vst": "application/vnd.visio",
	".mpp": "application/vnd.ms-project",
	".mpt": "application/vnd.ms-project",
	".wps": "application/vnd.ms-works",
}

// properties maps the names of the properties of a document to their
// namespaces and keys, as Dublin Core where there's an element for them
var properties = map[string]struct{ ns, key string }{
	"Title":                 {dc.Namespace, dc.Title},
	"Subject":               {dc.Namespace, dc.Subject},
	"Author":                {dc.Namespace, dc.Creator},
	"Comments":              {dc.Namespace, dc.Description},
	"CreateTime":            {dc.Namespace, dc.Date},
	"Keywords":              {Namespace, "keywords"},
	"LastAuthor":            {Namespace, "last-author"},
	"LastSaveTime":          {Namespace, "modified"},
	"AppName":               {Namespace, "application"},
	"PageCount":             {Namespace, "pages"},
	"WordCount":             {Namespace, "words"},
	"Slide count":           {Namespace, "slides"},
	"Company":               {Namespace, "company"},
	"Manager":               {Namespace, "manager"},
	"Category":              {Namespace, "category"},
	"Template":              {Namespace, "template"},
	"Paragraph count":       {Namespace, "paragraphs"},
	"Character count":       {Namespace, "characters"},
	"Note count":            {Namespace, "notes"},
	"Hidden slides content": {Namespace, "hidden-slides"},
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	for ext, contentType := range msofficeTypes {
		if err := mime.AddExtensionType(ext, contentType); err != nil {
			panic(err)
		}
	}

	metadata.AddHandler(reContentType, Namespace, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		meta, err := msofficeMetadata(r)
		return append(meta, metadata.KindMetadata(msofficeKinds[o.ContentType()])...), err
	}, dc.Namespace, Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// msofficeMetadata returns the properties of a document, from its summary
// information and document summary information: its title, subject,
// author, comments and when it was created as Dublin Core, and the others,
// such as its keywords, application, and number of pages, words or slides,
// in the msoffice namespace. A title which is the name of the file the
// document was made from isn't returned, so the title is from the
// document's own name. A document without properties has none.
func msofficeMetadata(r io.Reader) ([]metadata.Metadata, error) {
	ra, ok, err := readerAt(r)
	if err != nil || !ok {
		return nil, err
	}
	doc, err := mscfb.New(ra)
	if err != nil {
		return nil, metadata.ErrBadParameter.Withf("not a compound file: %v", err)
	}

	// Read the property sets
	entries := make(map[string]metadata.Metadata)
	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		if name := strings.TrimPrefix(entry.Name, "\x05"); name != summaryInformation && name != documentSummaryInformation {
			continue
		}
		props, err := msoleps.NewFrom(io.LimitReader(entry, maxPropertySetSize))
		if err != nil {
			continue
		}
		for _, prop := range props.Property {
			if p, exists := properties[prop.Name]; exists {
				addProperty(entries, p.ns, p.key, prop.T)
			}
		}
	}
	return slices.Collect(maps.Values(entries)), nil
}

// addProperty adds the value of a property to the entries, as a time, an
// integer, or a string, when it's set, and isn't a title from the name of a
// file
func addProperty(entries map[string]metadata.Metadata, ns, key string, value types.Type) {
	var v any
	switch t := value.(type) {
	case types.FileTime:
		if when := t.Time(); when.After(minTime) {
			v = when.UTC()
		}
	case types.Date:
		if when := t.Time(); when.After(minTime) {
			v = when.UTC()
		}
	default:
		s := strings.TrimSpace(strings.TrimRight(value.String(), "\x00"))
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && value.Type() != "CodeString" && value.Type() != "UnicodeString" {
			if n > 0 {
				v = n
			}
		} else if s != "" && !(key == dc.Title && title.Untitled(s)) {
			v = s
		}
	}
	if v == nil {
		return
	}
	if entry := meta.New(ns, key, v); entry != nil {
		entries[ns+string(metadata.Separator)+key] = entry
	}
}

// readerAt returns the document to read at any offset, which is the reader
// itself when it can be, as Extract's readers can, and otherwise the
// document read into memory, or false when it's larger than
// maxDocumentSize
func readerAt(r io.Reader) (io.ReaderAt, bool, error) {
	if ra, ok := r.(io.ReaderAt); ok {
		return ra, true, nil
	}
	data, err := io.ReadAll(io.LimitReader(r, maxDocumentSize+1))
	if err != nil {
		return nil, false, err
	} else if len(data) > maxDocumentSize {
		return nil, false, nil
	}
	return bytes.NewReader(data), true, nil
}
