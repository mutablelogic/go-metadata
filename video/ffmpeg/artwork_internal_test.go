//go:build cgo

package ffmpeg

import (
	"slices"
	"testing"
	"time"

	// Packages
	thumbnailer "github.com/mutablelogic/go-media/thumbnailer"
)

func Test_otherScenes(t *testing.T) {
	// The best thumbnail of each scene other than the poster's, of the best
	// scenes, in time order
	thumb := func(seconds, scene int, score float64) thumbnailer.Thumbnail {
		return thumbnailer.Thumbnail{Time: time.Duration(seconds) * time.Second, Scene: scene, Stats: thumbnailer.Stats{Score: score}}
	}
	times := func(thumbnails []thumbnailer.Thumbnail) []int {
		var result []int
		for _, t := range thumbnails {
			result = append(result, int(t.Time.Seconds()))
		}
		return result
	}
	poster := thumb(30, 1, 0.9)

	// A scene's best thumbnail, rather than its first, and none of the
	// poster's scene
	got := otherScenes(poster, []thumbnailer.Thumbnail{
		thumb(10, 0, 0.5), thumb(30, 1, 0.9), thumb(40, 1, 0.8), thumb(70, 2, 0.1), thumb(90, 2, 0.9),
	}, 5)
	if want := []int{10, 90}; !slices.Equal(times(got), want) {
		t.Errorf("expected %v, got %v", want, times(got))
	}

	// The best scenes, when there are more than n, in time order
	got = otherScenes(poster, []thumbnailer.Thumbnail{
		thumb(10, 0, 0.5), thumb(50, 2, 0.2), thumb(70, 3, 0.8), thumb(90, 4, 0.6),
	}, 2)
	if want := []int{70, 90}; !slices.Equal(times(got), want) {
		t.Errorf("expected %v, got %v", want, times(got))
	}

	// None, when there are only the poster's
	if got := otherScenes(poster, []thumbnailer.Thumbnail{poster}, 5); len(got) != 0 {
		t.Errorf("expected none, got %v", times(got))
	}
}
