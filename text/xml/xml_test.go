package xml_test

import (
	"context"
	"maps"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/text/xml"
	charmap "golang.org/x/text/encoding/charmap"
)

// extract returns the metadata of the XML document, of the content type, by
// key, with the options
func extract(t *testing.T, contentType, data string, opts ...metadata.Option) map[string]string {
	t.Helper()
	opts = append([]metadata.Option{metadata.WithContentType(contentType)}, opts...)
	_, meta, err := metadata.Extract(context.Background(), strings.NewReader(data), opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+":"+m.Key()] = m.Value()
	}
	return result
}

// data is the kind of an XML document
var data = map[string]string{"metadata:kind": "data", "dc:type": "Dataset"}

// with returns the kind of an XML document, with the metadata
func with(values map[string]string) map[string]string {
	result := maps.Clone(data)
	maps.Copy(result, values)
	return result
}

func Test_xml_000(t *testing.T) {
	latin1, err := charmap.ISO8859_1.NewEncoder().String(`<?xml version="1.0" encoding="ISO-8859-1"?><doc><title>Café</title></doc>`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, contentType, data string
		want                    map[string]string
	}{
		{"RSS", "application/rss+xml", `<?xml version="1.0"?><rss version="2.0"><channel><title> News
			Feed </title><item><title>Item</title></item></channel></rss>`,
			with(map[string]string{"xml:root": "rss", "dc:title": "News Feed"})},
		{"Atom", "application/atom+xml", `<feed xmlns="http://www.w3.org/2005/Atom"><title>Atom Feed</title></feed>`,
			with(map[string]string{"xml:root": "feed", "xml:namespace": "http://www.w3.org/2005/Atom", "dc:title": "Atom Feed"})},
		{"Plist", "text/xml", `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Title</key><string>Not a title</string></dict></plist>`,
			with(map[string]string{"xml:root": "plist", "xml:doctype": "-//Apple Computer//DTD PLIST 1.0//EN"})},
		{"Doctype", "text/xml", `<!DOCTYPE note [<!ELEMENT note (#PCDATA)>]><note>Hi</note>`,
			with(map[string]string{"xml:root": "note", "xml:doctype": "note"})},

		// A title in Dublin Core's namespace, within a header
		{"DublinCore", "application/xml", `<document xmlns:dc="http://purl.org/dc/elements/1.1/"><header><dc:title>Header Title</dc:title></header></document>`,
			with(map[string]string{"xml:root": "document", "dc:title": "Header Title"})},

		// A title which is too deep, or empty, isn't the title
		{"Deep", "application/xml", `<a><b><c><title>Too Deep</title></c></b></a>`,
			with(map[string]string{"xml:root": "a"})},
		{"Empty", "application/xml", `<a><title> </title><b><title>Second</title></b></a>`,
			with(map[string]string{"xml:root": "a", "dc:title": "Second"})},

		// A document in another character set, which it declares
		{"Latin1", "text/xml", latin1, with(map[string]string{"xml:root": "doc", "dc:title": "Café"})},

		// A document which isn't well-formed is read up to its error
		{"Broken", "application/xml", `<a><title>Broken</title><b></a>`, with(map[string]string{"xml:root": "a", "dc:title": "Broken"})},
		{"NotXML", "application/xml", `not xml at all`, data},

		// XHTML is read by the handler of HTML
		{"XHTML", "application/xhtml+xml", `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Page</title></head></html>`, map[string]string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := extract(t, test.contentType, test.data); !maps.Equal(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func Test_xml_001(t *testing.T) {
	// The document as an XML code block, only when the markdown namespace is
	// requested
	const doc = "<a>```</a>"
	got := extract(t, "text/xml", doc, metadata.WithNamespace(metadata.MarkdownNamespace))
	if want := "````xml\n<a>```</a>\n````\n"; got["markdown:body"] != want {
		t.Errorf("got %q, want %q", got["markdown:body"], want)
	}
	if got := extract(t, "text/xml", doc); got["markdown:body"] != "" {
		t.Errorf("unexpected Markdown %q", got["markdown:body"])
	}
}
