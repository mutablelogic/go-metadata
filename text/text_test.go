package text_test

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
	_ "github.com/mutablelogic/go-metadata/text"
)

const (
	TEST_DIR   = "../testfiles"
	TEST_FILES = "testfiles.json"
)

// testFile is the expected content type of a file in TEST_DIR, its
// metadata, by key, and its content as Markdown, from TEST_FILES
type testFile struct {
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata,omitempty"`
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

func Test_text_000(t *testing.T) {
	// Every text file in TEST_FILES with expected metadata has exactly that
	// metadata, and its content as Markdown, when it's expected
	data, err := os.ReadFile(filepath.Join(TEST_DIR, TEST_FILES))
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]testFile
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	for name, want := range expected {
		if !strings.HasPrefix(want.ContentType, "text/") || (want.Metadata == nil && want.Markdown == nil) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if want.Metadata != nil {
				if got := extract(t, name); !maps.Equal(got, want.Metadata) {
					t.Errorf("expected metadata %v, got %v", want.Metadata, got)
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
