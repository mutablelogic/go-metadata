package playlist

import (
	"context"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

// playlistMarkdown returns the Markdown of a playlist
func playlistMarkdown(t *testing.T, contentType, data string) string {
	t.Helper()
	_, meta, err := metadata.Extract(context.Background(), strings.NewReader(data), metadata.WithContentType(contentType), metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == metadata.MarkdownNamespace && m.Key() == "body" {
			return m.Value()
		}
	}
	return ""
}

func Test_markdown_000(t *testing.T) {
	// The title of a playlist is a heading, and its entries are a numbered
	// list, of their titles and durations, as links to URLs, and otherwise
	// followed by their locations, or their locations without a title
	got := playlistMarkdown(t, "audio/x-mpegurl", "#EXTM3U\n#PLAYLIST:Road Trip\n"+
		"#EXTINF:123,Artist One - First Song\nmusic/01.mp3\n"+
		"#EXTINF:200.5,Artist Two - Second [Live]\nhttp://example.com/02.mp3\n"+
		"#EXTINF:-1,Radio\nhttp://example.com/stream\n"+
		"#EXTINF:3723,\nlong_*file*.mp3\n"+
		"music/untitled.mp3\n")
	want := "# Road Trip\n\n" +
		"1. Artist One - First Song (2:03)\n   music/01.mp3\n" +
		"2. [Artist Two - Second \\[Live\\]](<http://example.com/02.mp3>) (3:21)\n" +
		"3. [Radio](<http://example.com/stream>)\n" +
		"4. long\\_\\*file\\*.mp3 (1:02:03)\n" +
		"5. music/untitled.mp3\n"
	if got != want {
		t.Errorf("expected\n%q\ngot\n%q", want, got)
	}
}

func Test_markdown_001(t *testing.T) {
	// A PLS playlist, without a title, and no Markdown unless it's requested
	got := playlistMarkdown(t, "audio/x-scpls", "[playlist]\nFile1=a.mp3\nTitle1=A\nLength1=60\nNumberOfEntries=1\n")
	if got != "1. A (1:00)\n   a.mp3\n" {
		t.Errorf("unexpected %q", got)
	}
	_, meta, err := metadata.Extract(context.Background(), strings.NewReader("#EXTM3U\na.mp3\n"), metadata.WithContentType("audio/x-mpegurl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == metadata.MarkdownNamespace {
			t.Errorf("expected no markdown, got %s", m.Key())
		}
	}
}

func Test_markdown_002(t *testing.T) {
	// A duration of zero isn't known, as Zoom's playlists have, so it isn't
	// shown, and there's no total duration
	data := "#EXTM3U\n#EXTINF:0,Meeting - zoom_0.mp4\nzoom_0.mp4\n#EXTINF:0,Meeting - zoom_1.mp4\nzoom_1.mp4\n"
	if got := playlistMarkdown(t, "audio/x-mpegurl", data); got != "1. Meeting - zoom\\_0.mp4\n   zoom\\_0.mp4\n2. Meeting - zoom\\_1.mp4\n   zoom\\_1.mp4\n" {
		t.Errorf("unexpected %q", got)
	}
	_, meta, err := metadata.Extract(context.Background(), strings.NewReader(data), metadata.WithContentType("audio/x-mpegurl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if m.NS() == Namespace && m.Key() == "duration" {
			t.Errorf("expected no duration, got %q", m.Value())
		}
	}
}
