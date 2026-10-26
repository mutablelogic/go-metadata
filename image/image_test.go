package image_test

import (
	"context"
	"os"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/image"
)

func Test_image_000(t *testing.T) {
	// With all the handlers of images, an SVG image's Markdown is its
	// source, without a note that an image has none, which a raster image
	// has instead
	markdown := func(t *testing.T, path, contentType string) map[string]string {
		t.Helper()
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_, meta, err := metadata.Extract(context.Background(), f, metadata.WithContentType(contentType), metadata.WithNamespace(metadata.MarkdownNamespace))
		if err != nil {
			t.Fatal(err)
		}
		result := make(map[string]string)
		for _, m := range meta {
			result[m.NS()+":"+m.Key()] = m.Value()
		}
		return result
	}
	svg := markdown(t, "../testfiles/sample.svg", "image/svg+xml")
	if !strings.HasPrefix(svg["markdown:body"], "```xml\n") {
		t.Errorf("expected the SVG image's source, got %q", svg["markdown:body"])
	}
	if _, exists := svg["markdown:note"]; exists {
		t.Errorf("expected no note, got %q", svg["markdown:note"])
	}
	png := markdown(t, "../testfiles/sample.png", "image/png")
	if _, exists := png["markdown:body"]; exists || png["markdown:note"] == "" {
		t.Errorf("expected a note, and no body, got %v", png)
	}
}
