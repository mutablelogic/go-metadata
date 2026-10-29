//go:build cgo && libfvad

package classify

import (
	"context"
	"errors"
	"io"
	"math"
	"regexp"
	"time"

	// Packages
	segmenter "github.com/mutablelogic/go-media/segmenter"
	metadata "github.com/mutablelogic/go-metadata"
	playlist "github.com/mutablelogic/go-metadata/audio/playlist"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of audio's metadata, which the class is
	// returned in, and which ffmpeg's handler reads, such as whether it's
	// protected with DRM, as audio:drm
	Namespace = "audio"

	// MaxDuration is the most of audio which is classified, from its start,
	// so long audio, such as an audiobook, is classified quickly
	MaxDuration = 10 * time.Minute

	// Classes of audio, as audio:class
	ClassSpeech  = "speech"
	ClassMusic   = "music"
	ClassSilence = "silence"
	ClassOther   = "other"

	// sampleRate is the rate audio is decoded at, in one channel, to be
	// classified, which is the rate the segmenter's classes were tuned at
	sampleRate = 16000

	// minSound is the least fraction of audio which isn't silence, for it
	// to be classified as anything other than silence
	minSound = 0.1
)

var (
	reContentType = regexp.MustCompile(`^audio/.+$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The class of audio, which runs after the handler of audio, for whether
	// it's protected with DRM
	metadata.AddHandlerAfter(reContentType, "classify", []string{Namespace}, func(ctx context.Context, r io.Reader, o *metadata.Opts) ([]metadata.Metadata, error) {
		// A playlist, which is text, isn't audio, and audio protected with
		// DRM can't be decoded
		if playlist.ContentTypes.MatchString(o.ContentType()) {
			return nil, nil
		} else if drm := o.Get(Namespace, "drm"); drm != nil && drm.Any() == true {
			return nil, nil
		}
		durations, err := decode(ctx, r)
		if err != nil {
			return nil, err
		}
		return classify(durations), nil
	}, Namespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// decode returns how long each class of the audio is, of up to its first
// MaxDuration, in segments which are as long as the window each segment is
// classified from, so each is classified from the whole of it
func decode(ctx context.Context, r io.Reader) (map[segmenter.Class]time.Duration, error) {
	seg, err := segmenter.NewReader(r, sampleRate,
		segmenter.WithMode(segmenter.ModeVADAggressive, 0, 0),
		segmenter.WithSegments(segmenter.ClassifyWindow),
	)
	if err != nil {
		return nil, err
	}
	defer seg.Close()

	result := make(map[segmenter.Class]time.Duration)
	if err := seg.Decode(ctx, func(segment *segmenter.Segment) error {
		if segment.Start >= MaxDuration {
			return io.EOF
		}
		result[segment.Class] += min(segment.Duration, MaxDuration-segment.Start)
		return nil
	}); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return result, nil
}

// classify returns the class of audio, from how long each class of it is,
// which is silence when too little of it is anything else, or else the
// longest of speech, music and other sound, and the fractions which are
// speech, music and silence, or nothing when there's no audio
func classify(durations map[segmenter.Class]time.Duration) []metadata.Metadata {
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	if total <= 0 {
		return nil
	}
	fraction := func(classes ...segmenter.Class) float64 {
		var d time.Duration
		for _, class := range classes {
			d += durations[class]
		}
		return math.Round(float64(d)/float64(total)*100) / 100
	}

	// Silence, unless enough of it is something else, which is the longest
	// of the classes which aren't silence
	class := ClassSilence
	if 1-float64(durations[segmenter.ClassSilence])/float64(total) >= minSound {
		class = ClassSpeech
		longest := durations[segmenter.ClassSpeech]
		if d := durations[segmenter.ClassMusic]; d > longest {
			class, longest = ClassMusic, d
		}
		if d := durations[segmenter.ClassOther] + durations[segmenter.ClassUnknown]; d > longest {
			class = ClassOther
		}
	}
	return []metadata.Metadata{
		meta.New(Namespace, "class", class),
		meta.New(Namespace, "speech", fraction(segmenter.ClassSpeech)),
		meta.New(Namespace, "music", fraction(segmenter.ClassMusic)),
		meta.New(Namespace, "silence", fraction(segmenter.ClassSilence)),
	}
}
