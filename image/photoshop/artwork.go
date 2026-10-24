package photoshop

import (
	"bytes"
	"context"
	"image"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	psd "github.com/oov/psd"

	// Image decoders
	_ "image/jpeg"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// thumbnailKey is the key of the artwork of a document, which is the
	// thumbnail embedded in it, in place of the composite image
	thumbnailKey = "thumbnail"

	// artworkPriority is higher than the default, so the thumbnail which is
	// embedded is the artwork, rather than the composite image, which the
	// raster images' handler returns
	artworkPriority = metadata.DefaultPriority + 10

	// thumbnailHeaderSize is the size of the header of the thumbnail
	// resource, before its JPEG image
	thumbnailHeaderSize = 28
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The thumbnail which is embedded, as artwork, only when artwork is
	// requested
	metadata.AddHandlerWithPriority(reContentType, "photoshop-artwork", artworkPriority, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		cfg, _, err := psd.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, nil
		}
		if thumbnail := thumbnailArtwork(cfg, o); thumbnail != nil {
			return []metadata.Metadata{thumbnail}, nil
		}

		// The composite image of a document of an application/ content
		// type, which the raster images' handler doesn't read
		if !isImage(o.ContentType()) {
			if composite := artwork.Image(data, thumbnailKey, o); composite != nil {
				return []metadata.Metadata{composite}, nil
			}
		}
		return nil, nil
	}, metadata.ArtworkNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// thumbnailArtwork returns the thumbnail embedded in the document as
// artwork:thumbnail, or nil when there isn't one, or it isn't a JPEG image
// which can be decoded, so the composite image is the artwork
func thumbnailArtwork(cfg psd.Config, o *metadata.Opts) metadata.Metadata {
	res, exists := cfg.Res[resourceThumbnail]
	if !exists || len(res.Data) <= thumbnailHeaderSize {
		return nil
	}
	data := res.Data[thumbnailHeaderSize:]
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return nil
	}
	return artwork.Image(data, thumbnailKey, o)
}
