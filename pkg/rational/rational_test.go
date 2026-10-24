package rational_test

import (
	"encoding/json"
	"testing"

	// Packages
	rational "github.com/mutablelogic/go-metadata/pkg/rational"
)

func Test_rational_000(t *testing.T) {
	// Text is a fraction or a decimal, reduced to its lowest terms
	for _, test := range []struct {
		text string
		want rational.Rational
	}{
		{"35/10", rational.New(7, 2)},
		{"1/60", rational.New(1, 60)},
		{" -1/4 ", rational.New(-1, 4)},
		{"3.5", rational.New(7, 2)},
		{"0", rational.New(0, 1)},
		{"72", rational.New(72, 1)},
	} {
		if got, err := rational.Parse(test.text); err != nil {
			t.Errorf("%q: %v", test.text, err)
		} else if got != test.want {
			t.Errorf("%q: expected %v, got %v", test.text, test.want, got)
		}
	}
}

func Test_rational_001(t *testing.T) {
	// Text which isn't a number, has a denominator of zero, or doesn't fit
	// 64 bits, is an error
	for _, text := range []string{"", "f/2.8", "1/0", "1/", "99999999999999999999/1"} {
		if got, err := rational.Parse(text); err == nil {
			t.Errorf("%q: expected an error, got %v", text, got)
		}
	}
}

func Test_rational_002(t *testing.T) {
	// A rational isn't reduced when it's made, is a fraction as text, and
	// back, and a number in JSON, or null without a denominator
	r := rational.New(35, 10)
	if r.String() != "35/10" {
		t.Errorf("expected 35/10, got %s", r)
	}
	if v, ok := r.Float64(); !ok || v != 3.5 {
		t.Errorf("expected 3.5, got %v", v)
	}
	var got rational.Rational
	if err := got.UnmarshalText([]byte("1/60")); err != nil || got != rational.New(1, 60) {
		t.Errorf("expected 1/60, got %v %v", got, err)
	}
	for _, test := range []struct {
		r    rational.Rational
		want string
	}{
		{r, "3.5"},
		{rational.New(-1, 4), "-0.25"},
		{rational.New(1, 0), "null"},
	} {
		if data, err := json.Marshal(test.r); err != nil {
			t.Fatal(err)
		} else if string(data) != test.want {
			t.Errorf("%v: expected %s, got %s", test.r, test.want, data)
		}
	}
	if _, ok := rational.New(1, 0).Float64(); ok || rational.New(1, 0).IsValid() {
		t.Error("expected no number without a denominator")
	}
}
