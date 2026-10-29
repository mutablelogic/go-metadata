package main

import (
	"bytes"
	"errors"
	"image"
	imagepng "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Packages
	lipgloss "github.com/charmbracelet/lipgloss"
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	xmp "github.com/mutablelogic/go-metadata/pkg/xmp"
)

func Test_table_000(t *testing.T) {
	// A table wider than the width is narrowed to fit it, and a table which
	// is narrower, or has no width, isn't changed
	headers := []string{"Path", "Content Type", "Key", "Value"}
	rows := [][]string{
		{"testfiles/index.html", "text/html; charset=utf-8", "dc:title", "picofuse: picofuse"},
		{"", "", "dc:description", strings.Repeat("a long description ", 10)},
	}
	natural := lipgloss.Width(renderTable(headers, rows, 0))

	for _, width := range []int{60, 100} {
		table := renderTable(headers, rows, width)
		for _, line := range strings.Split(table, "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line is %d wide: %q", width, w, line)
			}
		}
		if lines := strings.Count(table, "\n") + 1; lines != len(rows)+2 {
			t.Errorf("width %d: expected %d lines, got %d", width, len(rows)+2, lines)
		}
	}
	if table := renderTable(headers, rows, natural+10); lipgloss.Width(table) != natural {
		t.Errorf("expected a width of %d, got %d", natural, lipgloss.Width(table))
	}
}

func Test_glob_000(t *testing.T) {
	// A directory matches every file within it, a file matches itself, even
	// when its name has characters which are in patterns, and anything else
	// is a pattern, split into its directory and the pattern within it
	dir := t.TempDir()
	file := filepath.Join(dir, "x[1].gif")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path, base, pattern string
	}{
		{dir, dir, "**"},
		{file, dir, `x\[1\].gif`},
		{"testfiles/**/*.jpg", "testfiles", "**/*.jpg"},
		{"*.jpg", ".", "*.jpg"},
		{"../a/b/*.{jpg,png}", filepath.FromSlash("../a/b"), "*.{jpg,png}"},
		{`a/Other\ Backups/June/**`, filepath.FromSlash("a/Other Backups/June"), "**"},
		{`a/back\\slash/*`, filepath.FromSlash(`a/back\slash`), "*"},
	} {
		cli := CLI{Path: test.path}
		if base, pattern := cli.glob(); base != test.base || pattern != test.pattern {
			t.Errorf("%q: expected %q %q, got %q %q", test.path, test.base, test.pattern, base, pattern)
		}
	}
}

func Test_range_000(t *testing.T) {
	// A range can only be used with --markdown, and is the whole document
	// by default
	if err := (&CLI{Range: "0:100"}).Validate(); err == nil {
		t.Error("expected an error without --markdown")
	}
	if err := (&CLI{Range: "100", Markdown: true}).Validate(); err == nil {
		t.Error("expected an error for a range without a colon")
	}
	cli := CLI{Range: "10:100", Markdown: true}
	if err := cli.Validate(); err != nil || cli.offset != 10 || cli.limit != 90 {
		t.Errorf("expected 10 90, got %d %d %v", cli.offset, cli.limit, err)
	}
	cli = CLI{Markdown: true}
	if err := cli.Validate(); err != nil || cli.Range != ":" || cli.offset != 0 || cli.limit != 0 || cli.paged {
		t.Errorf("expected : 0 0, got %q %d %d %v", cli.Range, cli.offset, cli.limit, err)
	}
	if err := (&CLI{}).Validate(); err != nil {
		t.Errorf("expected no error without --markdown or --range, got %v", err)
	}
}

