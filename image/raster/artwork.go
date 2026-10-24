package raster

import (
	"context"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// thumbnailKey is the key of the artwork of an image, which is the image
	thumbnailKey = "thumbnail"

	// maxImageSize is the largest image which is read, as artwork, in bytes
	maxImageSize = 64 << 20
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The image itself, as artwork, only when artwork is requested
	metadata.AddHandler(reContentType, Namespace+"-artwork", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		return imageArtwork(r, o)
	}, metadata.ArtworkNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// imageArtwork returns the image as artwork:thumbnail, downscaled to fit the
// largest image dimension, and in a format which is displayed on the web, or
// nothing when it's in a format which isn't decoded, such as an SVG image,
// or it's larger than maxImageSize
func imageArtwork(r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxImageSize+1))
	if err != nil {
		return nil, err
	} else if len(data) > maxImageSize {
		return nil, nil
	}
	if thumbnail := artwork.Image(data, thumbnailKey, o); thumbnail != nil {
		return []metadata.Metadata{thumbnail}, nil
	}
	return nil, nil
}
