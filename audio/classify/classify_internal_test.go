//go:build cgo && libfvad

package classify

import (
	"testing"
	"time"

	// Packages
	segmenter "github.com/mutablelogic/go-media/segmenter"
)

func Test_classify_000(t *testing.T) {
	// The longest class which isn't silence, unless too little of the audio
	// is anything but silence
	for _, test := range []struct {
		durations map[segmenter.Class]time.Duration
		class     string
	}{
		{map[segmenter.Class]time.Duration{segmenter.ClassSpeech: 8 * time.Second, segmenter.ClassSilence: 2 * time.Second}, ClassSpeech},
		{map[segmenter.Class]time.Duration{segmenter.ClassSpeech: 2 * time.Second, segmenter.ClassMusic: 6 * time.Second}, ClassMusic},
		{map[segmenter.Class]time.Duration{segmenter.ClassOther: 5 * time.Second, segmenter.ClassMusic: 1 * time.Second}, ClassOther},
		{map[segmenter.Class]time.Duration{segmenter.ClassSilence: 10 * time.Second}, ClassSilence},
		{map[segmenter.Class]time.Duration{segmenter.ClassSilence: 95 * time.Second, segmenter.ClassSpeech: 5 * time.Second}, ClassSilence},
		{map[segmenter.Class]time.Duration{segmenter.ClassSilence: 85 * time.Second, segmenter.ClassSpeech: 15 * time.Second}, ClassSpeech},
	} {
		got := map[string]string{}
		for _, m := range classify(test.durations) {
			got[m.Key()] = m.Value()
		}
		if got["class"] != test.class {
			t.Errorf("%v: expected %q, got %q", test.durations, test.class, got["class"])
		}
	}
}

func Test_classify_001(t *testing.T) {
	// The fractions of speech, music and silence
	got := map[string]any{}
	for _, m := range classify(map[segmenter.Class]time.Duration{
		segmenter.ClassSpeech:  6 * time.Second,
		segmenter.ClassMusic:   3 * time.Second,
		segmenter.ClassSilence: 1 * time.Second,
	}) {
		got[m.Key()] = m.Any()
	}
	for key, want := range map[string]float64{"speech": 0.6, "music": 0.3, "silence": 0.1} {
		if got[key] != want {
			t.Errorf("%s: expected %v, got %v", key, want, got[key])
		}
	}
}

func Test_classify_002(t *testing.T) {
	// No audio has no class
	if got := classify(map[segmenter.Class]time.Duration{}); got != nil {
		t.Errorf("expected nothing, got %v", got)
	}
}
