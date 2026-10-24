package iwork_test

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"strings"
	"testing"
	"time"

	// Packages
	iwork "github.com/mutablelogic/go-metadata/pkg/iwork"
)

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

// gzipped returns the data compressed
func gzipped(t *testing.T, data string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write([]byte(data))
	w.Close()
	return buf.Bytes()
}

// parse returns the document, or fails the test
func parse(t *testing.T, data []byte) *iwork.Document {
	t.Helper()
	doc, err := iwork.Parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// index09 is the index of a document from Pages '09, with its properties,
// one of which is an array, and keywords which are separated by commas
const index09 = `<?xml version="1.0"?>
<sl:document xmlns:sl="http://developer.apple.com/namespaces/sl" xmlns:sf="http://developer.apple.com/namespaces/sf" xmlns:sfa="http://developer.apple.com/namespaces/sfa">
<sf:metadata>
<sf:title><sf:string sfa:string="Quarterly Report"/></sf:title>
<sf:keywords><sf:string sfa:string="finance, quarterly"/></sf:keywords>
<sf:authors><sf:array><sf:string sfa:string="Ada Lovelace"/><sf:string sfa:string="Charles Babbage"/></sf:array></sf:authors>
<sf:comment><sf:string sfa:string=""/></sf:comment>
<sf:copyright><sf:string sfa:string="© 2009 Analytical Engines"/></sf:copyright>
</sf:metadata>
<sf:text-body><sf:title><sf:string sfa:string="Not the title"/></sf:title></sf:text-body>
</sl:document>`

func Test_iwork_000(t *testing.T) {
	// A document from Pages '09, whose properties are in its index.xml, and
	// whose preview is its QuickLook thumbnail
	doc := parse(t, archive(t, map[string][]byte{
		"index.xml":               []byte(index09),
		"QuickLook/Thumbnail.jpg": []byte("thumbnail"),
	}))
	if doc.Title != "Quarterly Report" || strings.Join(doc.Authors, ";") != "Ada Lovelace;Charles Babbage" || strings.Join(doc.Keywords, ";") != "finance;quarterly" || doc.Comment != "" || doc.Copyright != "© 2009 Analytical Engines" || string(doc.Preview) != "thumbnail" {
		t.Errorf("got %+v", doc)
	}
}

func Test_iwork_001(t *testing.T) {
	// A document from Keynote '09, whose index is compressed index.apxl,
	// with its own prefix
	doc := parse(t, archive(t, map[string][]byte{
		"index.apxl.gz": gzipped(t, `<key:presentation xmlns:key="urn:key" xmlns:sf="urn:sf" xmlns:sfa="urn:sfa"><key:metadata><key:title><sf:string sfa:string="Launch"/></key:title><key:authors><sf:string sfa:string="Grace Hopper"/></key:authors></key:metadata></key:presentation>`),
	}))
	if doc.Title != "Launch" || strings.Join(doc.Authors, ";") != "Grace Hopper" || doc.Preview != nil {
		t.Errorf("got %+v", doc)
	}
}

func Test_iwork_002(t *testing.T) {
	// A document from iWork 2013 or later, whose template, when it was made
	// from it, and the version which last saved it, are in its build version
	// history, and whose preview is preview.jpg
	history := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><array>
<string>Template: Blank (2018-02-14 13:01)</string>
<string>M6.2-1234-1</string>
<string>M7.1-5683-2</string>
</array></plist>`
	doc := parse(t, archive(t, map[string][]byte{
		"Index/Document.iwa":                 []byte("iwa"),
		"Metadata/BuildVersionHistory.plist": []byte(history),
		"preview.jpg":                        []byte("preview"),
		"preview-micro.jpg":                  []byte("micro"),
	}))
	if doc.Template != "Blank" || !doc.Created.Equal(time.Date(2018, 2, 14, 13, 1, 0, 0, time.UTC)) || doc.Version != "7.1" || string(doc.Preview) != "preview" || doc.Title != "" {
		t.Errorf("got %+v", doc)
	}
}

func Test_iwork_003(t *testing.T) {
	// A zip archive which isn't an iWork document, and data which isn't a
	// zip archive, aren't one
	for _, data := range [][]byte{archive(t, map[string][]byte{"readme.txt": []byte("hi")}), []byte("not a zip archive")} {
		if _, err := iwork.Parse(bytes.NewReader(data), int64(len(data))); !errors.Is(err, iwork.ErrNotDocument) {
			t.Errorf("expected ErrNotDocument, got %v", err)
		}
	}
}
