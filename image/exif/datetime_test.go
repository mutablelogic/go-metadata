//go:build cgo

package exif

import (
	"testing"
	"time"

	// Packages
	rational "github.com/mutablelogic/go-metadata/pkg/rational"
)

func Test_datetime_000(t *testing.T) {
	// A date and time is in UTC without an offset, or at its offset
	for _, test := range []struct {
		value, offset string
		want          time.Time
		ok            bool
	}{
		{"2024:06:01 12:30:00", "", time.Date(2024, 6, 1, 12, 30, 0, 0, time.UTC), true},
		{"2024:06:01 12:30:00", "+02:00", time.Date(2024, 6, 1, 10, 30, 0, 0, time.UTC), true},
		{"2024:06:01 12:30:00", "-05:00", time.Date(2024, 6, 1, 17, 30, 0, 0, time.UTC), true},
		{"    :  :     :  :  ", "", time.Time{}, false},
		{"", "", time.Time{}, false},
	} {
		got, ok := parseDateTime(test.value, test.offset)
		if ok != test.ok || !got.Equal(test.want) {
			t.Errorf("%q %q: expected %v %v, got %v %v", test.value, test.offset, test.want, test.ok, got, ok)
		}
	}
}

func Test_gps_000(t *testing.T) {
	// Degrees, minutes and seconds are decimal degrees, and a zero
	// denominator isn't a number
	if got, ok := dmsDecimal([]rational.Rational{rational.New(52, 1), rational.New(30, 1), rational.New(3600, 100)}); !ok || got != 52.51 {
		t.Errorf("expected 52.51, got %v", got)
	}
	if _, ok := dmsDecimal([]rational.Rational{rational.New(52, 0), {}, {}}); ok {
		t.Error("expected no decimal degrees with a zero denominator")
	}
	if _, ok := dmsDecimal([]rational.Rational{rational.New(52, 1)}); ok {
		t.Error("expected no decimal degrees without minutes and seconds")
	}
	if got, ok := rationalFloat(rational.New(-1, 4)); !ok || got != -0.25 {
		t.Errorf("expected -0.25, got %v", got)
	}
}

func Test_datetime_001(t *testing.T) {
	// A date and time of zeros or spaces is unknown
	for value, want := range map[string]bool{
		"0000:00:00 00:00:00": true,
		"    :  :     :  :  ": true,
		"":                    true,
		"2024:06:01 12:30:00": false,
		"2000:00:00 00:00:00": false,
	} {
		if got := unknownDateTime(value); got != want {
			t.Errorf("%q: expected %v, got %v", value, want, got)
		}
	}
}

func Test_copyright_000(t *testing.T) {
	// The copyrights of the photographer and editor, without those which
	// aren't set, and a copyright which isn't formatted so as it is
	for value, want := range map[string]string{
		"[None] (Photographer) - [None] (Editor)":       "",
		"David Thorpe (Photographer) - [None] (Editor)": "David Thorpe",
		"[None] (Photographer) - Jane Doe (Editor)":     "Jane Doe",
		"Ada (Photographer) - Grace (Editor)":           "Ada; Grace",
		"© 2024 Ada":                                    "© 2024 Ada",
	} {
		if got := copyright(value); got != want {
			t.Errorf("%q: expected %q, got %q", value, want, got)
		}
	}
}
