package office_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	testassert "github.com/mutablelogic/go-metadata/internal/testassert"
)

var (
	assert  = testassert.Assert
	require = testassert.Require
)

// getMetadata returns the metadata of the reader, of the content type
func getMetadata(ctx context.Context, r io.Reader, contentType string, opts ...metadata.Option) ([]metadata.Metadata, error) {
	_, meta, err := metadata.Extract(ctx, r, append(opts, metadata.WithContentType(contentType))...)
	return meta, err
}

// rangeOf returns the range of limit bytes from the offset, as WithRange
// takes it, or the rest when the limit is zero
func rangeOf(offset, limit uint64) string {
	if limit == 0 {
		return fmt.Sprintf("%d:", offset)
	}
	return fmt.Sprintf("%d:%d", offset, offset+limit)
}

// coverPNG returns a picture, as a cover or thumbnail
func coverPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 40, 60))))
	return buf.Bytes()
}
