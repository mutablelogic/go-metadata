package plain_test

import (
	"bytes"
	"context"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/text/plain"
)

// markdown returns the Markdown of the text, or the part of it in the range
func markdown(t *testing.T, data []byte, opts ...metadata.Option) string {
	t.Helper()
	opts = append(opts, metadata.WithContentType("text/plain"), metadata.WithNamespace(metadata.MarkdownNamespace))
	_, meta, err := metadata.Extract(context.Background(), bytes.NewReader(data), opts...)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == metadata.MarkdownNamespace && m.Key() == "body" {
			return m.Value()
		}
	}
	t.Fatal("expected markdown:body")
	return ""
}

func Test_plain_000(t *testing.T) {
	// The text is decoded from UTF-8, its byte order mark, or Windows-1252
	for _, test := range []struct {
		name string
		data []byte
		want string
	}{
		{"utf-8", []byte("Café"), "Café"},
		{"utf-8 bom", []byte("\xEF\xBB\xBFCafé"), "Café"},
		{"utf-16le bom", []byte("\xFF\xFEC\x00a\x00f\x00\xE9\x00"), "Café"},
		{"utf-16be bom", []byte("\xFE\xFF\x00C\x00a\x00f\x00\xE9"), "Café"},
		{"windows-1252", []byte("Caf\xe9 \x93quoted\x94"), "Café “quoted”"},
	} {
		if got := markdown(t, test.data, metadata.WithRange(":")); got != test.want {
			t.Errorf("%s: expected %q, got %q", test.name, test.want, got)
		}
	}
}

func Test_plain_001(t *testing.T) {
	// The characters which are Markdown are escaped, line breaks are kept,
	// a blank line separates paragraphs, and indentation isn't code
	data := []byte("# Not a heading\r\n" +
		"some *emphasis* and a [link](x) and <tag>\n" +
		"- not a list\n" +
		"1. not a numbered list\n" +
		"\n" +
		"    indented\n" +
		"\tand a tab")
	want := "\\# Not a heading  \n" +
		"some \\*emphasis\\* and a \\[link\\](x) and \\<tag\\>  \n" +
		"\\- not a list  \n" +
		"1\\. not a numbered list\n" +
		"\n" +
		"    indented  \n" +
		"    and a tab"
	if got := markdown(t, data, metadata.WithRange(":")); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func Test_plain_002(t *testing.T) {
	// The Markdown is the first kilobyte by default
	data := bytes.Repeat([]byte("0123456789"), 200)
	if got := markdown(t, data); got != string(data[:1024]) {
		t.Errorf("expected the first kilobyte, got %d bytes", len(got))
	}
}

func Test_plain_003(t *testing.T) {
	// The Markdown is only returned when the markdown namespace is requested
	_, meta, err := metadata.Extract(context.Background(), bytes.NewReader([]byte("text")), metadata.WithContentType("text/plain"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == metadata.MarkdownNamespace {
			t.Errorf("expected no markdown, got %s:%s", m.NS(), m.Key())
		}
	}
}
