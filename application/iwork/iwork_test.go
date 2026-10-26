package iwork_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"maps"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/application/iwork"
)

// named is a reader with a name, for its extension
type named struct {
	*bytes.Reader
	name string
}

func (r named) Name() string { return r.name }

// archive returns a zip archive of the entries, by their names
func archive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// jpegImage returns a JPEG image of the width and height
func jpegImage(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// extract returns the content type and metadata of the named document, by
// key, with the options
func extract(t *testing.T, name string, data []byte, opts ...metadata.Option) (string, map[string]string) {
	t.Helper()
	contentType, meta, err := metadata.Extract(context.Background(), named{bytes.NewReader(data), name}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return contentType, result
}

// pages09 is a document from Pages '09
func pages09(t *testing.T) []byte {
	return archive(t, map[string][]byte{
		"index.xml": []byte(`<sl:document xmlns:sl="urn:sl" xmlns:sf="urn:sf" xmlns:sfa="urn:sfa"><sf:metadata>` +
			`<sf:title><sf:string sfa:string="Quarterly Report"/></sf:title>` +
			`<sf:authors><sf:array><sf:string sfa:string="Ada Lovelace"/><sf:string sfa:string="Charles Babbage"/></sf:array></sf:authors>` +
			`<sf:keywords><sf:string sfa:string="finance, quarterly"/></sf:keywords>` +
			`<sf:comment><sf:string sfa:string="Draft"/></sf:comment>` +
			`<sf:copyright><sf:string sfa:string="Analytical Engines"/></sf:copyright>` +
			`<sf:projects><sf:string sfa:string="Engine"/></sf:projects>` +
			`</sf:metadata></sl:document>`),
		"QuickLook/Thumbnail.jpg": jpegImage(t, 64, 48),
	})
}

func Test_iwork_000(t *testing.T) {
	// A document from Pages '09, which is a zip archive, is detected by its
	// extension, and is a document, with its properties, and artwork, as it
	// has a preview
	contentType, got := extract(t, "report.pages", pages09(t))
	want := map[string]string{
		"metadata:kind":  "document",
		"metadata:hint":  "artwork",
		"dc:type":        "Text",
		"dc:title":       "Quarterly Report",
		"dc:creator":     "Ada Lovelace, Charles Babbage",
		"dc:description": "Draft",
		"dc:rights":      "Analytical Engines",
		"iwork:keywords": "finance, quarterly",
		"iwork:projects": "Engine",
	}
	if contentType != "application/vnd.apple.pages" || !maps.Equal(got, want) {
		t.Errorf("got %q, %v\nwant %v", contentType, got, want)
	}
}

func Test_iwork_001(t *testing.T) {
	// A presentation and a spreadsheet from iWork 2013 or later, with their
	// template, when they were made from it, and the version which last
	// saved them, and the title from their names, without hints, as they
	// have no preview
	history := []byte(`<plist version="1.0"><array><string>Template: Blank (2018-02-14 13:01)</string><string>M7.1-5683-2</string></array></plist>`)
	for _, test := range []struct{ name, contentType, kind string }{
		{"launch.key", "application/vnd.apple.keynote", "presentation"},
		{"budget.numbers", "application/vnd.apple.numbers", "spreadsheet"},
	} {
		contentType, got := extract(t, test.name, archive(t, map[string][]byte{
			"Index/Document.iwa":                 []byte("iwa"),
			"Metadata/BuildVersionHistory.plist": history,
		}))
		if contentType != test.contentType || got["metadata:kind"] != test.kind || got["iwork:template"] != "Blank" || got["dc:date"] != "2018-02-14T13:01:00Z" || got["iwork:version"] != "7.1" || got["metadata:hint"] != "" {
			t.Errorf("%s: got %q, %v", test.name, contentType, got)
		}
	}
}

func Test_iwork_002(t *testing.T) {
	// The preview, only when the artwork namespace is requested, and a note
	// that the content can't be read, only when the markdown namespace is
	// requested
	data := pages09(t)
	if _, got := extract(t, "report.pages", data); got["artwork:preview"] != "" || got["markdown:note"] != "" {
		t.Errorf("got %v", got)
	}
	_, meta, err := metadata.Extract(context.Background(), named{bytes.NewReader(data), "report.pages"}, metadata.WithNamespace(metadata.ArtworkNamespace))
	if err != nil {
		t.Fatal(err)
	} else if len(meta) != 1 || meta[0].Key() != "preview" || meta[0].Image() == nil || meta[0].Image().Bounds().Dx() != 64 {
		t.Errorf("got %v", meta)
	}
	if _, got := extract(t, "report.pages", data, metadata.WithNamespace(metadata.MarkdownNamespace)); got["markdown:note"] == "" {
		t.Errorf("got %v", got)
	}
}

func Test_iwork_003(t *testing.T) {
	// A zip archive with an iWork extension which isn't a document is a bad
	// parameter, and a zip archive with another extension isn't one
	data := archive(t, map[string][]byte{"readme.txt": []byte("hi")})
	_, _, err := metadata.Extract(context.Background(), named{bytes.NewReader(data), "notes.pages"}, metadata.WithNamespace("dc"))
	if !errors.Is(err, metadata.ErrBadParameter) {
		t.Errorf("expected ErrBadParameter, got %v", err)
	}
	if contentType, _, err := metadata.ContentType(named{bytes.NewReader(data), "notes.zip"}); err != nil || contentType != "application/zip" {
		t.Errorf("got %q, %v", contentType, err)
	}
}