func Test_artwork_000(t *testing.T) {
	// The artwork is saved as images named by their keys, which aren't
	// overwritten unless it's forced
	var png bytes.Buffer
	if err := imagepng.Encode(&png, image.NewGray(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	r := result{Path: "doc.pdf", Metadata: map[string]any{
		"artwork:page0001": meta.NewImage("artwork", "page0001", png.Bytes()),
		"dc:title":         "doc",
	}}
	r.keys = []string{"artwork:page0001", "dc:title"}

	// The images are saved in the current directory
	t.Chdir(t.TempDir())
	cli := CLI{Artwork: true}
	var out bytes.Buffer
	if err := cli.writeArtwork(&out, r); err != nil {
		t.Fatal(err)
	}
	path := "page0001.png"
	if strings.TrimSpace(out.String()) != path {
		t.Errorf("expected %q, got %q", path, out.String())
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, png.Bytes()) {
		t.Errorf("expected the image, got %d bytes, %v", len(data), err)
	}

	// An image which exists isn't overwritten, unless it's forced
	if err := cli.writeArtwork(io.Discard, r); !errors.Is(err, fs.ErrExist) || !strings.Contains(err.Error(), "page0001.png already exists") {
		t.Errorf("expected an error, got %v", err)
	}
	cli.Force = true
	if err := cli.writeArtwork(io.Discard, r); err != nil {
		t.Error(err)
	}
}

func Test_artwork_001(t *testing.T) {
	// The extension of an image is from its type
	for contentType, want := range map[string]string{
		"image/jpeg":            ".jpg",
		"image/png":             ".png",
		"image/gif":             ".gif",
		"image/webp":            ".webp",
		"application/x-unknown": ".bin",
	} {
		if got := imageExtension(contentType); got != want {
			t.Errorf("%s: expected %s, got %s", contentType, want, got)
		}
	}
}

func Test_xmp_000(t *testing.T) {
	// The metadata of a file is an XMP packet, which describes the file by
	// its path, without its artwork or Markdown, and a file without
	// metadata has none
	var png bytes.Buffer
	if err := imagepng.Encode(&png, image.NewGray(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	r := result{Path: "photos/a.jpg", meta: []metadata.Metadata{
		meta.New("dc", "title", "A & B"),
		meta.New("image", "width", 640),
		meta.NewImage("artwork", "thumbnail", png.Bytes()),
		meta.New("markdown", "body", "# A"),
		nil,
	}}
	var out bytes.Buffer
	if err := (&CLI{XMP: true}).writeXMP(&out, r); err != nil {
		t.Fatal(err)
	}
	x, err := xmp.Parse(out.Bytes())
	if err != nil {
		t.Fatal(err)
	} else if x.About() != "photos/a.jpg" {
		t.Errorf("expected photos/a.jpg, got %q", x.About())
	}
	if got := x.First("dc:title"); got == nil || got.Value() != "A & B" {
		t.Errorf("expected dc:title, got %v", got)
	}
	if got := x.First("image:width"); got == nil || got.Value() != "640" {
		t.Errorf("expected image:width, got %v", got)
	}
	for _, key := range []string{"artwork:thumbnail", "markdown:body"} {
		if got := x.Get(key); len(got) != 0 {
			t.Errorf("expected no %s, got %v", key, got)
		}
	}

	out.Reset()
	if err := (&CLI{XMP: true}).writeXMP(&out, result{Path: "empty.bin"}); err != nil || out.Len() != 0 {
		t.Errorf("expected nothing, got %q %v", out.String(), err)
	}
}

func Test_result_000(t *testing.T) {
	// A key which a file has more than once, such as the identifiers of a
	// recording's artists, is a list of its values, without the same value
	// twice, and is separated by semicolons in a table
	r := result{Metadata: map[string]any{}}
	for _, m := range []metadata.Metadata{
		meta.New("musicbrainz", "artist-id", "a"),
		meta.New("musicbrainz", "artist-id", "b"),
		meta.New("musicbrainz", "artist-id", "a"),
		meta.New("dc", "title", "Title"),
	} {
		r.add(m.NS()+":"+m.Key(), jsonValue(m))
	}
	if ids := values(r.Metadata["musicbrainz:artist-id"]); len(ids) != 2 || valueString(ids[0]) != "a" || valueString(ids[1]) != "b" {
		t.Errorf("unexpected artist identifiers %v", ids)
	}
	if got := tableValue(r.Metadata["musicbrainz:artist-id"]); got != "a; b" {
		t.Errorf("unexpected table value %q", got)
	}
	if title, ok := r.value("dc:title"); !ok || title != "Title" {
		t.Errorf("unexpected title %q", title)
	}
	if _, ok := r.value("musicbrainz:artist-id"); ok {
		t.Error("expected no single value of a list")
	}
}

func Test_note_000(t *testing.T) {
	// A PDF which is cut short has a note of why it can't be read, once,
	// and when the metadata namespace isn't printed, the warning instead
	data, err := os.ReadFile(filepath.Join("..", "..", "testfiles", "sample_with_thumbnails.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "truncated.pdf")
	if err := os.WriteFile(path, data[:len(data)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	for _, ns := range [][]string{nil, {"dc"}} {
		cli := &CLI{Namespace: ns}
		r := &result{Path: path}
		if err := cli.describe(t.Context(), r); err != nil {
			t.Fatal(err)
		}
		note, _ := r.Metadata["metadata:note"].(string)
		if ns == nil && !strings.HasPrefix(note, "It can't be read, so it may be damaged or incomplete: ") {
			t.Errorf("expected the note, got %q", note)
		} else if ns != nil && !strings.HasPrefix(note, "bad parameter: ") {
			t.Errorf("with %v, expected the warning, got %q", ns, note)
		}
	}
}
