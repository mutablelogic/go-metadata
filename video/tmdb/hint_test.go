package tmdb

import (
	"slices"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	schema "github.com/mutablelogic/go-metadata/pkg/tmdb/schema"
)

func Test_hint_000(t *testing.T) {
	// A match's hints: its artwork, when it has an image, and its
	// collection, when it's a show's
	for _, test := range []struct {
		name  string
		m     match
		hints []string
	}{
		{"movie", match{movie: &schema.MovieDetails{}}, nil},
		{"movie with a poster", match{movie: &schema.MovieDetails{}}, []string{metadata.HintArtwork}},
		{"show", match{show: &schema.TVShowDetails{}}, []string{metadata.HintCollection}},
		{"episode with a still", match{show: &schema.TVShowDetails{}, episode: &schema.TVEpisode{}}, []string{metadata.HintArtwork, metadata.HintCollection}},
	} {
		switch test.name {
		case "movie with a poster":
			test.m.movie.PosterPath = "/poster.jpg"
		case "episode with a still":
			test.m.episode.StillPath = "/still.jpg"
		}
		if got := test.m.hints(); !slices.Equal(got, test.hints) {
			t.Errorf("%s: expected %q, got %q", test.name, test.hints, got)
		}
	}
}
