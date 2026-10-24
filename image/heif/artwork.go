//go:build cgo

package heif

import (
	"context"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	heif "github.com/mutablelogic/go-metadata/pkg/heif"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// thumbnailKey is the key of the artwork of an image, which is the
	// thumbnail embedded in it, in place of the image itself
	thumbnailKey = "thumbnail"

	// artworkPriority is higher than the default, so the thumbnail which is
	// embedded is the artwork, rather than the image itself, which the
	// raster images' handler returns
	artworkPriority = metadata.DefaultPriority + 10
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The thumbnail which is embedded, as artwork, only when artwork is
	// requested
	metadata.AddHandlerWithPriority(reContentType, "heif-artwork", artworkPriority, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		h, err := heif.Read(r)
		if err != nil {
			return nil, nil
		}
		defer h.Close()
		for _, thumbnail := range h.Thumbnails() {
			if data := artwork.Encode(artwork.Downscale(thumbnail, o.MaxImageDimension())); data != nil {
				return []metadata.Metadata{meta.NewImage(metadata.ArtworkNamespace, thumbnailKey, data)}, nil
			}
		}
		return nil, nil
	}, metadata.ArtworkNamespace)
}
