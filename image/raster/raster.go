// Package raster registers handlers of raster images, such as JPEG, PNG,
// GIF, BMP, TIFF and WebP images, which return their format and size, and
// the image itself as artwork
package raster

import (
	"bytes"
	"context"
	stdimage "image"
	"io"
	"regexp"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	tiff "github.com/mutablelogic/go-metadata/internal/tiff"

	// Image decoders
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of an image, such as its
	// format and size
	Namespace = "image"

	// maxHeadSize is the most of the start of an image which is read for its
	// EXIF data, which is in the APP1 segment near the start of a JPEG image
	maxHeadSize = 128 << 10
)

var (
	reContentType = regexp.MustCompile(`^image/.+$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The kind, format and size of an image, and a hint of its artwork, when
	// its header is decoded, as a thumbnail can then be made of it
	metadata.AddHandler(reContentType, Namespace, func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		meta, err := imageMetadata(r)
		if len(meta) > 0 {
			meta = append(meta, metadata.HintMetadata(metadata.HintArtwork)...)
		}
		return append(meta, metadata.KindMetadata(dc.KindImage)...), err
	}, Namespace, metadata.MetadataNamespace, dc.Namespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// imageMetadata returns the format of an image, such as jpeg, and its width
// and height in pixels, as image:format, image:width and image:height, from
// its header, rather than decoding the image, which can be large. The width
// and height are as it's displayed, so they're swapped when the orientation
// in the EXIF data of a JPEG or TIFF image rotates it by 90°. An image in a
// format which isn't decoded, such as an SVG image, or whose header can't be
// decoded, such as a JPEG file of only its EXIF metadata, has none, which
// isn't an error, as another handler may read it.
func imageMetadata(r io.Reader) ([]metadata.Metadata, error) {
	// The start of the image, which has its EXIF data, and then the rest
	head := make([]byte, maxHeadSize)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	head = head[:n]
	config, format, err := stdimage.DecodeConfig(io.MultiReader(bytes.NewReader(head), r))
	if err != nil {
		return nil, nil
	}
	width, height := config.Width, config.Height
	if (format == "jpeg" || format == "tiff") && tiff.Orientation(head) >= 5 {
		width, height = height, width
	}
	return []metadata.Metadata{
		meta.New(Namespace, "format", format),
		meta.New(Namespace, "width", width),
		meta.New(Namespace, "height", height),
	}, nil
}
