package metadata

import (
	"mime"
	"strings"

	// Packages
	byterange "github.com/mutablelogic/go-metadata/internal/byterange"
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

	// Add OTEL tracing
	tracer trace.Tracer
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
