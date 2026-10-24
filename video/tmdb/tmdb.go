// Package tmdb registers a handler of videos which looks up the movie or TV
// episode which a video is on TMDB, when a TMDB client is set (see
// metadata.WithTMDB): by its identifier, when it's set, and otherwise by
// searching with the video's title, and its year, season and episode when
// they're known, from its tags, such as the show of an episode from iTunes,
// and otherwise from its name (see tmdb.ExtractQuery).
package tmdb

import (
	"context"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
	schema "github.com/mutablelogic/go-metadata/pkg/tmdb/schema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// match is the movie, or the TV show and its episode, on TMDB which a video
// is. The episode is nil for a show's video whose episode isn't known.
type match struct {
	movie   *schema.MovieDetails
	show    *schema.TVShowDetails
	episode *schema.TVEpisode
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the metadata of a video on TMDB
	Namespace = "tmdb"

	// minDuration is the shortest a video can be to be looked up, as a
	// shorter one is a trailer or other extra, rather than a movie or an
	// episode
	minDuration = 5 * time.Minute

	// videoNamespace is the namespace of a video's duration and tags, such
	// as its show, season and episode, which the handler of videos returns,
	// when it's built with cgo
	videoNamespace = "video"

	// priority is the priority of the handler, so its title, from TMDB,
	// is in place of a video's own title, from its tags
	priority = 10
)

var (
	reContentType = regexp.MustCompile(`^video/.*$`)

	// excludedNameRe matches the name of a video which isn't a movie or an
	// episode, such as a trailer or a sample
	excludedNameRe = regexp.MustCompile(`(?i)(?:^|[\s._-])(?:sample|trailer|teaser|featurette|deleted[\s._-]scenes|behind[\s._-]the[\s._-]scenes)(?:[\s._-]|$)`)

	// deviceNameRe matches the names of videos from cameras, phones and
	// DVDs, without their extensions, which are numbered or dated, rather
	// than named for what's in them: a prefix and a number, such as
	// MOV00062, MVI_1234, DSC_0001, GOPR0001, GX010001, DJI_0001, C0001,
	// P1000123, IMG_1234, VID_20230101_123456, VID-20230101-WA0001 or
	// PXL_20230101_123456; a date and time, such as 20230101_123456; a
	// number of five digits or more, such as 00001, so a film named for a
	// year, such as 1917, isn't matched; a screen recording; or a DVD's
	// video, such as VIDEO_TS or VTS_01_1, which is a piece of a film
	deviceNameRe = regexp.MustCompile(`(?i)^(?:(?:mov|mvi|img|vid|dsc[fn]?|gopr|g[hpx]\d\d|dji|mah|m2u|clip|pxl|c|p)[_-]?\d{3,}.*|\d{8}[_-]\d{6}.*|\d{5,}|(?:screen[ _]?recording|rpreplay).*|video_ts|vts_\d+_\d+)$`)

	// excludedFolderRe matches the name of a folder of videos which aren't
	// movies or episodes, such as a movie's extras
	excludedFolderRe = regexp.MustCompile(`(?i)^(?:extras|featurettes|samples|trailers|bonus(?:[\s._-]features)?|deleted[\s._-]scenes|behind[\s._-]the[\s._-]scenes)$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The movie or episode on TMDB which a video is, when there's a TMDB
	// client: by its identifier, when it's set (see metadata.WithTMDBMovie
	// and metadata.WithTMDBEpisode), and otherwise searched for from the
	// video's tags and name. It runs after the handler of videos, for the
	// video's duration and tags, and its title, as dc:title, is in place of the
	// video's own.
	metadata.AddHandlerAfterWithPriority(reContentType, "tmdb", priority, []string{videoNamespace}, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		// Skip if there's no TMDB client available
		client := o.TMDB()
		if client == nil {
			return nil, nil
		}

		// Get the movie or the episode by its identifier, when it's set, or
		// else search for it, from the video's tags and name
		var m *match
		var err error
		if id := o.TMDBMovie(); id != 0 {
			m, err = getMovie(ctx, client, id)
		} else if show, season, episode := o.TMDBEpisode(); show != 0 {
			m, err = getEpisode(ctx, client, show, season, episode)
		} else {
			var name string
			if named, ok := r.(metadata.NamedReader); ok {
				name = named.Name()
			}
			m, err = search(ctx, client, name, o)
		}
		if err != nil || m == nil {
			return nil, err
		}

		// Return its metadata, and its artwork and Markdown, when they're
		// requested
		return m.result(ctx, client, o)
	}, Namespace, dc.Namespace, metadata.MetadataNamespace, metadata.ArtworkNamespace, metadata.MarkdownNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// search returns the movie or episode which the video with the name is, from its tags and name, or nothing when it isn't a movie or
// an episode, or none matches. A TV show's video is an episode, or the show,
// when its episode isn't known, and otherwise it's a movie.
func search(ctx context.Context, client *tmdb.Client, name string, o *metadata.Opts) (*match, error) {
	// Exclude TMDB search for videos which aren't movies or episodes
	duration := duration(o)
	if exclude(name, duration) {
		return nil, nil
	}

	// Search for the show or its episode, or the movie, with each query in
	// turn, until one matches
	for _, q := range queries(name, o) {
		var result *match
		var err error
		switch {
		case q.Name == "":
			continue
		case q.TV:
			result, err = searchShow(ctx, client, q, duration)
		default:
			result, err = searchMovie(ctx, client, q, duration)
		}
		if err != nil || result != nil {
			return result, err
		}
	}

	// No movie, show or episode matches
	return nil, nil
}

// queries returns the queries for the video with the name: its query (see
// query), and then, when its tags are of an episode which isn't its name's,
// the tagged show's episode which its name has, as a tag can number an
// episode in the whole show, rather than in its season, such as episode 76
// of Seinfeld, which is the 12th of its fifth season
func queries(name string, o *metadata.Opts) []tmdb.Query {
	q, named := query(name, o), tmdb.ExtractQuery(name)
	if !q.TV || q.Episode == 0 || named.Episode == 0 || (q.Season == named.Season && q.Episode == named.Episode) {
		return []tmdb.Query{q}
	}
	alt := q
	alt.Season, alt.Episode = named.Season, named.Episode
	return []tmdb.Query{q, alt}
}

// exclude returns true when the video at the path isn't a movie or an
// episode, so isn't looked up: when its name is of an extra, such as a
// trailer, or one a camera, phone or DVD gives a video, rather than what's
// in it, or it's a DVD's video, whose extension is VOB, which is a piece of
// a film, or it's in a folder of extras, or it's shorter than minDuration.
// A duration of zero isn't known.
func exclude(path string, duration time.Duration) bool {
	if duration > 0 && duration < minDuration {
		return true
	}
	ext := filepath.Ext(path)
	name := strings.TrimSuffix(filepath.Base(path), ext)
	switch {
	case strings.EqualFold(ext, ".vob"):
		return true
	case excludedNameRe.MatchString(name), deviceNameRe.MatchString(name):
		return true
	}
	return excludedFolderRe.MatchString(filepath.Base(filepath.Dir(path)))
}

// duration returns the duration of the video, from the handler of videos,
// or zero when it isn't known, such as when it's built without cgo
func duration(o *metadata.Opts) time.Duration {
	if m := o.Get(videoNamespace, "duration"); m != nil {
		if d, ok := m.Any().(time.Duration); ok {
			return d
		}
	}
	return 0
}

// query returns the query for the video with the name, whose tags, from
// the handler of videos, are in preference to its name. An episode's tags
// are its show, season and episode, such as iTunes has, and a movie's is its
// year. An episode's date isn't its show's year, which the year is. A
// tagged episode can have its season before it, such as 206, which is the
// sixth episode of the second season.
func query(name string, o *metadata.Opts) tmdb.Query {
	q := tmdb.ExtractQuery(name)
	if show := tag(o, "show"); show != "" {
		q.Name, q.TV = show, true
		if season := number(tag(o, "season-number")); season > 0 {
			q.Season = season
		}
		if episode := number(tag(o, "episode-sort")); episode > 0 {
			q.Episode = episode
		}
		if q.Season > 0 && q.Episode >= 100 && q.Episode/100 == q.Season {
			q.Episode %= 100
		}
		switch {
		case q.Episode == 0:
			q.Season = 0
		case q.Season == 0:
			q.Season = 1
		}
	} else if year := year(tag(o, "date")); year != 0 && q.Episode == 0 {
		q.Year = year
	}
	return q
}

// tag returns the value of the video's tag, from the handler of videos, or
// an empty string when it doesn't have it
func tag(o *metadata.Opts, key string) string {
	if m := o.Get(videoNamespace, key); m != nil {
		return strings.TrimSpace(m.Value())
	}
	return ""
}

// number returns the value as a number, or zero when it isn't one
func number(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}

// year returns the year of a date, such as 2000 or 2000-05-01, or zero when
// it doesn't start with one
func year(date string) int {
	if len(date) < 4 {
		return 0
	} else if year := number(date[:4]); year >= 1800 {
		return year
	}
	return 0
}
