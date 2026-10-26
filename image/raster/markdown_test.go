package raster_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/image/raster"
)

func Test_markdown_000(t *testing.T) {
	// An image has a note, rather than Markdown, when the markdown namespace
	// is requested, and none otherwise
	for _, contentType := range []string{"image/png", "image/jpeg"} {
		meta, err := getMetadata(context.Background(), bytes.NewReader(nil), contentType, metadata.WithNamespace(metadata.MarkdownNamespace))
		if err != nil {
			t.Fatal(err)
		} else if len(meta) != 1 || key(meta[0]) != "markdown:note" || !strings.Contains(meta[0].Value(), "image") {
			t.Errorf("%s: expected markdown:note, got %v", contentType, meta)
		}
		meta, err = getMetadata(context.Background(), bytes.NewReader(nil), contentType)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range meta {
			if m.NS() == metadata.MarkdownNamespace {
				t.Errorf("%s: expected no markdown, got %s", contentType, key(m))
			}
		}
	}
}

func Test_markdown_001(t *testing.T) {
	// An SVG image has no note, as its source is its Markdown
	meta, err := getMetadata(context.Background(), bytes.NewReader(nil), "image/svg+xml", metadata.WithNamespace(metadata.MarkdownNamespace))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meta {
		if key(m) == "markdown:note" {
			t.Errorf("expected no note, got %q", m.Value())
		}
	}
}
