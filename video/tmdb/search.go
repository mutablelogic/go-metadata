package tmdb

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	// Packages
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
	schema "github.com/mutablelogic/go-metadata/pkg/tmdb/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
	norm "golang.org/x/text/unicode/norm"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// searchCandidates is how many of a search's top results are checked
	searchCandidates = 5

	// runtimeTolerance is how far, in percent of the video's duration, a
	// movie's runtime can be from it, as a cut of a movie, or a video of it
	// with its credits removed, can be a little shorter or longer
	runtimeTolerance = 10

	// episodeTolerance is how far, in percent of the video's duration, an
	// episode's runtime can be from it, when no episode is within
	// runtimeTolerance, as TMDB's runtimes of episodes are often rounded,
	// and a broadcast can overrun, such as The Young Ones, whose episodes
	// are 34 minutes, which TMDB has as 30
	episodeTolerance = 25
)

// apostrophes are removed from a title, rather than separating words, so
// Schindler's List is Schindlers List, as a file's name often has it
var apostrophes = strings.NewReplacer("'", "", "’", "", "‘", "", "ʼ", "")

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// searchMovie returns the movie which the query is, or
// nothing when none matches (see matchMovie). With the query's year, it's
// searched for released in that year, and then, when none matches, in any
// year, but released within a year of it, as the year can be the movie's
// premiere, a year before its release, such as Sexy Beast, which premiered
// in 2000, and was released in 2001.
func searchMovie(ctx context.Context, client *tmdb.Client, q tmdb.Query, duration time.Duration) (*match, error) {
	// Search in the year
	if q.Year != 0 {
		req := schema.MovieSearchRequest{Term: q.Name, PrimaryReleaseYear: types.Ptr(strconv.Itoa(q.Year))}
		if result, err := matchMovie(ctx, client, req, q, duration, 0); err != nil || result != nil {
			return result, err
		}
	}

	// Search in any year, within a year of the query's
	return matchMovie(ctx, client, schema.MovieSearchRequest{Term: q.Name}, q, duration, 1)
}

