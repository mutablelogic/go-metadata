package application_test

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/application"
)

const (
	TEST_DIR   = "../testfiles"
	TEST_FILES = "testfiles.json"
)

// testFile is the expected content type of a file in TEST_DIR, its
// metadata, by key, its artwork, by key, as its type, and its content as
// Markdown, from TEST_FILES
type testFile struct {
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Artwork     map[string]string `json:"artwork,omitempty"`
	Markdown    *string           `json:"markdown,omitempty"`
}

// extract returns the metadata of the file, by key, as strings
func extract(t *testing.T, name string, opts ...metadata.Option) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join(TEST_DIR, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, meta, err := metadata.Extract(context.Background(), f, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return result
}

func Test_application_000(t *testing.T) {
	// Every application document in TEST_FILES with expected metadata has
	// exactly that metadata, and that artwork and Markdown, when they're
	// expected
	data, err := os.ReadFile(filepath.Join(TEST_DIR, TEST_FILES))
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]testFile
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	for name, want := range expected {
		// A mailbox is read by the handler of the message package
		if !strings.HasPrefix(want.ContentType, "application/") || want.ContentType == "application/mbox" || (want.Metadata == nil && want.Artwork == nil && want.Markdown == nil) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if want.Metadata != nil {
				if got := extract(t, name); !maps.Equal(got, want.Metadata) {
					t.Errorf("expected metadata %v, got %v", want.Metadata, got)
				}
			}
			if want.Artwork != nil {
				if got := extract(t, name, metadata.WithNamespace(metadata.ArtworkNamespace)); !maps.Equal(got, want.Artwork) {
					t.Errorf("expected artwork %v, got %v", want.Artwork, got)
				}
			}
			if want.Markdown != nil {
				got := extract(t, name, metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
				if body := got["markdown:body"]; body != *want.Markdown {
					t.Errorf("expected markdown %q, got %q", *want.Markdown, body)
				}
			}
		})
	}
}
