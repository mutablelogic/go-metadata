package html_test

import (
	"context"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/text/html"
)

// extract returns the metadata of the document, by key
func extract(t *testing.T, doc string, opts ...metadata.Option) map[string]string {
	t.Helper()
	opts = append(opts, metadata.WithContentType("text/html"))
	_, meta, err := metadata.Extract(context.Background(), strings.NewReader(doc), opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return result
}

// expect fails when the metadata doesn't have the keys with the values, or
// has a key which is expected to be missing, with an empty value
func expect(t *testing.T, got map[string]string, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if value == "" {
			if v, exists := got[key]; exists {
				t.Errorf("%s: expected no value, got %q", key, v)
			}
		} else if got[key] != value {
			t.Errorf("%s: expected %q, got %q", key, value, got[key])
		}
	}
}

func Test_html_000(t *testing.T) {
	// The document is decoded from the character set of its meta element
	doc := "<html><head><meta charset=\"windows-1252\"><title>Caf\xe9 \x93menu\x94</title></head></html>"
	expect(t, extract(t, doc), map[string]string{
		"dc:title": "Café “menu”",
	})
}

func Test_html_001(t *testing.T) {
	// A document with no character set is UTF-8, when it's valid UTF-8, and
	// Windows-1252 otherwise
	expect(t, extract(t, "<title>Café</title>"), map[string]string{
		"dc:title": "Café",
	})
	expect(t, extract(t, "<title>Caf\xe9</title>"), map[string]string{
		"dc:title": "Café",
	})
}

func Test_html_002(t *testing.T) {
	// Meta elements which are how the document is displayed, served, or
	// verified, and microdata properties, aren't metadata
	doc := `<html><head>
		<meta name="referrer" content="origin">
		<meta http-equiv="X-UA-Compatible" content="IE=edge">
		<meta name="format-detection" content="telephone=no">
		<meta name="apple-mobile-web-app-capable" content="yes">
		<meta name="apple-mobile-web-app-status-bar-style" content="black">
		<meta name="msapplication-TileColor" content="#fff">
		<meta name="google-site-verification" content="abc">
		<meta itemprop="name" content="Site">
		<meta name="generator" content="Hugo">
		<meta http-equiv="refresh" content="0; URL=https://example.com/">
	</head></html>`
	expect(t, extract(t, doc), map[string]string{
		"html:referrer":                              "",
		"html:x-ua-compatible":                       "",
		"html:format-detection":                      "",
		"html:apple-mobile-web-app-capable":          "",
		"html:apple-mobile-web-app-status-bar-style": "",
		"html:msapplication-tilecolor":               "",
		"html:google-site-verification":              "",
		"html:name":                                  "",
		"html:generator":                             "Hugo",
		"html:refresh":                               "0; URL=https://example.com/",
	})
}

func Test_html_003(t *testing.T) {
	// The language of the html element, and the canonical link, are preferred
	// to the meta elements for the language and identifier, and the date is
	// the date the document was published
	doc := `<html lang="en-GB"><head>
		<link rel="canonical" href="https://example.com/page">
		<meta property="og:locale" content="en_US">
		<meta property="og:url" content="https://example.com/page?ref=og">
		<meta property="article:published_time" content="2015-07-04T18:38:12+00:00">
		<meta property="article:modified_time" content="2016-01-01T00:00:00+00:00">
	</head></html>`
	expect(t, extract(t, doc), map[string]string{
		"dc:language":                 "en-GB",
		"dc:identifier":               "https://example.com/page",
		"dc:date":                     "2015-07-04T18:38:12+00:00",
		"html:og-locale":              "",
		"html:og-url":                 "",
		"html:article-published_time": "",
		"html:article-modified_time":  "2016-01-01T00:00:00+00:00",
	})
}

func Test_html_004(t *testing.T) {
	// The meta elements are the language and identifier, with a locale as a
	// language, when there is no language on the html element or canonical
	// link, and the date is when the document was created, when there is no
	// published date
	doc := `<html><head>
		<meta property="og:locale" content="en_US">
		<meta property="og:url" content="https://example.com/page">
		<meta name="DateCreated" content="2016-05-17T15:59:54Z">
	</head></html>`
	expect(t, extract(t, doc), map[string]string{
		"dc:language":   "en-US",
		"dc:identifier": "https://example.com/page",
		"dc:date":       "2016-05-17T15:59:54Z",
	})
}

func Test_html_005(t *testing.T) {
	// The title element is preferred to the title for social media, and the
	// description to the one for social media
	doc := `<html><head>
		<title>Page</title>
		<meta property="og:title" content="Social">
		<meta property="og:description" content="Social description">
		<meta name="description" content="Description">
		<meta property="og:image" content="https://example.com/a.png">
	</head></html>`
	expect(t, extract(t, doc), map[string]string{
		"dc:title":       "Page",
		"dc:description": "Description",
		"html:og-image":  "https://example.com/a.png",
	})
}

func Test_html_006(t *testing.T) {
	// The content is Markdown, only when the markdown namespace is
	// requested, decoded from the document's character set
	doc := "<html><head><meta charset=\"windows-1252\"><title>Page</title><script>var x = 1;</script></head>" +
		"<body><h1>Caf\xe9</h1><p>Some <b>bold</b> text, and a <a href=\"https://example.com/\">link</a>.</p>" +
		"<ul><li>one</li><li>two</li></ul></body></html>"
	want := "# Café\n\nSome **bold** text, and a [link](https://example.com/).\n\n- one\n- two"
	expect(t, extract(t, doc, metadata.WithNamespace(metadata.MarkdownNamespace)), map[string]string{
		"markdown:body": want,
		"dc:title":      "",
	})
	expect(t, extract(t, doc), map[string]string{
		"markdown:body": "",
		"dc:title":      "Page",
	})

	// A range of the Markdown is a part of it, with the offset of the next
	// part
	expect(t, extract(t, doc, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange("2:7")), map[string]string{
		"markdown:body":   "Café",
		"markdown:offset": "2",
		"markdown:next":   "7",
	})
}

func Test_html_007(t *testing.T) {
	// An empty document has no metadata, rather than an error
	expect(t, extract(t, ""), map[string]string{
		"dc:title": "",
	})
}
