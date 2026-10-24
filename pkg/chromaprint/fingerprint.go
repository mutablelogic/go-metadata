//go:build cgo && chromaprint

package chromaprint

import (
	"context"
	"errors"
	"io"
	"time"
	"unsafe"

	// Packages
	segmenter "github.com/mutablelogic/go-media/pkg/segmenter"
	chromaprint "github.com/mutablelogic/go-media/sys/chromaprint"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// sampleRate is the rate which audio is decoded at, in one channel, to
	// be fingerprinted
	sampleRate = 32000

	// MaxDuration is the most of audio which is fingerprinted, which is
	// enough for AcoustID to match it
	MaxDuration = 120 * time.Second
)

// Enabled is true, as it's built with cgo and the chromaprint tag, so
// audio is fingerprinted
const Enabled = true

// ErrNoAudio is returned for audio which has no samples to fingerprint
var ErrNoAudio = errors.New("no audio to fingerprint")

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Fingerprint returns the Chromaprint fingerprint of the audio, which is of
// up to the first MaxDuration of it, as AcoustID looks it up, and the
// duration of the audio, which is the whole of it, when it's known, or else
// of what was fingerprinted
func Fingerprint(ctx context.Context, r io.Reader) (string, time.Duration, error) {
	seg, err := segmenter.NewFromReader(r, sampleRate, segmenter.WithSegmentSize(time.Second))
	if err != nil {
		return "", 0, err
	}
	defer seg.Close()

	fp := chromaprint.NewChromaprint(chromaprint.ALGORITHM_DEFAULT)
	if fp == nil {
		return "", 0, errors.New("chromaprint: can't create a context")
	}
	defer fp.Free()
	if err := fp.Start(sampleRate, 1); err != nil {
		return "", 0, err
	}

	// Write the samples, up to MaxDuration
	var decoded time.Duration
	if err := seg.DecodeInt16(ctx, func(timestamp time.Duration, data []int16) error {
		if timestamp >= MaxDuration {
			return io.EOF
		} else if len(data) == 0 {
			return nil
		}
		// The samples as bytes, which keeps them a Go pointer, rather than a
		// uintptr, which isn't one
		if err := fp.Write(unsafe.Slice((*byte)(unsafe.Pointer(&data[0])), len(data)*2)); err != nil {
			return err
		}
		decoded = timestamp + time.Duration(len(data))*time.Second/sampleRate
		return nil
	}); err != nil && !errors.Is(err, io.EOF) {
		return "", 0, err
	}
	if decoded == 0 {
		return "", 0, ErrNoAudio
	}

	if err := fp.Finish(); err != nil {
		return "", 0, err
	}
	fingerprint, err := fp.GetFingerprint()
	if err != nil {
		return "", 0, err
	}

	// The duration of the whole of the audio, when it's known
	duration := seg.Duration()
	if duration <= 0 {
		duration = decoded
	}
	return fingerprint, duration, nil
}
