package vcard

import (
	"context"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	vcf "github.com/mutablelogic/go-metadata/pkg/vcard"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// photoKey is the key of the photo of a contact, as artwork
	photoKey = "photo"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The photo of the first contact, as artwork, only when artwork is
	// requested. A photo which is linked, rather than embedded, isn't read.
	metadata.AddHandler(reContentType, "vcard-artwork", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		cards, err := vcf.Decode(io.LimitReader(r, maxSize))
		if err != nil || len(cards) == 0 || cards[0].Photo == nil || len(cards[0].Photo.Data) == 0 {
			return nil, err
		}
		if photo := artwork.Image(cards[0].Photo.Data, photoKey, o); photo != nil {
			return []metadata.Metadata{photo}, nil
		}
		return nil, nil
	}, metadata.ArtworkNamespace)
}
