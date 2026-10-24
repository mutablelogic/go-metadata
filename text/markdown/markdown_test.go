package markdown_test

import (
	"context"
	"io"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/text/markdown"
)

// named is a reader with a name
type named struct {
	io.Reader
	name string
}

func (r named) Name() string { return r.name }

// extract returns the metadata of the Markdown document, named doc.md, by
// key, and any error
func extract(t *testing.T, doc string, opts ...metadata.Option) (map[string]metadata.Metadata, error) {
	t.Helper()
	opts = append(opts, metadata.WithContentType("text/markdown"))
	_, meta, err := metadata.Extract(context.Background(), named{strings.NewReader(doc), "doc.md"}, opts...)
	result := make(map[string]metadata.Metadata, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m
	}
	return result, err
}

// values returns the values of the metadata of the document, by key
func values(t *testing.T, doc string, opts ...metadata.Option) map[string]string {
	t.Helper()
	meta, err := extract(t, doc, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for key, m := range meta {
		result[key] = m.Value()
	}
	return result
}

func Test_markdown_000(t *testing.T) {
	// The title is the first level one heading, without its markup, or else
	// the first heading, or else the title from the document's name
	for _, test := range []struct {
		doc, want string
	}{
		{"Some text\n\n## Introduction\n\n# Hello _world_ and `code`\n\n# Second\n", "Hello world and code"},
		{"Some text\n\n## Introduction\n\n### More\n", "Introduction"},
		{"Some text without a heading\n", "doc"},
	} {
		if got := values(t, test.doc)["dc:title"]; got != test.want {
			t.Errorf("%q: expected %q, got %q", test.doc, test.want, got)
		}
	}
}

func Test_markdown_001(t *testing.T) {
	// The fields of the front matter are Dublin Core, where there's an
	// element for them, and otherwise in the markdown namespace, and its
	// title is preferred to the headings
	doc := "---\n" +
		"Title: Release Notes\n" +
		"description: What's new\n" +
		"author: [Ada, Grace]\n" +
		"tags:\n  - go\n  - metadata\n" +
		"lang: en-GB\n" +
		"draft: true\n" +
		"og:image: cover.png\n" +
		"---\n" +
		"# Heading One\n"
	got := values(t, doc)
	for key, want := range map[string]string{
		"dc:title":          "Release Notes",
		"dc:description":    "What's new",
		"dc:creator":        "Ada, Grace",
		"dc:subject":        "go, metadata",
		"dc:language":       "en-GB",
		"markdown:draft":    "true",
		"markdown:og-image": "cover.png",
	} {
		if got[key] != want {
			t.Errorf("%s: expected %q, got %q", key, want, got[key])
		}
	}
	if _, exists := got["markdown:title"]; exists {
		t.Error("expected the title only as dc:title")
	}
}

func Test_markdown_002(t *testing.T) {
	// Without a title in the front matter, the title is from the headings,
	// and a document which only starts with a rule has no front matter
	if got := values(t, "---\nauthor: Ada\n---\n# Heading\n")["dc:title"]; got != "Heading" {
		t.Errorf("expected Heading, got %q", got)
	}
	if got := values(t, "---\n\n# Heading\n"); got["dc:title"] != "Heading" || got["dc:creator"] != "" {
		t.Errorf("expected no front matter, got %v", got)
	}
}

func Test_markdown_003(t *testing.T) {
	// Invalid front matter is an error, along with the title from the
	// headings
	meta, err := extract(t, "---\ntitle: [unclosed\n---\n# Heading\n")
	if err == nil {
		t.Error("expected an error")
	} else if title, exists := meta["dc:title"]; !exists || title.Value() != "Heading" {
		t.Errorf("expected the title Heading, got %v", meta)
	}
}

func Test_markdown_004(t *testing.T) {
	// The content is the document without its front matter, only when the
	// markdown namespace is requested
	doc := "---\ntitle: Notes\n---\n# Heading\r\n\r\nSome *text*.\r\n"
	got := values(t, doc, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if want := "# Heading\n\nSome *text*.\n"; got["markdown:body"] != want {
		t.Errorf("expected %q, got %q", want, got["markdown:body"])
	}
	if _, exists := values(t, doc)["markdown:body"]; exists {
		t.Error("expected no markdown:body without the markdown namespace")
	}
}

func Test_markdown_005(t *testing.T) {
	// A date in the front matter is the text it's written as, rather than a
	// time, wherever it is
	doc := "---\n" +
		"date: 2024-05-01\n" +
		"updated: 2024-05-02T10:30:00+01:00\n" +
		"history: [2023-01-01, 2024-01-01]\n" +
		"---\n"
	got := values(t, doc)
	for key, want := range map[string]string{
		"dc:date":          "2024-05-01",
		"markdown:updated": "2024-05-02T10:30:00+01:00",
		"markdown:history": "[2023-01-01 2024-01-01]",
	} {
		if got[key] != want {
			t.Errorf("%s: expected %q, got %q", key, want, got[key])
		}
	}
}

func Test_markdown_006(t *testing.T) {
	// A field of the front matter whose name is a key of the content is
	// prefixed, so it doesn't clash with the content
	doc := "---\nbody: field\nNote: a note\nnext: 2\noffset: 1\n---\n# Heading\n"
	got := values(t, doc, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	for key, want := range map[string]string{
		"markdown:body":               "# Heading\n",
		"markdown:frontmatter-body":   "field",
		"markdown:frontmatter-note":   "a note",
		"markdown:frontmatter-next":   "2",
		"markdown:frontmatter-offset": "1",
	} {
		if got[key] != want {
			t.Errorf("%s: expected %q, got %q", key, want, got[key])
		}
	}
	for _, key := range []string{"markdown:note", "markdown:next", "markdown:offset"} {
		if _, exists := got[key]; exists {
			t.Errorf("%s: expected no value, got %q", key, got[key])
		}
	}
}
