package meta

import (
	"bytes"
	"encoding/json"
	"image"
	"net/http"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	types "github.com/mutablelogic/go-server/pkg/types"

	// Image decoders
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// imageEntry is a metadata entry which is an image, such as artwork
type imageEntry struct {
	ns, key  string
	mimeType string
	data     []byte
	img      image.Image
}

var _ metadata.Metadata = imageEntry{}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewImage returns a metadata entry which is the encoded image data, such
// as a JPEG or PNG image, or nil when the namespace or key isn't an
// identifier, or the data isn't an image which can be decoded
func NewImage(ns, key string, data []byte) metadata.Metadata {
	if !types.IsIdentifier(ns) || !types.IsIdentifier(key) {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return imageEntry{ns: ns, key: key, mimeType: http.DetectContentType(data), data: data, img: img}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// NS returns the namespace
func (e imageEntry) NS() string {
	return e.ns
}

// Key returns the key within the namespace
func (e imageEntry) Key() string {
	return e.key
}

// Value returns the type of the image, such as image/jpeg
func (e imageEntry) Value() string {
	return e.mimeType
}

// Bytes returns the encoded image
func (e imageEntry) Bytes() []byte {
	return e.data
}

// Image returns the decoded image
func (e imageEntry) Image() image.Image {
	return e.img
}

// Any returns the decoded image
func (e imageEntry) Any() any {
	return e.img
}

// MarshalJSON describes the image, by its type, size in bytes, and
// dimensions, rather than its data
func (e imageEntry) MarshalJSON() ([]byte, error) {
	bounds := e.img.Bounds()
	return json.Marshal(struct {
		Type   string `json:"type"`
		Size   int    `json:"size"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}{e.mimeType, len(e.data), bounds.Dx(), bounds.Dy()})
}
