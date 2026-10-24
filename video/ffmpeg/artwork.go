//go:build cgo

package ffmpeg

import (
	"context"
	"fmt"
	"io"

	// Packages
	gomedia "github.com/mutablelogic/go-media"
	reader "github.com/mutablelogic/go-media/reader"
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// coverKey is the key of the cover art of a video, as artwork, which is
	// coverKey-2 and so on for those after the first
	coverKey = "cover"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The cover art of a video, as artwork, only when artwork is requested
	metadata.AddHandler(reContentType, "ffmpeg-artwork", func(_ context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		}
		rd, err := reader.NewReader(r)
		if err != nil {
			return nil, nil
		}
		defer rd.Close()
		return coverArtwork(rd.Metadata(gomedia.MetaArtwork), o), nil
	}, metadata.ArtworkNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// coverArtwork returns the cover art of a video, which are its attached
// pictures, such as an MP4 covr atom, as artwork:cover, artwork:cover-2 and
// so on, without those which aren't images
func coverArtwork(pictures []gomedia.Metadata, o *metadata.Opts) []metadata.Metadata {
	var result []metadata.Metadata
	for _, picture := range pictures {
		key := coverKey
		if len(result) > 0 {
			key = fmt.Sprintf("%s-%d", coverKey, len(result)+1)
		}
		if entry := artwork.Image(picture.Bytes(), key, o); entry != nil {
			result = append(result, entry)
		}
	}
	return result
}
