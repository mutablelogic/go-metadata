//go:build cgo

package heif_test

import (
	"context"
	"io"
	"os"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

const (
	TEST_DIR = "../../testfiles"
)

// getMetadata returns the metadata of the reader, of the content type
func getMetadata(ctx context.Context, r io.Reader, contentType string, opts ...metadata.Option) ([]metadata.Metadata, error) {
	_, meta, err := metadata.Extract(ctx, r, append(opts, metadata.WithContentType(contentType))...)
	return meta, err
}

// key returns the key of the metadata, with its namespace
func key(m metadata.Metadata) string {
	return m.NS() + string(metadata.Separator) + m.Key()
}

// contentTypeForFile returns the content type of the file at the path
func contentTypeForFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	contentType, _, err := metadata.ContentType(f)
	if err != nil {
		t.Fatal(err)
	}
	return contentType
}
