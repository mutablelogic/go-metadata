// Package rational defines a rational number, a numerator and denominator,
// as in EXIF and XMP metadata, such as an exposure time of 1/60 second, which
// is text such as "1/60", and a number in JSON
package rational

import (
	"encoding"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Rational is a signed rational number, as a numerator and a denominator,
// which isn't reduced, as it's stored, such as 35/10 for f/3.5. A rational
// with a denominator of zero isn't a number.
type Rational struct {
	Numerator   int64
	Denominator int64
}

var _ encoding.TextMarshaler = Rational{}
var _ encoding.TextUnmarshaler = (*Rational)(nil)
var _ json.Marshaler = Rational{}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// separator is between the numerator and denominator in text
	separator = "/"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a rational number, as it is, without reducing it
func New(numerator, denominator int64) Rational {
	return Rational{Numerator: numerator, Denominator: denominator}
}

// Parse returns a rational number from text, which is a fraction, such as
// "35/10" or "-1/4", or a decimal, such as "3.5", reduced to its lowest
// terms, such as 7/2, or an error when it isn't a number, or its denominator
// is zero, or it doesn't fit 64 bits
func Parse(text string) (Rational, error) {
	r, ok := new(big.Rat).SetString(strings.TrimSpace(text))
	if !ok {
		return Rational{}, metadata.ErrBadParameter.Withf("rational %q: not a number", text)
	} else if !r.Num().IsInt64() || !r.Denom().IsInt64() {
		return Rational{}, metadata.ErrBadParameter.Withf("rational %q: too large", text)
	}
	return Rational{Numerator: r.Num().Int64(), Denominator: r.Denom().Int64()}, nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// String returns the rational number as a fraction, such as "35/10"
func (r Rational) String() string {
	return strconv.FormatInt(r.Numerator, 10) + separator + strconv.FormatInt(r.Denominator, 10)
}

// MarshalText returns the rational number as a fraction, such as "35/10"
func (r Rational) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

// UnmarshalText sets the rational number from a fraction, such as "35/10",
// or a decimal, such as "3.5" (see Parse)
func (r *Rational) UnmarshalText(text []byte) error {
	v, err := Parse(string(text))
	if err != nil {
		return err
	}
	*r = v
	return nil
}

// MarshalJSON returns the rational number as a number, such as 3.5, or null
// when its denominator is zero
func (r Rational) MarshalJSON() ([]byte, error) {
	if v, ok := r.Float64(); ok {
		return json.Marshal(v)
	}
	return []byte("null"), nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Float64 returns the rational number as a float64, or false when its
// denominator is zero
func (r Rational) Float64() (float64, bool) {
	if r.Denominator == 0 {
		return 0, false
	}
	return float64(r.Numerator) / float64(r.Denominator), true
}

// IsValid returns true when the rational number has a denominator, so is a
// number
func (r Rational) IsValid() bool {
	return r.Denominator != 0
}
