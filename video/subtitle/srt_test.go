package subtitle_test

import (
	"context"
	"io"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/video/subtitle"
)

// namedReader is a reader with a file name
type namedReader struct {
	*strings.Reader
	name string
}

func (r namedReader) Name() string { return r.name }

// srt has a BOM, CRLF line endings, styling, a repeated cue, a cue without a
// blank line after it, Markdown's characters, and a line in Windows-1252
const srt = "\uFEFF1\r\n00:00:01,000 --> 00:00:03,500\r\n<i>Hello</i> there.\r\n{\\an8}General Kenobi!\r\n\r\n" +
	"2\r\n00:00:04,000 --> 00:00:05,000\r\nHello there.\r\nGeneral Kenobi!\r\n\r\n" +
	"3\r\n00:01:02.5 --> 00:01:04,000\r\n*Not* bold, [not] a link\r\n" +
	"4\r\n01:02:03,000 --> 01:02:05,000\r\nCaf\xe9 au lait\r\n\r\n" +
	"5\r\n01:02:06,000 --> 01:02:07,000\r\n<font color=\"#fff\"></font>\r\n"

const srtMarkdown = "**1 · 00:00:01 → 00:00:03** Hello there.\nGeneral Kenobi!\n\n" +
	"**3 · 00:01:02 → 00:01:04** \\*Not\\* bold, \\[not\\] a link\n\n" +
	"**4 · 01:02:03 → 01:02:05** Café au lait\n"

// extract returns the metadata of the subtitles, by key
func extract(t *testing.T, r io.Reader, opts ...metadata.Option) map[string]string {
	t.Helper()
	_, meta, err := metadata.Extract(context.Background(), r, append(opts, metadata.WithContentType("application/x-subrip"))...)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]string{}
	for _, m := range meta {
		result[m.NS()+":"+m.Key()] = m.Value()
	}
	return result
}

// markdown returns the body and next of the subtitles' Markdown
func markdown(t *testing.T, data string, opts ...metadata.Option) (string, string) {
	t.Helper()
	got := extract(t, strings.NewReader(data), append(opts, metadata.WithNamespace(metadata.MarkdownNamespace))...)
	return got["markdown:body"], got["markdown:next"]
}

func Test_srt_000(t *testing.T) {
	// The cues are paragraphs, which start with their numbers and times,
	// without their styling, or the repeated cue, and with Markdown's
	// characters escaped, and .srt is SubRip subtitles
	if body, next := markdown(t, srt, metadata.WithRange(":")); body != srtMarkdown || next != "" {
		t.Errorf("got %q, next %q", body, next)
	}
	contentType, _, err := metadata.ContentType(namedReader{strings.NewReader(srt), "film.srt"})
	if err != nil || contentType != "application/x-subrip" {
		t.Errorf("expected application/x-subrip, got %q %v", contentType, err)
	}
}

func Test_srt_001(t *testing.T) {
	// A part of the Markdown, and the next
	if body, next := markdown(t, srt, metadata.WithRange("0:16")); body != srtMarkdown[:16] || next != "16" {
		t.Errorf("got %q, next %q", body, next)
	}
}

func Test_srt_002(t *testing.T) {
	// Without the markdown namespace, subtitles are only their kind
	got := extract(t, strings.NewReader(srt))
	if len(got) != 2 || got["metadata:kind"] != "subtitles" || got["dc:type"] != "Text" {
		t.Errorf("expected only the kind, got %v", got)
	}
}

func Test_srt_003(t *testing.T) {
	// UTF-16 subtitles, with a byte order mark, in either byte order
	for _, bigEndian := range []bool{false, true} {
		data := []byte{0xFF, 0xFE}
		if bigEndian {
			data = []byte{0xFE, 0xFF}
		}
		for _, r := range "1\r\n00:00:01,000 --> 00:00:02,000\r\nCafé\r\n" {
			if bigEndian {
				data = append(data, byte(r>>8), byte(r))
			} else {
				data = append(data, byte(r), byte(r>>8))
			}
		}
		if body, _ := markdown(t, string(data)); body != "**1 · 00:00:01 → 00:00:02** Café\n" {
			t.Errorf("big endian %v: got %q", bigEndian, body)
		}
	}
}

func Test_srt_004(t *testing.T) {
	// A cue without a number is the previous cue's and one, and the first
	// is one, and a cue whose end can't be read has its start
	const unnumbered = "00:00:01,000 --> 00:00:02,000\nOne\n\n00:00:03,000 --> 00:00:04,000\nTwo\n\n7\n00:00:05,000 --> 00:00:06,000\nSeven\n\n00:00:07,000 -->\nEight\n"
	want := "**1 · 00:00:01 → 00:00:02** One\n\n**2 · 00:00:03 → 00:00:04** Two\n\n**7 · 00:00:05 → 00:00:06** Seven\n\n**8 · 00:00:07** Eight\n"
	if body, _ := markdown(t, unnumbered, metadata.WithRange(":")); body != want {
		t.Errorf("got %q", body)
	}
}

func Test_srt_005(t *testing.T) {
	// The language of subtitles, and what they're for, are from their name,
	// and the title is the name of the video, rather than of the file
	got := extract(t, namedReader{strings.NewReader(srt), "/videos/Film.pt-BR.forced.srt"})
	for key, want := range map[string]string{
		"dc:language":     "pt-BR",
		"dc:title":        "Film",
		"subtitle:forced": "true",
		"metadata:kind":   "subtitles",
	} {
		if got[key] != want {
			t.Errorf("%s: expected %q, got %q", key, want, got[key])
		}
	}
}
