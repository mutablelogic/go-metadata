package tmdb

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	transport "github.com/mutablelogic/go-client/pkg/transport"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// imageEndpoint is where TMDB's images are, which is the secure base URL of
// its configuration
const imageEndpoint = "https://image.tmdb.org/t/p"

// The sizes of images, which are the widths TMDB has, or the original
const (
	SizePoster   = "w780"
	SizeBackdrop = "w780"
	SizeStill    = "w300"
	SizeOriginal = "original"
)

// The widths, in pixels, of the sizes of posters, backdrops and stills which
// TMDB has, smallest first, other than the original
var (
	posterWidths   = []int{92, 154, 185, 342, 500, 780}
	backdropWidths = []int{300, 780, 1280}
	stillWidths    = []int{92, 185, 300}
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// PosterSize returns the smallest size of a poster, whose height is one and
// a half times its width, whose width and height are at least the
// dimension, in pixels, when one of them is, or else the original
func PosterSize(dimension int) string {
	return imageSize(posterWidths, (dimension*2+2)/3)
}

// BackdropSize returns the smallest size of a backdrop, which is wider than
// it's high, whose width is at least the dimension, in pixels, or else the
// original
func BackdropSize(dimension int) string {
	return imageSize(backdropWidths, dimension)
}

// StillSize returns the smallest size of an episode's still, which is wider
// than it's high, whose width is at least the dimension, in pixels, or else
// the original
func StillSize(dimension int) string {
	return imageSize(stillWidths, dimension)
}

// Image returns the image at the path, which is a movie's or show's poster
// or backdrop path, or an episode's still path, such as
// /qJ2tW6WMUDux911r6m7haRef0WH.jpg, in the size, such as SizePoster. The
// image server doesn't need the API token, so it isn't sent, but the request
// counts towards the client's quota.
func (c *Client) Image(ctx context.Context, size, path string) ([]byte, error) {
	if size = strings.TrimSpace(size); size == "" || strings.Contains(size, "/") {
		return nil, httpresponse.ErrBadRequest.Withf("invalid image size %q", size)
	} else if path = strings.TrimSpace(path); path == "" || !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		return nil, httpresponse.ErrBadRequest.Withf("invalid image path %q", path)
	}
	var buf bytes.Buffer
	if err := c.DoWithContext(transport.WithSkipTokenInjection(ctx), nil, &buf,
		client.OptReqEndpoint(imageEndpoint),
		client.OptPath(size, strings.TrimPrefix(path, "/")),
	); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// imageSize returns the size of the smallest of the widths which is at least
// the width, or else the original
func imageSize(widths []int, width int) string {
	for _, w := range widths {
		if w >= width {
			return "w" + strconv.Itoa(w)
		}
	}
	return SizeOriginal
}