// matchMovie returns the first of the search's top results
// whose title is the query's, and whose runtime is the video's duration,
// within runtimeTolerance, or nothing when none is. With the query's year,
// a result is released within years of it. TMDB's search can't search by
// runtime, and its results don't have one, so each candidate's runtime is
// from its details. A duration of zero isn't known, and matches any
// runtime, but a runtime which isn't known only matches a duration which
// isn't either.
func matchMovie(ctx context.Context, client *tmdb.Client, req schema.MovieSearchRequest, q tmdb.Query, duration time.Duration, years int) (*match, error) {
	resp, err := client.SearchMovies(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, movie := range resp.Results[:min(len(resp.Results), searchCandidates)] {
		if !titleMatches(q.Name, movie.Title, movie.OriginalTitle) || !yearMatches(movie.ReleaseDate, q.Year, years) {
			continue
		}
		details, err := client.GetMovie(ctx, movie.ID)
		if errors.Is(err, httpresponse.ErrNotFound) {
			continue
		} else if err != nil {
			return nil, err
		}
		// A movie whose runtime isn't known isn't the video, whose duration
		// is, as almost every film's runtime is known, and one whose isn't,
		// such as a short from 1919, would otherwise match any video
		runtime := time.Duration(details.Runtime) * time.Minute
		if duration > 0 && runtime <= 0 {
			continue
		} else if runtimeMatches(runtime, duration, runtimeTolerance) {
			return &match{movie: details}, nil
		}
	}

	// No movie matches
	return nil, nil
}

// searchShow returns the query's episode of a TV show, or the show, when
// the episode isn't known, or nothing when none matches
// (see matchShow). With the query's year, the show is searched for first
// aired in that year, and then, when none matches, in any year, as the year
// can be an episode's, rather than the year the show was first aired, such
// as for a later season.
func searchShow(ctx context.Context, client *tmdb.Client, q tmdb.Query, duration time.Duration) (*match, error) {
	// Search in the year
	if q.Year != 0 {
		req := schema.TVSearchRequest{Term: q.Name, FirstAirDateYear: types.Ptr(strconv.Itoa(q.Year))}
		if result, err := matchShow(ctx, client, req, q, duration); err != nil || result != nil {
			return result, err
		}
	}

	// Search in any year
	return matchShow(ctx, client, schema.TVSearchRequest{Term: q.Name}, q, duration)
}

// matchShow returns the query's episode of the first of the search's top
// results whose name is the query's, which has the episode, and whose
// episode's runtime is the video's duration, within runtimeTolerance, or
// the first such show, when the episode isn't
// known, whose typical runtime is the duration. When none's runtime is,
// it's the first such show whose runtime is within episodeTolerance, or
// else nothing. An episode's runtime is its show's typical one when it
// isn't known, and a duration or runtime of zero isn't known, and matches
// any.
func matchShow(ctx context.Context, client *tmdb.Client, req schema.TVSearchRequest, q tmdb.Query, duration time.Duration) (*match, error) {
	resp, err := client.SearchTVShows(ctx, req)
	if err != nil {
		return nil, err
	}
	var fallback *match
	for _, show := range resp.Results[:min(len(resp.Results), searchCandidates)] {
		if !titleMatches(q.Name, show.Name, show.OriginalName) {
			continue
		}

		// The show's episode, which it may not have, or none when the
		// episode isn't known
		episode := new(schema.TVEpisode)
		if q.Episode != 0 {
			if episode, err = client.GetTVEpisode(ctx, show.ID, uint64(q.Season), uint64(q.Episode)); errors.Is(err, httpresponse.ErrNotFound) {
				continue
			} else if err != nil {
				return nil, err
			}
		}

		// The show's details, for the episode's runtime when it isn't known
		details, err := client.GetTVShow(ctx, show.ID)
		if errors.Is(err, httpresponse.ErrNotFound) {
			continue
		} else if err != nil {
			return nil, err
		}
		result := &match{show: details, episode: episode}
		if q.Episode == 0 {
			result.episode = nil
		}
		runtime := episodeRuntime(details, episode)
		if runtimeMatches(runtime, duration, runtimeTolerance) {
			return result, nil
		} else if fallback == nil && runtimeMatches(runtime, duration, episodeTolerance) {
			fallback = result
		}
	}

	// The first show or episode within episodeTolerance, or nothing
	return fallback, nil
}

// titleMatches returns true when any of the titles, such as a movie's title
// and its original title, is the name, ignoring case, punctuation, and a
// leading "the" (see normalizeTitle)
func titleMatches(name string, titles ...string) bool {
	name = normalizeTitle(name)
	if name == "" {
		return false
	}
	for _, title := range titles {
		if normalizeTitle(title) == name {
			return true
		}
	}
	return false
}

// normalizeTitle returns the words of a title, in lower case, separated by
// a space, without accents, apostrophes or other punctuation, or a leading
// "the", which a file's name often leaves out, so The Hunt for Red October
// is hunt for red october, Mission: Impossible is mission impossible, and
// Ascenseur pour l'échafaud is ascenseur pour lechafaud. Other articles
// aren't removed, as they're part of other titles, such as A Beautiful
// Mind.
func normalizeTitle(title string) string {
	title = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, norm.NFD.String(title))
	words := strings.FieldsFunc(apostrophes.Replace(strings.ToLower(title)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) > 1 && words[0] == "the" {
		words = words[1:]
	}
	return strings.Join(words, " ")
}

// yearMatches returns true when the date, such as 2001-01-12, is within
// years of the year, or the year isn't known
func yearMatches(date string, year, years int) bool {
	if year == 0 {
		return true
	}
	released, err := strconv.Atoi(date[:min(4, len(date))])
	return err == nil && released >= year-years && released <= year+years
}

// runtimeMatches returns true when the runtime is within the tolerance, in
// percent, of the duration, or either isn't known
func runtimeMatches(runtime, duration time.Duration, tolerance int) bool {
	if runtime <= 0 || duration <= 0 {
		return true
	}
	diff := duration - runtime
	if diff < 0 {
		diff = -diff
	}
	return diff*100 <= duration*time.Duration(tolerance)
}
