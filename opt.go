package metadata

import (
	"mime"
	"slices"
	"strings"

	// Packages
	byterange "github.com/mutablelogic/go-metadata/internal/byterange"
	tmdb "github.com/mutablelogic/go-metadata/pkg/tmdb"
	types "github.com/mutablelogic/go-server/pkg/types"
	trace "go.opentelemetry.io/otel/trace"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Opts struct {
	// List of namespaces that the caller is interested in.
	namespaces []string

	// Returned content type, and its parameters
	contentType string
	params      map[string]string

	// Maximum size of the input, or zero for no limit
	maxSize int64

	// The part of the Markdown which a handler of the markdown namespace
	// returns: from the byte offset, and at most limit bytes, or all of it
	// when the limit is zero
	offset, limit uint64

	// The password of a document which is encrypted
	password string

	// The largest width or height of an image, in pixels, such as artwork,
	// which is downscaled to fit
	maxImageDimension int

	// The resolution of an image of a page, in dots per inch, which is
	// downscaled to it
	imageDPI int

	// The client of TMDB, which a handler of videos uses to look up a movie
	// or TV episode, or nil when it isn't looked up
	tmdb *tmdb.Client

	// The movie on TMDB which a video is, by its identifier, or the episode,
	// by its show's identifier, and its season and episode numbers, which
	// are got rather than looked up, or zero when they aren't known
	tmdbMovie                         uint64
	tmdbShow, tmdbSeason, tmdbEpisode uint64

	// Add OTEL tracing
	tracer trace.Tracer

	// The metadata of the handlers which ran first, which a handler added
	// with AddHandlerAfter gets
	prior []Metadata
}

type Option func(*Opts) error

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func applyOptions(opts ...Option) (*Opts, error) {
	o := &Opts{limit: byterange.DefaultLimit, maxImageDimension: defaultMaxImageDimension}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SETTER FUNCTIONS

// Set the otel tracer to use for metadata operations.
func WithTracer(tracer trace.Tracer) Option {
	return func(o *Opts) error {
		o.tracer = tracer
		return nil
	}
}

// Set the content type of the reader, which selects the handlers to run.
// When not set, the content type is detected from the reader.
func WithContentType(contentType string) Option {
	return func(o *Opts) error {
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return ErrBadParameter.Withf("invalid content type: %q", contentType)
		}
		o.contentType, o.params = mediaType, params
		return nil
	}
}

// Set the maximum size of the input, in bytes. A larger input is a bad
// parameter, and a stream is copied to a temporary file only up to this
// size. When not set, or zero, there is no limit.
func WithMaxSize(size int64) Option {
	return func(o *Opts) error {
		if size < 0 {
			return ErrBadParameter.Withf("invalid max size: %d", size)
		}
		o.maxSize = size
		return nil
	}
}

// WithRange sets the part of a document's Markdown which a handler of the
// markdown namespace returns, as a range of bytes, so a long document is
// read a part at a time: from:to, from:, :to, or : for all of it. When not
// set, or empty, it's the first kilobyte. A range which ends holds at least
// a whole character.
func WithRange(r string) Option {
	return func(o *Opts) error {
		offset, limit, err := byterange.Parse(r)
		if err != nil {
			return ErrBadParameter.With(err)
		}
		o.offset, o.limit = offset, limit
		return nil
	}
}

// Set the password of a document which is encrypted, such as a PDF document
// which needs a password to be opened, which a handler uses to read it
func WithPassword(password string) Option {
	return func(o *Opts) error {
		o.password = password
		return nil
	}
}

// Set the largest width or height of an image, in pixels, which a handler
// downscales an image to fit, keeping its aspect ratio. It's the size of an
// image without a size of its own, such as a photo, and the most an image of
// a page can be, such as a poster, unless its resolution is set (see
// WithImageDPI). When not set, it's 1024 pixels.
func WithMaxImageDimension(pixels int) Option {
	return func(o *Opts) error {
		if pixels <= 0 {
			return ErrBadParameter.Withf("invalid max image dimension: %d", pixels)
		}
		o.maxImageDimension = pixels
		return nil
	}
}

// Set the resolution of an image of a page, in dots per inch, such as the
// artwork of a scanned page, which a handler downscales an image to, from
// the size of its page, keeping its aspect ratio, so an A4 page at 300 dpi,
// as for OCR, is 2480 by 3508 pixels. An image is never upscaled. When it's
// set, an image of a page is at its resolution, however large, and when it
// isn't, it's 150 dpi, and at most the largest dimension (see
// WithMaxImageDimension).
func WithImageDPI(dpi int) Option {
	return func(o *Opts) error {
		if dpi <= 0 {
			return ErrBadParameter.Withf("invalid image dpi: %d", dpi)
		}
		o.imageDPI = dpi
		return nil
	}
}

// Set the client of TMDB, which a handler of videos uses to look up the
// movie or TV episode which a video is, from its name. When not set, videos
// aren't looked up.
func WithTMDB(client *tmdb.Client) Option {
	return func(o *Opts) error {
		if client == nil {
			return ErrBadParameter.With("nil TMDB client")
		}
		o.tmdb = client
		return nil
	}
}

// Set the movie on TMDB which a video is, by its identifier, such as 603 for
// The Matrix, which is got rather than looked up from the video's name, when
// a TMDB client is set (see WithTMDB). It replaces an episode set with
// WithTMDBEpisode.
func WithTMDBMovie(id uint64) Option {
	return func(o *Opts) error {
		if id == 0 {
			return ErrBadParameter.With("invalid TMDB movie: 0")
		}
		o.tmdbMovie = id
		o.tmdbShow, o.tmdbSeason, o.tmdbEpisode = 0, 0, 0
		return nil
	}
}

// Set the episode of a TV show on TMDB which a video is, by its show's
// identifier, such as 1396 for Breaking Bad, and its season and episode
// numbers, which is got rather than looked up from the video's name, when a
// TMDB client is set (see WithTMDB). An episode has an identifier of its
// own, but TMDB gets an episode only by its show, season and episode. The
// season is zero for a show's specials. It replaces a movie set with
// WithTMDBMovie.
func WithTMDBEpisode(show, season, episode uint64) Option {
	return func(o *Opts) error {
		if show == 0 || episode == 0 {
			return ErrBadParameter.Withf("invalid TMDB episode: show %d, episode %d", show, episode)
		}
		o.tmdbShow, o.tmdbSeason, o.tmdbEpisode = show, season, episode
		o.tmdbMovie = 0
		return nil
	}
}

// Return only the metadata from the given namespaces.
func WithNamespace(ns ...string) Option {
	return func(o *Opts) error {
		for _, n := range ns {
			if !types.IsIdentifier(n) {
				return ErrBadParameter.Withf("invalid namespace: %q", n)
			}
		}
		o.namespaces = ns
		return nil
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - GETTERS

// HasNamespace reports whether ns was requested via WithNamespace,
func (o *Opts) HasNamespace(ns string) bool {
	for _, namespace := range o.namespaces {
		if strings.EqualFold(namespace, ns) {
			return true
		}
	}
	return false
}

// Range returns the byte offset and limit of the part of the Markdown which
// a handler returns, set via WithRange, or the first kilobyte when it wasn't
// set. A limit of zero is to the end.
func (o *Opts) Range() (offset, limit uint64) {
	return o.offset, o.limit
}

// MaxImageDimension returns the largest width or height of an image, in
// pixels, which a handler downscales an image to fit, set via
// WithMaxImageDimension, or 1024 pixels when it wasn't set
func (o *Opts) MaxImageDimension() int {
	return o.maxImageDimension
}

// ImageDPI returns the resolution of an image of a page, in dots per inch,
// which a handler downscales an image to, set via WithImageDPI, or 150 dpi
// when it wasn't set
func (o *Opts) ImageDPI() int {
	if o.imageDPI == 0 {
		return defaultImageDPI
	}
	return o.imageDPI
}

// HasImageDPI reports whether the resolution of an image of a page was set
// via WithImageDPI, when an image of a page is at its resolution, rather
// than at most the largest dimension
func (o *Opts) HasImageDPI() bool {
	return o.imageDPI != 0
}

// Password returns the password of a document which is encrypted, set via
// WithPassword, or an empty string when it wasn't set
func (o *Opts) Password() string {
	return o.password
}

// ContentType returns the content type that was set via WithContentType, or
// detected from the reader when it wasn't set.
func (o *Opts) ContentType() string {
	return o.contentType
}

// TMDB returns the client of TMDB set via WithTMDB, or nil when it wasn't
// set, when videos aren't looked up
func (o *Opts) TMDB() *tmdb.Client {
	return o.tmdb
}

// TMDBMovie returns the identifier of the movie on TMDB set via
// WithTMDBMovie, or zero when it wasn't set
func (o *Opts) TMDBMovie() uint64 {
	return o.tmdbMovie
}

// TMDBEpisode returns the identifier of the show on TMDB, and the season and
// episode numbers, set via WithTMDBEpisode, or zeros when they weren't set
func (o *Opts) TMDBEpisode() (show, season, episode uint64) {
	return o.tmdbShow, o.tmdbSeason, o.tmdbEpisode
}

// Get returns the first metadata with the namespace and key from the
// handlers which ran before this one, or nil when there's none. Only a
// handler added with AddHandlerAfter gets it, of the namespaces it uses.
func (o *Opts) Get(ns, key string) Metadata {
	if i := slices.IndexFunc(o.prior, has(ns, key)); i >= 0 {
		return o.prior[i]
	}
	return nil
}
