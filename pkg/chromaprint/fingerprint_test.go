//go:build cgo && chromaprint

package chromaprint_test

import (
	"context"
	"encoding/base64"
	"errors"
	"math/bits"
	"os"
	"strings"
	"testing"
	"time"

	// Packages
	chromaprint "github.com/mutablelogic/go-metadata/pkg/chromaprint"
)

func Test_chromaprint_000(t *testing.T) {
	// The fingerprint of audio, and its duration
	r, err := os.Open("../../testfiles/jfk.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	fingerprint, duration, err := chromaprint.Fingerprint(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(fingerprint, "AQAA") {
		t.Errorf("unexpected fingerprint %q", fingerprint)
	}
	if duration < 10*time.Second || duration > 12*time.Second {
		t.Errorf("unexpected duration %v", duration)
	}

	// The fingerprint is of all the audio, from its start, as the fingerprint
	// of the audio decoded by the ffmpeg command is, with the same number of
	// items, and nearly the same bits, which can differ a little with the FFT
	// Chromaprint is built with
	got, err := decodeFingerprint(fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	want, err := decodeFingerprint(jfkFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d items, got %d", len(want), len(got))
	}
	var diff int
	for i := range got {
		diff += bits.OnesCount32(got[i] ^ want[i])
	}
	if rate := float64(diff) / float64(32*len(got)); rate > maxBitErrorRate {
		t.Errorf("expected a bit error rate of at most %v, got %.3f", maxBitErrorRate, rate)
	}
}

////////////////////////////////////////////////////////////////////////////////
// HELPERS

const (
	// jfkFingerprint is the fingerprint of jfk.wav, as decoded by the ffmpeg
	// command, at 32kHz in one channel, which is the reference
	jfkFingerprint = "AQAAQ5ISPluiYMqXFJXDHpcePGSR3D-yHojO4xfKC8myXBLxB7OoTQITKkl-VNbxRJIwZVlQXg1xhsXFo-GD00hyBbl4_IGV43nxHxYT_Ogv5EGyRylsHtMT4b_wMMWZ4pxzKEfzo8cbG4-yEF8CZ5Xw1Jj0OAma77jyotrQyDOuo-Z2oWmW4Li0G1Yf4ZNKpGuhK8hzOAmJ8yKePYV_AsEIMggAohRxgAELBBAAECQgIQYC4wwjTgAwjChQAMAAM8IAQRlV1AhBFPDAA2AAcw4YYgQA"

	// maxBitErrorRate is the most fraction of the bits of a fingerprint
	// which can differ from the reference's
	maxBitErrorRate = 0.1
)

// decodeFingerprint decodes a compressed Chromaprint fingerprint, as
// Chromaprint compresses it: a header of its algorithm, and its number of
// items, and then each item, as it differs from the one before, by the
// positions of its bits which are set, as the differences between them,
// packed in three bits each, and those of seven or more continued in five
// bits each, after them
func decodeFingerprint(s string) ([]uint32, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	} else if len(data) < 4 {
		return nil, errors.New("fingerprint is too short")
	}
	n := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	packed := data[4:]

	// read returns the value of width bits at a bit offset, from the least
	// significant bit of each byte, or false past the end
	read := func(offset, width int) (int, bool) {
		var v int
		for i := range width {
			bit := offset + i
			if bit/8 >= len(packed) {
				return 0, false
			}
			v |= int(packed[bit/8]>>(bit%8)&1) << i
		}
		return v, true
	}

	// The differences between the positions of bits, with zero at the end
	// of each item, and then those of seven or more continued
	var diffs []int
	offset := 0
	for items := 0; items < n; offset += 3 {
		v, ok := read(offset, 3)
		if !ok {
			return nil, errors.New("fingerprint is truncated")
		}
		diffs = append(diffs, v)
		if v == 0 {
			items++
		}
	}
	offset = (offset + 7) / 8 * 8
	for i, v := range diffs {
		if v == 7 {
			e, ok := read(offset, 5)
			if !ok {
				return nil, errors.New("fingerprint is truncated")
			}
			diffs[i] += e
			offset += 5
		}
	}

	// The items, from the bits which differ from the item before
	result := make([]uint32, 0, n)
	var item uint32
	var position int
	for _, v := range diffs {
		if v == 0 {
			if len(result) > 0 {
				item ^= result[len(result)-1]
			}
			result = append(result, item)
			item, position = 0, 0
			continue
		}
		position += v
		item |= 1 << (position - 1)
	}
	return result, nil
}

func Test_chromaprint_001(t *testing.T) {
	// What isn't audio is an error
	if _, _, err := chromaprint.Fingerprint(context.Background(), strings.NewReader("not audio")); err == nil {
		t.Error("expected an error")
	} else if errors.Is(err, context.Canceled) {
		t.Errorf("unexpected error %v", err)
	}
}

func Test_chromaprint_002(t *testing.T) {
	// The fingerprint of jfk.wav which the old segmenter made, which was
	// missing its first item, isn't the reference
	old, err := decodeFingerprint("AQAAQpISPsqyoLkwyckiKca1I8-R3DrYTzh2hOwDkZ6GkBeFV8eeWIZ_cFqNKlqCKdxU_Ca-GaVe_MHtI8l1pFVR3WiUo8yPOsnBBeeF5gpyHVI4JUKYZ8fxZjn2YybTDM-CZPmP8HgYxriijDgTOKvwZ-iboAmV5cHzoT-aH-eG0IeWKRPmNB9ix4G1z3j1YOvwhDwi0QnRJGXwG4p5ZfAfACERMQQ4oZYQSChhjEAEGgYIkpAJSQx1Bhg4EEEECOSURYIogBgxwDKAFCAEASkEAg")
	if err != nil {
		t.Fatal(err)
	}
	want, err := decodeFingerprint(jfkFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 67 || len(old) != 66 {
		t.Errorf("expected 67 and 66 items, got %d and %d", len(want), len(old))
	}
}
