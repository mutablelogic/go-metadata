package mbox_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/message/mbox"
	_ "github.com/mutablelogic/go-metadata/pkg/mbox"
)

// named is a reader with a name, for its extension
type named struct {
	*strings.Reader
	name string
}

func (r named) Name() string { return r.name }

// extract returns the metadata of the mailbox, by key, with the options
func extract(t *testing.T, data string, opts ...metadata.Option) map[string]string {
	t.Helper()
	contentType, meta, err := metadata.Extract(context.Background(), named{strings.NewReader(data), "Inbox.mbx"}, opts...)
	if err != nil {
		t.Fatal(err)
	} else if contentType != "application/mbox; charset=utf-8" {
		t.Errorf("content type %q", contentType)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return result
}

// message returns a message in a mailbox, from the sender, with the subject,
// on the day of February 2002
func message(from, subject string, day int) string {
	return fmt.Sprintf("From ???@??? Mon Feb %2d 10:00:00 2002\r\nFrom: %s\r\nSubject: %s\r\n\r\nHello\r\n", day, from, subject)
}

func Test_mbox_000(t *testing.T) {
	// The messages, the dates of the first and last, which are from their
	// From lines, when they have no Date header, their subjects, and their
	// senders, those who sent the most first, by their names, whatever case
	// their addresses are in, and whether or not they're with their names
	got := extract(t, message("charles@example.com", "One", 12)+
		message("Ada Lovelace <ada@example.com>", "Two", 11)+
		message("ADA@example.com", "", 14)+
		message("Charles Babbage <charles@example.com>", "Three", 13)+
		message("ada@example.com", "Four", 15))
	want := map[string]string{
		"metadata:kind":   "mailbox",
		"dc:type":         "Collection",
		"dc:title":        "Inbox",
		"mbox:messages":   "5",
		"mbox:date-start": "2002-02-11T10:00:00Z",
		"mbox:date-end":   "2002-02-15T10:00:00Z",
		"mbox:subjects":   "One; Two; Three; Four",
		"mbox:senders":    "Ada Lovelace; Charles Babbage",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s: got %q, want %q", key, got[key], value)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %v", got)
	}
}

func Test_mbox_001(t *testing.T) {
	// Only the 1000 most common subjects, with a note, but every message is
	// counted
	var b strings.Builder
	for i := range 1002 {
		b.WriteString(message("ada@example.com", fmt.Sprintf("Message %d", i), 11))
	}
	got := extract(t, b.String())
	if got["mbox:messages"] != "1002" || strings.Count(got["mbox:subjects"], "; ") != 999 || !strings.Contains(got["mbox:note"], "1000 most common of the mailbox's 1002 subjects") {
		t.Errorf("messages %q, note %q", got["mbox:messages"], got["mbox:note"])
	}
}

func Test_mbox_003(t *testing.T) {
	// A subject is once, without the prefixes of replies and forwarded
	// messages, whatever case it's in, the most common first, and a sender
	// with more than one address is one sender, by their name
	got := extract(t, message("Tyler Bell <tyler@example.com>", "site", 11)+
		message("Tyler Bell <tyler@example.org>", "Spec", 11)+
		message("ada@example.com", "Re: Spec", 12)+
		message("Tyler Bell <TBell@example.net>", "RE: spec", 13)+
		message("ada@example.com", "Fwd: Re[2]: java", 14)+
		message("Ada Lovelace <ada@example.com>", "AW: java", 15)+
		message("ada@example.com", "Re:", 16))
	if want := "Spec; java; site; Re:"; got["mbox:subjects"] != want {
		t.Errorf("subjects %q, want %q", got["mbox:subjects"], want)
	}
	if want := "Ada Lovelace; Tyler Bell"; got["mbox:senders"] != want {
		t.Errorf("senders %q, want %q", got["mbox:senders"], want)
	}
}

func Test_mbox_002(t *testing.T) {
	// The messages as Markdown, a message without a subject with a heading,
	// so they're separated, only when the markdown namespace is requested,
	// a page at a time
	data := message("ada@example.com", "One", 11) + message("ada@example.com", "", 12)
	got := extract(t, data, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	want := "# One\n\n- **From:** ada@example.com\n- **Date:** Mon, 11 Feb 2002 10:00 +0000\n\nHello\n\n" +
		"# (no subject)\n\n- **From:** ada@example.com\n- **Date:** Tue, 12 Feb 2002 10:00 +0000\n\nHello\n"
	if got["markdown:body"] != want {
		t.Errorf("got %q\nwant %q", got["markdown:body"], want)
	}
	if got := extract(t, data); got["markdown:body"] != "" {
		t.Errorf("unexpected Markdown %q", got["markdown:body"])
	}
	if got := extract(t, data, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange("0:10")); got["markdown:body"] != "# One\n\n- *" || got["markdown:next"] != "10" {
		t.Errorf("got %q, next %q", got["markdown:body"], got["markdown:next"])
	}
}
