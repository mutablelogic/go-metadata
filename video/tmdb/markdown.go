package tmdb

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	imarkdown "github.com/mutablelogic/go-metadata/internal/markdown"
	itext "github.com/mutablelogic/go-metadata/internal/text"
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// The URLs of a movie, show and episode on TMDB, and of a title on IMDb
	tmdbMovieURL   = "https://www.themoviedb.org/movie/%d"
	tmdbShowURL    = "https://www.themoviedb.org/tv/%d"
	tmdbEpisodeURL = "https://www.themoviedb.org/tv/%d/season/%d/episode/%d"
	imdbURL        = "https://www.imdb.com/title/%s/"
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// metadata returns the metadata of the movie, episode or show
func (m *match) metadata() []metadata.Metadata {
	switch {
	case m.movie != nil:
		return movieMetadata(m.movie)
	case m.episode != nil:
		return episodeMetadata(m.show, m.episode)
	default:
		return showMetadata(m.show)
	}
}

// result returns the metadata of the movie, episode or show, its images,
// from TMDB, only when the artwork namespace is requested, and its
// Markdown, only when the markdown namespace is requested
func (m *match) result(ctx context.Context, client *tmdb.Client, o *metadata.Opts) ([]metadata.Metadata, error) {
	result := append(m.metadata(), metadata.HintMetadata(m.hints()...)...)
	if o.HasNamespace(metadata.ArtworkNamespace) {
		images, err := m.artwork(ctx, client, o)
		if err != nil {
			return nil, err
		}
		result = append(result, images...)
	}
	if o.HasNamespace(metadata.MarkdownNamespace) {
		w := imarkdown.NewWriter(o)
		markdown, err := w.Metadata(m.writeMarkdown(w))
		return append(result, markdown...), err
	}
	return result, nil
}

// writeMarkdown writes the movie, episode or show as Markdown: its title as
// a heading, its details as a list, such as its show, season, episode, when
// it was released, its runtime, and links to it on TMDB and IMDb, and its
// overview as paragraphs
func (m *match) writeMarkdown(w io.StringWriter) error {
	var title, overview string
	var items []string
	item := func(label, value string) {
		if value != "" {
			items = append(items, "- **"+label+":** "+value)
		}
	}
	link := func(label, url string) string {
		return "[" + itext.Escape(label) + "](" + url + ")"
	}

	switch {
	case m.movie != nil:
		title, overview = m.movie.Title, m.movie.Overview
		if m.movie.OriginalTitle != m.movie.Title {
			item("Original title", itext.Escape(m.movie.OriginalTitle))
		}
		item("Released", itext.Escape(m.movie.ReleaseDate))
		item("Runtime", runtimeText(time.Duration(m.movie.Runtime)*time.Minute))
		item("TMDB", link(fmt.Sprint(m.movie.ID), fmt.Sprintf(tmdbMovieURL, m.movie.ID)))
		if m.movie.IMDbID != "" {
			item("IMDb", link(m.movie.IMDbID, fmt.Sprintf(imdbURL, m.movie.IMDbID)))
		}
	case m.episode != nil:
		title, overview = episodeTitle(m.show, m.episode), m.episode.Overview
		item("Show", itext.Escape(m.show.Name))
		item("Season", fmt.Sprint(m.episode.SeasonNumber))
		item("Episode", fmt.Sprint(m.episode.EpisodeNumber))
		if m.episode.Name != "" {
			item("Title", itext.Escape(m.episode.Name))
		}
		item("Aired", itext.Escape(m.episode.AirDate))
		item("Runtime", runtimeText(episodeRuntime(m.show, m.episode)))
		item("TMDB", link(fmt.Sprint(m.episode.ID), fmt.Sprintf(tmdbEpisodeURL, m.show.ID, m.episode.SeasonNumber, m.episode.EpisodeNumber)))
		if ids := m.episode.ExternalIDs; ids != nil && ids.IMDbID != "" {
			item("IMDb", link(ids.IMDbID, fmt.Sprintf(imdbURL, ids.IMDbID)))
		}
	default:
		title, overview = m.show.Name, m.show.Overview
		item("First aired", itext.Escape(m.show.FirstAirDate))
		if m.show.NumberOfSeasons > 0 {
			item("Seasons", fmt.Sprint(m.show.NumberOfSeasons))
		}
		item("TMDB", link(fmt.Sprint(m.show.ID), fmt.Sprintf(tmdbShowURL, m.show.ID)))
		if ids := m.show.ExternalIDs; ids != nil && ids.IMDbID != "" {
			item("IMDb", link(ids.IMDbID, fmt.Sprintf(imdbURL, ids.IMDbID)))
		}
	}

	// The title, details and overview
	var b strings.Builder
	b.WriteString("# " + itext.Escape(title) + "\n\n")
	if len(items) > 0 {
		b.WriteString(strings.Join(items, "\n") + "\n")
	}
	for _, paragraph := range strings.Split(strings.ReplaceAll(overview, "\r\n", "\n"), "\n\n") {
		if paragraph = strings.TrimSpace(paragraph); paragraph != "" {
			b.WriteString("\n" + itext.Escape(paragraph) + "\n")
		}
	}
	_, err := w.WriteString(b.String())
	return err
}

// runtimeText returns a runtime as text, such as "2h 16m" or "47m", or an
// empty string when it isn't known
func runtimeText(runtime time.Duration) string {
	minutes := int(runtime / time.Minute)
	switch {
	case minutes <= 0:
		return ""
	case minutes < 60:
		return fmt.Sprintf("%dm", minutes)
	case minutes%60 == 0:
		return fmt.Sprintf("%dh", minutes/60)
	default:
		return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
	}
}
