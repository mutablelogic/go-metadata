// Package acoustid implements a client for the AcoustID API, which looks up
// the recordings which a Chromaprint fingerprint of audio is, and their
// MusicBrainz identifiers
// https://acoustid.org/webservice
package acoustid

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-metadata/pkg/acoustid/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of the AcoustID API, with the API key of an application
// which is registered with AcoustID
type Client struct {
	*client.Client
	key string
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	endpoint = "https://api.acoustid.org/v2"

	// rateLimit is the most requests each second which AcoustID allows
	rateLimit = 3.0

	// statusOK is the status of a response which isn't an error
	statusOK = "ok"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates an AcoustID client with the API key of an application, which
// is registered at https://acoustid.org/new-application, and options. Its
// requests are limited to three each second, which AcoustID allows.
func New(key string, opts ...client.ClientOpt) (*Client, error) {
	if key = strings.TrimSpace(key); key == "" {
		return nil, httpresponse.ErrBadRequest.With("missing AcoustID API key")
	}
	opts = append([]client.ClientOpt{
		client.OptEndpoint(endpoint),
		client.OptRateLimit(rateLimit),
	}, opts...)
	c, err := client.New(opts...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c, key: key}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Lookup returns the AcoustID tracks which the Chromaprint fingerprint of
// audio of the duration matches, with the best match first, and their
// metadata, such as their recordings and releases, which the meta flags
// select. The duration is of the whole of the audio, rather than of the part
// which was fingerprinted, and is in whole seconds. A fingerprint which
// matches nothing has no tracks.
func (c *Client) Lookup(ctx context.Context, fingerprint string, duration time.Duration, meta Meta) ([]schema.Result, error) {
	switch {
	case strings.TrimSpace(fingerprint) == "":
		return nil, httpresponse.ErrBadRequest.With("missing fingerprint")
	case duration < time.Second:
		return nil, httpresponse.ErrBadRequest.Withf("invalid duration: %v", duration)
	}

	params := url.Values{}
	params.Set("fingerprint", fingerprint)
	params.Set("duration", strconv.FormatInt(int64(duration/time.Second), 10))
	return c.lookup(ctx, params, meta)
}

// LookupTrack returns the AcoustID track with the identifier, such as the
// one which MusicBrainz Picard writes in the tags of audio, and its
// metadata, such as its recordings, which the meta flags select. An
// identifier which isn't known has no track.
func (c *Client) LookupTrack(ctx context.Context, id string, meta Meta) ([]schema.Result, error) {
	if id = strings.TrimSpace(id); id == "" {
		return nil, httpresponse.ErrBadRequest.With("missing AcoustID identifier")
	}
	params := url.Values{}
	params.Set("trackid", id)
	return c.lookup(ctx, params, meta)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// lookup returns the tracks which the parameters of a lookup, which are a
// fingerprint and duration, or a track's identifier, match, with the
// metadata which the meta flags select
func (c *Client) lookup(ctx context.Context, params url.Values, meta Meta) ([]schema.Result, error) {
	params.Set("client", c.key)
	if meta != MetaNone {
		params.Set("meta", meta.String())
	}

	var response schema.LookupResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("lookup"), client.OptQuery(params)); err != nil {
		return nil, err
	}
	if response.Status != statusOK {
		message := response.Error.Message
		if message == "" {
			message = "unknown error"
		}
		return nil, httpresponse.ErrBadRequest.Withf("acoustid: %s", message)
	}
	return response.Results, nil
}
