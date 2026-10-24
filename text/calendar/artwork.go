package calendar

import (
	"context"
	"io"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// imageKey is the key of the image of a calendar, as artwork
	imageKey = "image"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The image of the calendar, or of its first event which has one, as
	// artwork, only when artwork is requested. An image which is linked,
	// rather than embedded, isn't fetched.
	// Reference: https://www.rfc-editor.org/rfc/rfc7986#section-5.10
	metadata.AddHandler(reContentType, "ical-artwork", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		c, err := ical.Decode(io.LimitReader(r, maxSize))
		if err != nil {
			return nil, err
		}
		images := []*ical.Image{c.Image}
		for _, e := range c.Events {
			images = append(images, e.Image)
		}
		for _, image := range images {
			if image == nil || len(image.Data) == 0 {
				continue
			}
			if entry := artwork.Image(image.Data, imageKey, o); entry != nil {
				return []metadata.Metadata{entry}, nil
			}
		}
		return nil, nil
	}, metadata.ArtworkNamespace)
}
