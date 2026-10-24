package msoffice_test

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/application/msoffice"
)

// extract returns the metadata of the file in testfiles, by key
func extract(t *testing.T, name string, opts ...metadata.Option) (map[string]string, error) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "testfiles", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, meta, err := metadata.Extract(context.Background(), f, opts...)
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return result, err
}

func Test_msoffice_000(t *testing.T) {
	// The properties of a workbook, from its summary information, and its
	// document summary information
	got, err := extract(t, "sample.xls")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"dc:title":             "Sample Workbook",
		"dc:subject":           "Samples",
		"dc:creator":           "Ada Lovelace",
		"dc:description":       "A sample document",
		"dc:date":              "2024-05-01T09:00:00Z",
		"msoffice:keywords":    "sample, legacy",
		"msoffice:last-author": "Grace Hopper",
		"msoffice:modified":    "2024-05-02T10:30:00Z",
		"msoffice:application": "Microsoft Excel",
		"msoffice:company":     "Analytical Engines Ltd",
		"dc:type":              "Dataset",
		"metadata:kind":        "spreadsheet",
	}
	if !maps.Equal(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func Test_msoffice_001(t *testing.T) {
	// A title which is the name of the file the document was made from is the
	// title from the document's name
	got, err := extract(t, "sample.doc")
	if err != nil {
		t.Fatal(err)
	} else if got["dc:title"] != "sample" || got["dc:creator"] != "Ada Lovelace" {
		t.Errorf("expected the title sample, by Ada Lovelace, got %v", got)
	}
}

func Test_msoffice_002(t *testing.T) {
	// The content can't be read, which a note says, rather than the
	// properties
	got, err := extract(t, "sample.xls", metadata.WithNamespace(metadata.MarkdownNamespace))
	if err != nil {
		t.Fatal(err)
	} else if len(got) != 1 || !strings.Contains(got["markdown:note"], "can't be read") {
		t.Errorf("expected only a note, got %v", got)
	}
}

func Test_msoffice_003(t *testing.T) {
	// A document which isn't a compound file, such as a .doc file which is
	// RTF, is a bad parameter
	_, _, err := metadata.Extract(context.Background(), bytes.NewReader([]byte(`{\rtf1 Hello}`)), metadata.WithContentType("application/msword"))
	if !errors.Is(err, metadata.ErrBadParameter) {
		t.Errorf("expected ErrBadParameter, got %v", err)
	}
}
