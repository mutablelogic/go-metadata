// Package meta provides metadata entries for the handlers of content types
package meta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// entry is a metadata entry with a key and a value, and the text of the
// value, when it's formatted, such as "f/2.8" for a number
type entry struct {
	ns    string
	key   string
	value any
	text  string
}

var _ metadata.Metadata = entry{}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	Separator = metadata.Separator
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a metadata entry
func New(ns, key string, value any) metadata.Metadata {
	if !types.IsIdentifier(ns) || !types.IsIdentifier(key) {
		return nil
	}
	return entry{ns: ns, key: key, value: value}
}

// NewText returns a metadata entry whose value is formatted as the text,
// such as an f-number of 2.8 as "f/2.8", which is its Value, rather than
// the value itself, which is its Any, and in JSON. The text is the value,
// formatted as New does, when it's empty.
func NewText(ns, key string, value any, text string) metadata.Metadata {
	if !types.IsIdentifier(ns) || !types.IsIdentifier(key) {
		return nil
	}
	return entry{ns: ns, key: key, value: value, text: text}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// NS returns the namespace
func (e entry) NS() string {
	return e.ns
}

// Key returns the key within the namespace
func (e entry) Key() string {
	return e.key
}

// Value returns the value as a string, which is its text when it has one,
// RFC3339 for a time, and a string for duration
func (e entry) Value() string {
	if e.text != "" {
		return e.text
	}
	switch v := e.value.(type) {
	case string:
		return v
	case time.Time:
		return v.Format(time.RFC3339)
	case time.Duration:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

// Bytes returns the value as a string, as bytes
func (e entry) Bytes() []byte {
	return []byte(e.Value())
}

// Image returns nil, as the value isn't an image
func (e entry) Image() image.Image {
	return nil
}

// Any returns the value
func (e entry) Any() any {
	return e.value
}

// JSON marshaller for the metadata entry's value, which is the string form
// for a duration, without escaping the characters which are HTML's, such as
// &, so they're as they are
func (e entry) MarshalJSON() ([]byte, error) {
	var value any = e.value
	if v, ok := e.value.(time.Duration); ok {
		value = v.String()
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
