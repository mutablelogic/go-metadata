package rtf_test

import (
	"context"
	"maps"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/application/rtf"
)

// extract returns the metadata of the RTF document, by key, with the options
func extract(t *testing.T, data string, opts ...metadata.Option) map[string]string {
	t.Helper()
	opts = append([]metadata.Option{metadata.WithContentType("application/rtf")}, opts...)
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

// document is an RTF document from Word, with its info group, a font table,
// a header, a picture, a field, Unicode characters, and a table
const document = `{\rtf1\ansi\ansicpg1252\uc1\deff0{\fonttbl{\f0\froman\fcharset0 Times New Roman;}}
{\colortbl;\red0\green0\blue0;}{\*\generator Riched20 5.40.11.2210;}
{\info{\title Quarterly \'93Report\'94}{\subject Search}{\author Ada Lovelace}{\operator Charles Babbage}
{\keywords search, audiences}{\doccomm A comment}{\company BBC}
{\creatim\yr2004\mo6\dy24\hr18\min4}{\revtim\yr2005\mo1\dy2\hr9\min30}{\nofpages3}{\nofwords1200}}
{\header \pard Header text\par}
\pard\plain\f0\fs24 The {\b first} paragraph, caf\'e9 and na\u239?ve.\par
{\pict\wmetafile8 0123456789abcdef}\pard Second {\field{\*\fldinst HYPERLINK "http://example.com"}{\fldrslt link}} with a\tab tab\emdash and \{braces\}.\par
\par
\trowd\cellx1000\cellx2000 One\cell Two\cell\row
}`

func Test_rtf_000(t *testing.T) {
	got := extract(t, document)
	want := map[string]string{
		"metadata:kind":   "document",
		"dc:type":         "Text",
		"dc:title":        "Quarterly “Report”",
		"dc:subject":      "Search",
		"dc:creator":      "Ada Lovelace",
		"dc:description":  "A comment",
		"dc:date":         "2004-06-24T18:04:00Z",
		"rtf:keywords":    "search, audiences",
		"rtf:last-author": "Charles Babbage",
		"rtf:modified":    "2005-01-02T09:30:00Z",
		"rtf:application": "Riched20 5.40.11.2210",
		"rtf:pages":       "3",
		"rtf:words":       "1200",
		"rtf:company":     "BBC",
	}
	if !maps.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func Test_rtf_001(t *testing.T) {
	// The text, as Markdown, without the header, picture, or field's
	// instruction, only when the markdown namespace is requested
	got := extract(t, document, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	want := "The first paragraph, café and naïve.\n" +
		"\nSecond link with a tab—and {braces}.\n" +
		"\nOne Two\n"
	if got["markdown:body"] != want {
		t.Errorf("got %q\nwant %q", got["markdown:body"], want)
	}
	if got := extract(t, document); got["markdown:body"] != "" {
		t.Errorf("unexpected Markdown %q", got["markdown:body"])
	}
}

func Test_rtf_002(t *testing.T) {
	// A Mac document, in the Mac Roman code page, from Cocoa, without an
	// info group, and an untitled title isn't the title
	for _, test := range []struct {
		data string
		want map[string]string
	}{
		{`{\rtf1\mac\ansicpg10000\cocoartf102{\fonttbl\f0\froman Times;}\f0 Caf\'8e\par}`, map[string]string{
			"metadata:kind": "document", "dc:type": "Text", "rtf:application": "Cocoa",
		}},
		{`{\rtf1\ansi{\info{\title Microsoft Word - report.doc}{\author A}}Text\par}`, map[string]string{
			"metadata:kind": "document", "dc:type": "Text", "dc:creator": "A",
		}},
		{`not rtf at all`, map[string]string{"metadata:kind": "document", "dc:type": "Text"}},
	} {
		if got := extract(t, test.data); !maps.Equal(got, test.want) {
			t.Errorf("%q: got %v, want %v", test.data, got, test.want)
		}
	}
	got := extract(t, `{\rtf1\mac\ansicpg10000\cocoartf102{\fonttbl\f0\froman Times;}\f0 Caf\'8e\par}`, metadata.WithNamespace(metadata.MarkdownNamespace))
	if want := "Café\n"; got["markdown:body"] != want {
		t.Errorf("got %q, want %q", got["markdown:body"], want)
	}
}

// named is a reader with a name, for its extension
type named struct {
	*strings.Reader
	name string
}

func (r named) Name() string { return r.name }

func Test_rtf_003(t *testing.T) {
	// An RTF document saved as .doc, as WordPad does, is RTF
	contentType, meta, err := metadata.Extract(context.Background(), named{strings.NewReader(document), "report.doc"})
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string, len(meta))
	for _, m := range meta {
		got[m.NS()+":"+m.Key()] = m.Value()
	}
	if contentType != "application/rtf" || got["dc:title"] != "Quarterly “Report”" || got["metadata:kind"] != "document" {
		t.Errorf("got %q, %v", contentType, got)
	}
}
