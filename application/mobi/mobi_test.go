package mobi_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/application/mobi"
)

const contentType = "application/x-mobipocket-ebook"

// sample returns the sample book
func sample(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testfiles", "sample.mobi"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// extract returns the metadata of the book, by key
func extract(t *testing.T, data []byte, opts ...metadata.Option) (map[string]string, error) {
	t.Helper()
	_, meta, err := metadata.Extract(context.Background(), bytes.NewReader(data), append(opts, metadata.WithContentType(contentType))...)
	result := make(map[string]string, len(meta))
	for _, m := range meta {
		result[m.NS()+string(metadata.Separator)+m.Key()] = m.Value()
	}
	return result, err
}

// patch returns the book with the 16-bit value at the offset of its first
// record, such as its compression, or its encryption
func patch(t *testing.T, data []byte, offset int, value uint16) []byte {
	t.Helper()
	data = bytes.Clone(data)
	record0 := int(binary.BigEndian.Uint32(data[78:]))
	binary.BigEndian.PutUint16(data[record0+offset:], value)
	return data
}

func Test_mobi_000(t *testing.T) {
	// The metadata, from the EXTH header: its authors, rather than its
	// contributors, its description as plain text, and its ISBN
	got, err := extract(t, sample(t))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"dc:title":       "A Sample Kindle Book",
		"dc:creator":     "Ada Lovelace, Charles Babbage",
		"dc:publisher":   "Analytical Press",
		"dc:description": "A sample book & its text.",
		"dc:identifier":  "9780306406157",
		"dc:subject":     "Computing, History",
		"dc:date":        "2024-05-01T09:00:00Z",
		"dc:language":    "en-GB",
		"dc:rights":      "Public domain",
		"mobi:asin":      "B000SAMPLE",
		"mobi:type":      "EBOK",
		"dc:type":        "Text",
		"metadata:kind":  "book",
	}
	if !maps.Equal(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func Test_mobi_001(t *testing.T) {
	// The cover, when artwork is requested, and the text, decompressed,
	// without its trailing entries, as Markdown, in the book's encoding, when
	// it's requested
	got, err := extract(t, sample(t), metadata.WithNamespace(metadata.ArtworkNamespace))
	if err != nil {
		t.Fatal(err)
	} else if got["artwork:cover"] != "image/png" || len(got) != 1 {
		t.Errorf("expected only the cover, got %v", got)
	}
	got, err = extract(t, sample(t), metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# A Sample Kindle Book\n\nIt was a **bright** cold day in April, and the clocks were striking thirteen. It was a bright cold day in April, again, as the text repeats itself, so it compresses.\n\n## Chapter Two\n\n- first\n- second\n\nCafé au lait, and a \\*star\\*."
	if got["markdown:body"] != want {
		t.Errorf("expected %q, got %q", want, got["markdown:body"])
	}
}

func Test_mobi_002(t *testing.T) {
	// A book whose text is encrypted, or compressed in a way which isn't
	// supported, has a note, rather than its text, and its metadata
	for name, test := range map[string]struct {
		offset int
		value  uint16
		note   string
	}{
		"encrypted": {12, 2, "DRM"},
		"huffman":   {0, 17480, "isn't supported"},
	} {
		data := patch(t, sample(t), test.offset, test.value)
		got, err := extract(t, data, metadata.WithNamespace(metadata.MarkdownNamespace))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		} else if _, exists := got["markdown:body"]; exists || !strings.Contains(got["markdown:note"], test.note) {
			t.Errorf("%s: expected a note, got %v", name, got)
		}
		if got, err := extract(t, data); err != nil || got["dc:title"] != "A Sample Kindle Book" {
			t.Errorf("%s: expected the metadata, got %v, %v", name, got, err)
		}
	}
	if got, _ := extract(t, patch(t, sample(t), 12, 2)); got["mobi:encrypted"] != "true" {
		t.Errorf("expected mobi:encrypted, got %v", got)
	}
}

func Test_mobi_003(t *testing.T) {
	// A file which isn't a book, or is cut short, is a bad parameter
	data := sample(t)
	for name, data := range map[string][]byte{
		"not a book": []byte(strings.Repeat("not a mobi book ", 10)),
		"cut short":  data[:70],
		"empty":      nil,
	} {
		if _, err := extract(t, data); !errors.Is(err, metadata.ErrBadParameter) {
			t.Errorf("%s: expected ErrBadParameter, got %v", name, err)
		}
	}
}
