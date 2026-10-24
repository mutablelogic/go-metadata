package email_test

import (
	"context"
	"maps"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/message/email"
	_ "github.com/mutablelogic/go-metadata/pkg/eml"
)

// named is a reader with a name, for its extension and title
type named struct {
	*strings.Reader
	name string
}

func (r named) Name() string { return r.name }

// lines returns the lines joined with CRLF
func lines(lines ...string) string {
	return strings.Join(lines, "\r\n") + "\r\n"
}

// extract returns the metadata of the email with the name, by key, with the
// options
func extract(t *testing.T, email, name string, opts ...metadata.Option) map[string]string {
	t.Helper()
	_, meta, err := metadata.Extract(context.Background(), named{strings.NewReader(email), name}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return result
}

func Test_email_000(t *testing.T) {
	// An email is detected as text, and refined by its extension
	for _, name := range []string{"hello.eml", "Hello.EML"} {
		contentType, _, err := metadata.ContentType(named{strings.NewReader(lines("From: a@example.com", "Subject: Hello", "", "Hi")), name})
		if err != nil {
			t.Fatal(err)
		} else if contentType != "message/rfc822" {
			t.Errorf("%s: got %q", name, contentType)
		}
	}
}

func Test_email_001(t *testing.T) {
	// The subject is the title, the sender's name the creator, the start of
	// the text, without its quoted lines, the description, and the
	// addresses are lists
	got := extract(t, lines(
		"From: Ada Lovelace <ada@example.com>",
		"To: Charles Babbage <charles@example.com>, mary@example.com",
		"Cc: <lord@example.com>",
		"Subject: The Analytical Engine",
		"Date: Wed, 21 Oct 2026 09:30:00 +0200",
		"Message-ID: <abc123@example.com>",
		"",
		"Dear Charles,",
		"",
		"Here are my   notes on the engine.",
		"> Your earlier letter",
	), "letter.eml")
	want := map[string]string{
		"metadata:kind":  "email",
		"dc:type":        "Text",
		"dc:title":       "The Analytical Engine",
		"dc:creator":     "Ada Lovelace",
		"dc:date":        "2026-10-21T07:30:00Z",
		"dc:identifier":  "abc123@example.com",
		"dc:description": "Dear Charles, Here are my notes on the engine.",
		"email:from":     "Ada Lovelace <ada@example.com>",
		"email:to":       "Charles Babbage <charles@example.com>, mary@example.com",
		"email:cc":       "lord@example.com",
	}
	if !maps.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func Test_email_002(t *testing.T) {
	// Without a subject, the title is the file's name, and the creator is
	// the sender's address, when they have no name. The description stops
	// at the original message which it replies to.
	got := extract(t, lines(
		"From: ada@example.com",
		"",
		"Yes, let's.",
		"",
		"-----Original Message-----",
		"From: Charles",
		"Shall we meet?",
	), "a-note.eml")
	if got["dc:title"] != "a-note" || got["dc:creator"] != "ada@example.com" || got["dc:description"] != "Yes, let's." {
		t.Errorf("got %v", got)
	}
}

func Test_email_003(t *testing.T) {
	// A long text is cut short, at a word
	got := extract(t, lines("Subject: Long", "", strings.Repeat("word ", 400)), "long.eml")["dc:description"]
	if len(got) > 1000+len("…") || !strings.HasSuffix(got, "word…") {
		t.Errorf("got %q", got)
	}
}

func Test_email_004(t *testing.T) {
	// The headers and text as Markdown, with the text's line breaks, and
	// its quoted lines as block quotes, as deep as they're quoted, only
	// when the markdown namespace is requested
	email := lines(
		"From: Ada Lovelace <ada@example.com>",
		"To: charles@example.com",
		"Subject: Re: *Engine*",
		"Date: Wed, 21 Oct 2026 09:30:00 +0200",
		"X-Attachments: C:\\EUDORA\\attach\\notes.doc;",
		"",
		"Yes,",
		"tomorrow.",
		"",
		"> Shall we meet?",
		"> > About the engine",
	)
	want := "# Re: \\*Engine\\*\n\n" +
		"- **From:** Ada Lovelace \\<ada@example.com\\>\n" +
		"- **To:** charles@example.com\n" +
		"- **Date:** Wed, 21 Oct 2026 09:30 +0200\n" +
		"- **Attachments:** notes.doc\n\n" +
		"Yes,\\\ntomorrow.\n\n" +
		"> Shall we meet?\n\n" +
		"> > About the engine\n"
	got := extract(t, email, "re.eml", metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if got["markdown:body"] != want {
		t.Errorf("got %q\nwant %q", got["markdown:body"], want)
	}
	if got := extract(t, email, "re.eml"); got["markdown:body"] != "" {
		t.Errorf("unexpected Markdown %q", got["markdown:body"])
	}
}

func Test_email_005(t *testing.T) {
	// A message with only HTML, whose description is its text, and which is
	// converted to Markdown
	email := lines(
		"Subject: News",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>Hello <b>world</b></p><ul><li>one</li></ul>",
	)
	if got := extract(t, email, "news.eml")["dc:description"]; got != "Hello world one" {
		t.Errorf("description %q", got)
	}
	got := extract(t, email, "news.eml", metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if want := "# News\n\nHello **world**\n\n- one\n"; got["markdown:body"] != want {
		t.Errorf("got %q\nwant %q", got["markdown:body"], want)
	}
}
