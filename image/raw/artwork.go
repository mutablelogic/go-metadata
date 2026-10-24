//go:build cgo

package raw

import (
	"context"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	tiff "github.com/mutablelogic/go-metadata/internal/tiff"
	raw "github.com/mutablelogic/go-metadata/pkg/raw"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// thumbnailKey is the key of the artwork of a RAW image, which is the
	// preview embedded in it
	thumbnailKey = "thumbnail"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The preview which is embedded, as artwork, only when artwork is
	// requested
	metadata.AddHandlerWithPriority(raw.ContentTypes, "raw-artwork", priority, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		img, err := raw.Parse(data)
		if err != nil {
			return nil, nil
		}
		defer img.Close()
		if preview := previewArtwork(img, tiff.Orientation(tiff.Normalize(data)), o); preview != nil {
			return []metadata.Metadata{preview}, nil
		}
		return nil, nil
	}, metadata.ArtworkNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// previewArtwork returns the preview embedded in the RAW image, as
// artwork:thumbnail, transformed by the orientation of the image, unless the
// preview has an orientation of its own, or nil when there isn't one. A
// preview which is a JPEG image is kept as it is when it's small enough and
// isn't transformed, and one which is a bitmap is encoded.
func previewArtwork(img *raw.RAW, orientation int, o *metadata.Opts) metadata.Metadata {
	data, err := img.ThumbnailBytes()
	if err != nil {
		return nil
	}
	if own := tiff.Orientation(data); own != 1 {
		orientation = own
	}
	if entry := artwork.OrientedImage(data, thumbnailKey, orientation, o); entry != nil {
		return entry
	}

	// A preview which is a bitmap, rather than an encoded image
	preview, err := img.Thumbnail()
	if err != nil || preview == nil {
		return nil
	}
	data = artwork.Encode(artwork.Orient(artwork.Downscale(preview, o.MaxImageDimension()), orientation))
	if data == nil {
		return nil
	}
	return meta.NewImage(metadata.ArtworkNamespace, thumbnailKey, data)
}
