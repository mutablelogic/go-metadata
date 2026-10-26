package text_test

import (
	"bytes"
	"context"
	"mime"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/image/svg"
	_ "github.com/mutablelogic/go-metadata/text"
)

// markdownOf returns the Markdown of the data of the type, in the range,
// with its metadata, by namespace and key
func markdownOf(t *testing.T, data, contentType, byteRange string) (string, map[string]any) {
	t.Helper()
	_, meta, err := metadata.Extract(context.Background(), bytes.NewReader([]byte(data)), metadata.WithContentType(contentType), metadata.WithNamespace(metadata.MarkdownNamespace, metadata.MetadataNamespace), metadata.WithRange(byteRange))
	if err != nil {
		t.Fatalf("%s: %v", contentType, err)
	}
	values := make(map[string]any)
	body, found := "", false
	for _, m := range meta {
		values[m.NS()+":"+m.Key()] = m.Any()
		if m.NS() == metadata.MarkdownNamespace && m.Key() == "body" {
			if found {
				t.Errorf("%s: more than one markdown:body", contentType)
			}
			body, found = m.Value(), true
		}
	}
	if !found {
		t.Fatalf("%s: expected markdown:body", contentType)
	}
	return body, values
}

func Test_stream_000(t *testing.T) {
	// Plain text which is longer than 16 MiB is read beyond it, a part at a
	// time
	text := strings.Repeat("0123456789", 2<<20) + "the end"
	body, values := markdownOf(t, text, "text/plain", "20971520:")
	if body != "the end" {
		t.Errorf("expected the end of the text, got %q", body)
	}
	if values["markdown:next"] != nil {
		t.Errorf("expected no next part")
	}
}

func Test_stream_001(t *testing.T) {
	// Other names of types of source code, data which is JSON, by its
	// suffix, and GeoJSON, by its extension, are code blocks in their
	// languages, and data
	for _, test := range []struct {
		contentType, language, kind string
	}{
		{"application/javascript", "javascript", "code"},
		{"application/x-yaml", "yaml", "data"},
		{"text/yaml", "yaml", "data"},
		{"application/x-sh", "sh", "code"},
		{"application/ld+json", "json", "data"},
		{"application/geo+json", "json", "data"},
		{"application/x-ndjson", "json", "data"},
		{"application/vnd.example_record+json", "json", "data"},
		{mime.TypeByExtension(".geojson"), "json", "data"},
	} {
		body, values := markdownOf(t, "x = 1\n", test.contentType, ":")
		if want := "```" + test.language + "\nx = 1\n```\n"; body != want {
			t.Errorf("%s: expected %q, got %q", test.contentType, want, body)
		}
		if values["metadata:kind"] != test.kind {
			t.Errorf("%s: expected kind %q, got %v", test.contentType, test.kind, values["metadata:kind"])
		}
	}
}

func Test_stream_002(t *testing.T) {
	// Text which no other handler reads is plain text, and text which
	// another handler reads isn't, as well
	body, values := markdownOf(t, "a *line*", "text/x-log", ":")
	if body != `a \*line\*` {
		t.Errorf("expected the text as Markdown, got %q", body)
	}
	if values["metadata:kind"] != nil {
		t.Errorf("expected no kind, got %v", values["metadata:kind"])
	}
	markdownOf(t, "name,size\nada,1\n", "text/csv", ":")
	markdownOf(t, "a *line*", "text/plain", ":")
}

func Test_stream_003(t *testing.T) {
	// A Markdown document's front matter, with any line endings, isn't its
	// body, and without its end, it's all body
	for _, test := range []struct {
		document, want string
	}{
		{"---\ntitle: A\n---\n# Body\n", "# Body\n"},
		{"---\r\ntitle: A\r\n...\r\n# Body\r\n", "# Body\n"},
		{"---\ntitle: A\n", "---\ntitle: A\n"},
		{"# Body\r\ntext", "# Body\ntext"},
		{"---\n" + strings.Repeat("a", 2<<20), "---\n" + strings.Repeat("a", 2<<20)},
		{"---\n" + strings.Repeat("a", 20<<10) + "\n---\nBody", "Body"},
	} {
		if body, _ := markdownOf(t, test.document, "text/markdown", ":"); body != test.want {
			t.Errorf("%q: expected %q, got %q", test.document[:min(len(test.document), 40)], test.want[:min(len(test.want), 40)], body[:min(len(body), 40)])
		}
	}
}

func Test_stream_004(t *testing.T) {
	// An SVG image's source is an XML code block, and an XML document's,
	// which is longer than 16 MiB, is read beyond it
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`
	if body, _ := markdownOf(t, svg, "image/svg+xml", ":"); body != "```xml\n"+svg+"\n```\n" {
		t.Errorf("expected the image's source, got %q", body)
	}
	xml := "<root>" + strings.Repeat("<a/>", 5<<20) + "</root>"
	if body, _ := markdownOf(t, xml, "application/xml", "20971520:"); !strings.HasSuffix(body, "<a/></root>\n```\n") {
		t.Errorf("expected the end of the document, got %q", body[max(0, len(body)-40):])
	}
}
