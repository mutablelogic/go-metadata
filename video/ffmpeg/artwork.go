//go:build cgo

package ffmpeg

import (
	"cmp"
	"context"
	"fmt"
	"image"
	"io"
	"maps"
	"slices"

	// Packages
	gomedia "github.com/mutablelogic/go-media"
	reader "github.com/mutablelogic/go-media/reader"
	thumbnailer "github.com/mutablelogic/go-media/thumbnailer"
	metadata "github.com/mutablelogic/go-metadata"
	artwork "github.com/mutablelogic/go-metadata/internal/artwork"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// coverKey is the key of the cover art of a video, as artwork, which is
	// coverKey-2 and so on for those after the first
	coverKey = "cover"

	// thumbnailKey is the key of the frame which best represents a video, as
	// artwork, and thumbnailKey-2 and so on are the keys of the frames of
	// its other scenes, in time order
	thumbnailKey = "thumbnail"

	// maxThumbnails is the most frames of a video which are its artwork
	maxThumbnails = 6
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

	// Frames of a video, as artwork, only when artwork is requested, which
	// runs after the handler of video, for whether it's protected with DRM,
	// when its frames can't be decoded
	metadata.AddHandlerAfter(reContentType, "ffmpeg-thumbnails", []string{VideoNamespace}, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		if !o.HasNamespace(metadata.ArtworkNamespace) {
			return nil, nil
		} else if drm := o.Get(VideoNamespace, "drm"); drm != nil && drm.Any() == true {
			return nil, nil
		}
		return thumbnailArtwork(ctx, r, o)
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

// thumbnailArtwork returns the frames of a video, as artwork: the one which
// best represents it, as artwork:thumbnail, and the best of each of its
// other scenes which make good thumbnails, as artwork:thumbnail-2 and so on,
// in time order, or nothing when it has no video stream, or its frames can't
// be decoded
func thumbnailArtwork(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
	th, err := thumbnailer.NewReader(r, thumbnailer.WithSize(0, 0))
	if err != nil {
		return nil, nil
	}
	defer th.Close()

	// The poster, and the thumbnails of other scenes
	poster, err := th.Poster(ctx)
	if err != nil {
		return nil, ctx.Err()
	}
	thumbnails, err := th.Thumbnails(ctx, maxThumbnails)
	if err != nil {
		return nil, ctx.Err()
	}
	var result []metadata.Metadata
	if entry := frameImage(poster.Image, thumbnailKey, o); entry != nil {
		result = append(result, entry)
	}

	for _, thumbnail := range otherScenes(*poster, thumbnails, maxThumbnails-1) {
		if entry := frameImage(thumbnail.Image, fmt.Sprintf("%s-%d", thumbnailKey, len(result)+1), o); entry != nil {
			result = append(result, entry)
		}
	}
	return result, nil
}

// otherScenes returns the best thumbnail of each scene other than the
// poster's, as there are more thumbnails than scenes when a video has few,
// and of the best n scenes when there are more, in time order
func otherScenes(poster thumbnailer.Thumbnail, thumbnails []thumbnailer.Thumbnail, n int) []thumbnailer.Thumbnail {
	best := make(map[int]thumbnailer.Thumbnail)
	for _, thumbnail := range thumbnails {
		if thumbnail.Scene == poster.Scene {
			continue
		}
		if b, exists := best[thumbnail.Scene]; !exists || thumbnail.Stats.Score > b.Stats.Score {
			best[thumbnail.Scene] = thumbnail
		}
	}
	result := slices.Collect(maps.Values(best))
	slices.SortFunc(result, func(a, b thumbnailer.Thumbnail) int { return cmp.Compare(b.Stats.Score, a.Stats.Score) })
	result = result[:min(n, len(result))]
	slices.SortFunc(result, func(a, b thumbnailer.Thumbnail) int { return cmp.Compare(a.Time, b.Time) })
	return result
}

// frameImage returns a frame of a video as artwork, with the key, downscaled
// to fit the largest image dimension, or nil when it can't be encoded
func frameImage(img image.Image, key string, o *metadata.Opts) metadata.Metadata {
	data := artwork.Encode(artwork.Downscale(img, o.MaxImageDimension()))
	if data == nil {
		return nil
	}
	return meta.NewImage(metadata.ArtworkNamespace, key, data)
}
