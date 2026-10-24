//go:build cgo && chromaprint

package chromaprint_test

import (
	"context"
	"errors"
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
}

func Test_chromaprint_001(t *testing.T) {
	// What isn't audio is an error
	if _, _, err := chromaprint.Fingerprint(context.Background(), strings.NewReader("not audio")); err == nil {
		t.Error("expected an error")
	} else if errors.Is(err, context.Canceled) {
		t.Errorf("unexpected error %v", err)
	}
}
