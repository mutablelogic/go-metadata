package tmdb

import (
	"context"
	"errors"
	"strconv"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
	schema "github.com/mutablelogic/go-metadata/pkg/tmdb/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// getMovie returns the movie with the identifier, or nothing when TMDB
// doesn't have it
func getMovie(ctx context.Context, client *tmdb.Client, id uint64) (*match, error) {
	details, err := client.GetMovie(ctx, id)
	if errors.Is(err, httpresponse.ErrNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &match{movie: details}, nil
}

// movieMetadata returns the metadata of the movie: its kind, a film, its
// identifier, title, which is its dc:title too, release date and overview, which are its
// dc:date and dc:description too, runtime and IMDb identifier, which is its
// dc:identifier too, when they're known
func movieMetadata(movie *schema.MovieDetails) []metadata.Metadata {
	result := append(metadata.KindMetadata(dc.KindFilm),
		meta.New(Namespace, "id", strconv.FormatUint(movie.ID, 10)),
		meta.New(Namespace, "title", movie.Title),
		meta.New(dc.Namespace, dc.Title, movie.Title),
	)
	if movie.ReleaseDate != "" {
		result = append(result,
			meta.New(Namespace, "release_date", movie.ReleaseDate),
			meta.New(dc.Namespace, dc.Date, movie.ReleaseDate),
		)
	}
	if movie.Overview != "" {
		result = append(result,
			meta.New(Namespace, "overview", movie.Overview),
			meta.New(dc.Namespace, dc.Description, movie.Overview),
		)
	}
	if movie.Runtime > 0 {
		result = append(result, meta.New(Namespace, "runtime", time.Duration(movie.Runtime)*time.Minute))
	}
	if movie.IMDbID != "" {
		result = append(result,
			meta.New(Namespace, "imdb_id", movie.IMDbID),
			meta.New(dc.Namespace, dc.Identifier, movie.IMDbID),
		)
	}
	return result
}
