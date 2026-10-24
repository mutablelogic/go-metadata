package vcard

import (
	"errors"
	"io"
	"mime/quotedprintable"
	"slices"
	"strconv"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	contentline "github.com/mutablelogic/go-metadata/internal/contentline"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Decoder reads contacts from vCard 2.1, 3.0 or 4.0, one at a time
type Decoder struct {
	lines *contentline.Reader
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// MaxLineSize is the longest line which is read, once it's unfolded,
	// which includes a photo of several megabytes, as base64
	MaxLineSize = 16 << 20
)

// dateLayouts are the layouts of a date, such as a birthday, in vCard 4.0,
// as a date or a date without a year, and in vCard 3.0, which can have a
// time after it
var dateLayouts = []string{"20060102", "2006-01-02", "--0102", "--01-02"}

// timeLayouts are the layouts of when a contact was last changed
var timeLayouts = []string{"20060102T150405Z", "20060102T150405", time.RFC3339, "2006-01-02T15:04:05", "20060102"}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewDecoder returns a decoder of contacts from r
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{lines: contentline.NewReader(r, MaxLineSize)}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Decode returns the next contact, or io.EOF when there are no more. Lines
// outside a vCard are ignored, and a vCard which isn't ended is returned.
func (d *Decoder) Decode() (*Card, error) {
	var card *Card
	for {
		line, err := d.lines.Next()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, metadata.ErrBadParameter.Withf("vcard: %v", err)
		}
		property, ok := contentline.Parse(line)
		if !ok {
			continue
		}
		switch {
		case property.Name == "BEGIN" && strings.EqualFold(property.Value, "VCARD"):
			card = new(Card)
		case property.Name == "END" && strings.EqualFold(property.Value, "VCARD") && card != nil:
			return card, nil
		case card != nil:
			card.set(property)
		}
	}
	if card != nil {
		return card, nil
	}
	return nil, io.EOF
}

// Decode returns the contacts of r, which are vCard 2.1, 3.0 or 4.0
func Decode(r io.Reader) ([]*Card, error) {
	d := NewDecoder(r)
	var result []*Card
	for {
		card, err := d.Decode()
		if errors.Is(err, io.EOF) {
			return result, nil
		} else if err != nil {
			return result, err
		}
		result = append(result, card)
	}
}

