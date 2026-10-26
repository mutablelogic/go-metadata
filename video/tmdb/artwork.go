package tmdb

import (
	"context"
	"slices"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// tmdbImage is an image of a movie, show or episode on TMDB: its key as
// artwork, its size, and its path, which is empty when there's none
type tmdbImage struct {
	key, size, path string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The keys of the images of a movie, show or episode, as artwork
const (
	posterKey   = "poster"
	backdropKey = "backdrop"
	stillKey    = "still"
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// images returns the images of the movie, or of the show, and the episode's
// still, in the smallest sizes whose largest dimension is at least the
// dimension
func (m *match) images(dimension int) []tmdbImage {
	if m.movie != nil {
		return []tmdbImage{
			{posterKey, tmdb.PosterSize(dimension), m.movie.PosterPath},
			{backdropKey, tmdb.BackdropSize(dimension), m.movie.BackdropPath},
		}
	}
	var images []tmdbImage
	if m.episode != nil {
		images = append(images, tmdbImage{stillKey, tmdb.StillSize(dimension), m.episode.StillPath})
	}
	return append(images,
		tmdbImage{posterKey, tmdb.PosterSize(dimension), m.show.PosterPath},
		tmdbImage{backdropKey, tmdb.BackdropSize(dimension), m.show.BackdropPath},
	)
}

// hints returns the hints of the movie, show or episode: its artwork, when
// it has an image, and its collection, when it's a show's, or an episode of
// one, which aren't downloaded
func (m *match) hints() []string {
	var hints []string
	if slices.ContainsFunc(m.images(0), func(image tmdbImage) bool { return image.path != "" }) {
		hints = append(hints, metadata.HintArtwork)
	}
	if m.show != nil {
		hints = append(hints, metadata.HintCollection)
	}
	return hints
}

// artwork returns the images of the movie, show or episode, downloaded
// from TMDB, as artwork, downscaled to fit the largest image dimension (see
// metadata.WithMaxImageDimension). An image which can't be downloaded or
// decoded isn't returned, unless the context is done.
func (m *match) artwork(ctx context.Context, client *tmdb.Client, o *metadata.Opts) ([]metadata.Metadata, error) {
	var result []metadata.Metadata
	for _, image := range m.images(o.MaxImageDimension()) {
		if image.path == "" {
			continue
		}
		data, err := client.Image(ctx, image.size, image.path)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if a := artwork.Image(data, image.key, o); a != nil {
			result = append(result, a)
		}
	}
	return result, nil
}
