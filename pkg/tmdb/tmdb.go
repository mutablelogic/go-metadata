package tmdb

import (
	"context"
	"net/url"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-metadata/pkg/tmdb/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// withExternalIDs requests a show's or episode's identifiers in other
// databases, such as IMDb, with its details, rather than separately
var withExternalIDs = url.Values{"append_to_response": []string{"external_ids"}}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SEARCH

// SearchMovies returns a page of the movies whose titles match the search,
// with the most relevant first
func (c *Client) SearchMovies(ctx context.Context, req schema.MovieSearchRequest) (*schema.MovieSearchResponse, error) {
	var result schema.MovieSearchResponse
	if err := c.DoWithContext(ctx, nil, &result, client.OptPath("search", "movie"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return types.Ptr(result), nil
}

// SearchTVShows returns a page of the TV shows whose names match the search,
// with the most relevant first. TMDB searches shows, rather than episodes,
// which are got from a show with GetTVEpisode.
func (c *Client) SearchTVShows(ctx context.Context, req schema.TVSearchRequest) (*schema.TVSearchResponse, error) {
	var result schema.TVSearchResponse
	if err := c.DoWithContext(ctx, nil, &result, client.OptPath("search", "tv"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return types.Ptr(result), nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - GET

// GetMovie returns the details of a movie, such as its runtime
func (c *Client) GetMovie(ctx context.Context, id uint64) (*schema.MovieDetails, error) {
	var result schema.MovieDetails
	if err := c.DoWithContext(ctx, nil, &result, client.OptPath("movie", id)); err != nil {
		return nil, err
	}
	return types.Ptr(result), nil
}

// GetTVShow returns the details of a TV show, such as the typical runtimes
// of its episodes, with its identifiers in other databases, such as IMDb
func (c *Client) GetTVShow(ctx context.Context, id uint64) (*schema.TVShowDetails, error) {
	var result schema.TVShowDetails
	if err := c.DoWithContext(ctx, nil, &result, client.OptPath("tv", id), client.OptQuery(withExternalIDs)); err != nil {
		return nil, err
	}
	return types.Ptr(result), nil
}

// GetTVEpisode returns an episode of a TV show, by its season and episode
// numbers, such as its name and runtime, with its identifiers in other
// databases, such as IMDb
func (c *Client) GetTVEpisode(ctx context.Context, id, season, episode uint64) (*schema.TVEpisode, error) {
	var result schema.TVEpisode
	if err := c.DoWithContext(ctx, nil, &result, client.OptPath("tv", id, "season", season, "episode", episode), client.OptQuery(withExternalIDs)); err != nil {
		return nil, err
	}
	return types.Ptr(result), nil
}