// UnmarshalText sets the contact from the first vCard of the text
func (c *Card) UnmarshalText(data []byte) error {
	cards, err := Decode(strings.NewReader(string(data)))
	if err != nil {
		return err
	} else if len(cards) == 0 {
		return metadata.ErrBadParameter.With("vcard: no contact")
	}
	*c = *cards[0]
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// set sets the field of the contact of a property, whose value is decoded,
// when it's quoted-printable, as vCard 2.1 has
func (c *Card) set(p contentline.Property) {
	value := p.Value
	encoding := strings.ToUpper(first(p.Params["ENCODING"]))
	if encoding == "QUOTED-PRINTABLE" {
		if decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(value))); err == nil {
			value = string(decoded)
		}
	}
	types, pref := typesOf(p.Params)

	switch p.Name {
	case "UID":
		c.UID = unescape(value)
	case "KIND":
		c.Kind = strings.ToLower(unescape(value))
	case "FN":
		c.FN = unescape(value)
	case "N":
		parts := pad(contentline.Split(value, ';'), 5)
		c.Name = Name{Family: parts[0], Given: parts[1], Middle: parts[2], Prefix: parts[3], Suffix: parts[4]}
	case "NICKNAME":
		c.Nicknames = append(c.Nicknames, nonEmpty(contentline.Split(value, ','))...)
	case "EMAIL":
		c.Emails = appendValue(c.Emails, unescape(value), types, pref)
	case "TEL":
		c.Phones = appendValue(c.Phones, strings.TrimPrefix(unescape(value), "tel:"), types, pref)
	case "URL":
		c.URLs = appendValue(c.URLs, strings.TrimSpace(value), types, pref)
	case "ADR":
		parts := pad(contentline.Split(value, ';'), 7)
		if address := (Address{Types: types, POBox: parts[0], Extended: parts[1], Street: parts[2], Locality: parts[3], Region: parts[4], PostalCode: parts[5], Country: parts[6]}); address.String() != "" {
			c.Addresses = append(c.Addresses, trimAddress(address))
		}
	case "ORG":
		parts := nonEmpty(contentline.Split(value, ';'))
		if len(parts) > 0 {
			c.Orgs = append(c.Orgs, Organization{Name: parts[0], Units: parts[1:]})
		}
	case "TITLE":
		if title := unescape(value); title != "" {
			c.Titles = append(c.Titles, title)
		}
	case "BDAY":
		if d, ok := parseDate(unescape(value)); ok {
			c.Birthday = &d
		}
	case "NOTE":
		if note := unescape(value); note != "" {
			c.Notes = append(c.Notes, note)
		}
	case "CATEGORIES":
		c.Categories = append(c.Categories, nonEmpty(contentline.Split(value, ','))...)
	case "PHOTO":
		base64Encoded := encoding == "B" || encoding == "BASE64"
		imageType := first(p.Params["MEDIATYPE"])
		if imageType == "" {
			imageType = first(slices.DeleteFunc(slices.Clone(p.Params["TYPE"]), func(t string) bool { return strings.EqualFold(t, "pref") }))
		}
		if photo := parsePhoto(value, base64Encoded, imageType); photo != nil && c.Photo == nil {
			c.Photo = photo
		}
	case "REV":
		for _, layout := range timeLayouts {
			if t, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
				c.Rev = t
				break
			}
		}
	}
}

// typesOf returns the types of a property, in lower case, such as home, and
// whether it's preferred, as vCard 4.0's PREF, or vCard 3.0's pref type
func typesOf(params map[string][]string) ([]string, bool) {
	var types []string
	pref := len(params["PREF"]) > 0
	for _, param := range params["TYPE"] {
		// A list of types can be quoted, such as TYPE="voice,cell"
		for _, t := range strings.Split(param, ",") {
			switch t = strings.ToLower(strings.TrimSpace(t)); t {
			case "":
			case "pref":
				pref = true
			default:
				if !slices.Contains(types, t) {
					types = append(types, t)
				}
			}
		}
	}
	return types, pref
}

// appendValue appends a value, with its types, unless it's empty
func appendValue(values []Value, value string, types []string, pref bool) []Value {
	if value = strings.TrimSpace(value); value == "" {
		return values
	}
	return append(values, Value{Value: value, Types: types, Pref: pref})
}

// parseDate returns a date, which is without a year when it starts with
// two hyphens, or false when it isn't one
func parseDate(value string) (Date, bool) {
	value = strings.TrimSpace(value)
	if date, _, found := strings.Cut(value, "T"); found {
		value = date
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			d := Date{Month: int(t.Month()), Day: t.Day()}
			if !strings.HasPrefix(layout, "--") {
				d.Year = t.Year()
			}
			return d, true
		}
	}
	// A year alone, which a date can be in vCard 4.0
	if year, err := strconv.Atoi(value); err == nil && len(value) == 4 {
		return Date{Year: year}, false
	}
	return Date{}, false
}

// unescape returns a text value unescaped, without its spaces
func unescape(value string) string {
	return strings.TrimSpace(contentline.Unescape(value))
}

// first returns the first value, or an empty string
func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// pad returns the values, without their spaces, with empty values after
// them, so there are at least n
func pad(values []string, n int) []string {
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	for len(values) < n {
		values = append(values, "")
	}
	return values
}

// trimAddress returns the address without the spaces of its parts
func trimAddress(a Address) Address {
	for _, part := range []*string{&a.POBox, &a.Extended, &a.Street, &a.Locality, &a.Region, &a.PostalCode, &a.Country} {
		*part = strings.TrimSpace(*part)
	}
	return a
}
