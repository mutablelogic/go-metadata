package vcard

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	// Packages
	contentline "github.com/mutablelogic/go-metadata/internal/contentline"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Encoder writes contacts as vCard 4.0
type Encoder struct {
	w io.Writer
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// reParamValue matches a type which can be written as a parameter, such as
// home or x-mobile
var reParamValue = regexp.MustCompile(`^[a-z0-9-]+$`)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewEncoder returns an encoder of contacts to w
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Encode writes the contact as vCard 4.0. Its formatted name, which a vCard
// must have, is its display name (see Card.DisplayName). Its UID is text, as
// it may not be a URI, its URLs aren't escaped, as they're URIs, its photo
// is embedded as a data URI, or linked, and when it was last changed is in
// UTC. Values which are empty aren't written.
func (e *Encoder) Encode(c *Card) error {
	w := contentline.NewWriter(e.w)
	w.Write("BEGIN", nil, "VCARD")
	w.Write("VERSION", nil, "4.0")
	if uid := strings.TrimSpace(c.UID); uid != "" {
		w.Write("UID", []string{"VALUE=text"}, contentline.Escape(uid))
	}
	if kind := strings.ToLower(strings.TrimSpace(c.Kind)); kind != "" {
		w.Write("KIND", nil, contentline.Escape(kind))
	}

	// The formatted name, and the parts of the name
	w.Write("FN", nil, contentline.Escape(c.DisplayName()))
	if !c.Name.IsZero() {
		w.Write("N", nil, components(c.Name.Family, c.Name.Given, c.Name.Middle, c.Name.Prefix, c.Name.Suffix))
	}
	for _, nickname := range c.Nicknames {
		text(w, "NICKNAME", nil, nickname)
	}

	// How to reach it
	for _, email := range c.Emails {
		text(w, "EMAIL", params(email.Types, email.Pref), email.Value)
	}
	for _, phone := range c.Phones {
		text(w, "TEL", params(phone.Types, phone.Pref), phone.Value)
	}
	for _, url := range c.URLs {
		if value := uri(url.Value); value != "" {
			w.Write("URL", params(url.Types, url.Pref), value)
		}
	}
	for _, address := range c.Addresses {
		if address.String() != "" {
			w.Write("ADR", params(address.Types, false), components(address.POBox, address.Extended, address.Street, address.Locality, address.Region, address.PostalCode, address.Country))
		}
	}

	// Where it works, and the rest
	for _, org := range c.Orgs {
		if org.String() != "" {
			w.Write("ORG", nil, components(append([]string{org.Name}, org.Units...)...))
		}
	}
	for _, title := range c.Titles {
		text(w, "TITLE", nil, title)
	}
	if d := c.Birthday; d != nil && d.Month > 0 && d.Day > 0 {
		if d.Year > 0 {
			w.Write("BDAY", nil, fmt.Sprintf("%04d%02d%02d", d.Year, d.Month, d.Day))
		} else {
			w.Write("BDAY", nil, fmt.Sprintf("--%02d%02d", d.Month, d.Day))
		}
	}
	for _, note := range c.Notes {
		text(w, "NOTE", nil, note)
	}
	if categories := nonEmpty(c.Categories); len(categories) > 0 {
		escaped := make([]string, len(categories))
		for i, category := range categories {
			escaped[i] = contentline.Escape(category)
		}
		w.Write("CATEGORIES", nil, strings.Join(escaped, ","))
	}
	if uri := c.Photo.DataURI(); uri != "" {
		w.Write("PHOTO", nil, uri)
	}
	if !c.Rev.IsZero() {
		w.Write("REV", nil, c.Rev.UTC().Format("20060102T150405Z"))
	}
	return w.Write("END", nil, "VCARD")
}

// MarshalText returns the contact as vCard 4.0 (see Encoder.Encode)
func (c *Card) MarshalText() ([]byte, error) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// text writes a property, with its value escaped as text, unless it's empty
func text(w *contentline.Writer, name string, params []string, value string) {
	if value = strings.TrimSpace(value); value != "" {
		w.Write(name, params, contentline.Escape(value))
	}
}

// params returns the parameters of a value: its types, such as home, which
// can be parameters, as TYPE, and whether it's preferred, as PREF
func params(types []string, pref bool) []string {
	var values []string
	for _, t := range types {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" && t != "pref" && reParamValue.MatchString(t) {
			values = append(values, t)
		}
	}
	var result []string
	if len(values) > 0 {
		result = append(result, "TYPE="+strings.Join(values, ","))
	}
	if pref {
		result = append(result, "PREF=1")
	}
	return result
}

// uri returns a URI, which isn't escaped as text, as its commas and
// semicolons are part of it, but whose line breaks, which would end the
// property, are percent-encoded
func uri(value string) string {
	return strings.NewReplacer("\r", "%0D", "\n", "%0A").Replace(strings.TrimSpace(value))
}

// components returns the values escaped, and separated with semicolons, as
// the components of a name, address or organisation
func components(values ...string) string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = contentline.Escape(strings.TrimSpace(value))
	}
	return strings.Join(result, ";")
}

// nonEmpty returns the values which aren't empty, without their spaces
func nonEmpty(values []string) []string {
	var result []string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
