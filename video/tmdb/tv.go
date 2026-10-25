package tmdb

import (
	"context"
	"errors"
	"fmt"
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

// getEpisode returns the episode of the show with the identifier, by its
// season and episode numbers, or nothing when TMDB doesn't have it
func getEpisode(ctx context.Context, client *tmdb.Client, id, season, number uint64) (*match, error) {
	show, err := client.GetTVShow(ctx, id)
	if errors.Is(err, httpresponse.ErrNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	episode, err := client.GetTVEpisode(ctx, id, season, number)
	if errors.Is(err, httpresponse.ErrNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &match{show: show, episode: episode}, nil
}

// episodeMetadata returns the metadata of the show's episode: its kind, an
// episode, its identifier, and its show's, the show's name, its season and
// episode numbers, title, air date and overview, which are its dc:date and
// dc:description too, runtime, which is the show's typical one when the
// episode's isn't known, and IMDb identifier, which is its dc:identifier
// too, when they're known, and its dc:title (see episodeTitle)
func episodeMetadata(show *schema.TVShowDetails, episode *schema.TVEpisode) []metadata.Metadata {
	title := episodeTitle(show, episode)
	result := append(metadata.KindMetadata(dc.KindEpisode),
		meta.New(Namespace, "id", strconv.FormatUint(episode.ID, 10)),
		meta.New(Namespace, "showid", strconv.FormatUint(show.ID, 10)),
		meta.New(Namespace, "show", show.Name),
		meta.New(Namespace, "season", episode.SeasonNumber),
		meta.New(Namespace, "episode", episode.EpisodeNumber),
		meta.New(dc.Namespace, dc.Title, title),
	)
	if episode.Name != "" {
		result = append(result, meta.New(Namespace, "title", episode.Name))
	}
	if episode.AirDate != "" {
		result = append(result,
			meta.New(Namespace, "date-release", episode.AirDate),
			meta.New(dc.Namespace, dc.Date, episode.AirDate),
		)
	}
	if episode.Overview != "" {
		result = append(result,
			meta.New(Namespace, "overview", episode.Overview),
			meta.New(dc.Namespace, dc.Description, episode.Overview),
		)
	}
	if runtime := episodeRuntime(show, episode); runtime > 0 {
		result = append(result, meta.New(Namespace, "runtime", runtime))
	}
	if episode.ExternalIDs != nil && episode.ExternalIDs.IMDbID != "" {
		result = append(result,
			meta.New(Namespace, "imdbid", episode.ExternalIDs.IMDbID),
			meta.New(dc.Namespace, dc.Identifier, episode.ExternalIDs.IMDbID),
		)
	}
	return result
}

// showMetadata returns the metadata of the show's video, whose episode
// isn't known: its kind, an episode, the show's identifier, name, which is
// its dc:title too, the date it was first aired and its overview, which are
// its dc:date and dc:description too, and its IMDb identifier, which is its
// dc:identifier too, when they're known
func showMetadata(show *schema.TVShowDetails) []metadata.Metadata {
	result := append(metadata.KindMetadata(dc.KindEpisode),
		meta.New(Namespace, "showid", strconv.FormatUint(show.ID, 10)),
		meta.New(Namespace, "show", show.Name),
		meta.New(dc.Namespace, dc.Title, show.Name),
	)
	if show.FirstAirDate != "" {
		result = append(result,
			meta.New(Namespace, "date-release", show.FirstAirDate),
			meta.New(dc.Namespace, dc.Date, show.FirstAirDate),
		)
	}
	if show.Overview != "" {
		result = append(result,
			meta.New(Namespace, "overview", show.Overview),
			meta.New(dc.Namespace, dc.Description, show.Overview),
		)
	}
	if show.ExternalIDs != nil && show.ExternalIDs.IMDbID != "" {
		result = append(result,
			meta.New(Namespace, "imdbid", show.ExternalIDs.IMDbID),
			meta.New(dc.Namespace, dc.Identifier, show.ExternalIDs.IMDbID),
		)
	}
	return result
}

// episodeTitle returns the title of the show's episode: the show's name,
// its season, and the episode's title, or its number when it has none, such
// as "Seinfeld – Season 1 – The Stake Out". Season zero is a show's
// specials.
func episodeTitle(show *schema.TVShowDetails, episode *schema.TVEpisode) string {
	season := fmt.Sprintf("Season %d", episode.SeasonNumber)
	if episode.SeasonNumber == 0 {
		season = "Specials"
	}
	name := episode.Name
	if name == "" {
		name = fmt.Sprintf("Episode %d", episode.EpisodeNumber)
	}
	return show.Name + " – " + season + " – " + name
}

// episodeRuntime returns the runtime of the show's episode, which is the
// show's typical one when the episode's isn't known, or zero when neither is
func episodeRuntime(show *schema.TVShowDetails, episode *schema.TVEpisode) time.Duration {
	runtime := episode.Runtime
	if runtime == 0 && len(show.EpisodeRunTime) > 0 {
		runtime = show.EpisodeRunTime[0]
	}
	return time.Duration(runtime) * time.Minute
}
