//go:build cgo

package exif

import (
	"bytes"
	"context"
	"image"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	exif "github.com/mutablelogic/go-metadata/pkg/exif"

	// Image decoders
	_ "image/jpeg"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// thumbnailKey is the key of the artwork of a photo, which is the
	// thumbnail embedded in its EXIF metadata, in place of the photo itself
	thumbnailKey = "thumbnail"

	// artworkPriority is higher than the default, so the thumbnail which is
	// embedded is the artwork, rather than the photo itself
	artworkPriority = metadata.DefaultPriority + 10
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The thumbnail which is embedded, as artwork, only when artwork is
	// requested
	metadata.AddHandlerWithPriority(reContentType, EXIFNamespace+"-artwork", artworkPriority, func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		f, err := exif.Read(r)
		if err != nil {
			return nil, nil
		}
		defer f.Close()
		if thumbnail := thumbnailArtwork(f.Thumbnail(), f.Orientation(), o); thumbnail != nil {
			return []metadata.Metadata{thumbnail}, nil
		}
		return nil, nil
	}, metadata.ArtworkNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// thumbnailArtwork returns the thumbnail as artwork:thumbnail, transformed
// by the orientation of the photo, as it has no EXIF data of its own, or nil
// when there isn't one, or it isn't an image, such as when it's cut short,
// so the photo itself is the artwork
func thumbnailArtwork(data []byte, orientation int, o *metadata.Opts) metadata.Metadata {
	if len(data) == 0 {
		return nil
	} else if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return nil
	}
	return artwork.OrientedImage(data, thumbnailKey, orientation, o)
}
