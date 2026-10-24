package vcard

import (
	"bytes"
	"encoding/base64"
	"image"
	"net/http"
	"strings"

	// Image decoders
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Photo is the photo of a contact, which is embedded in its vCard, as its
// data and its media type, such as image/jpeg, or linked, as a URI
type Photo struct {
	MediaType string `json:"media_type,omitempty"`
	Data      []byte `json:"data,omitempty"`
	URI       string `json:"uri,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Image returns the photo, when it's embedded, decoded, or nil when it's a
// link, or can't be decoded
func (p *Photo) Image() image.Image {
	if p == nil || len(p.Data) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(p.Data))
	if err != nil {
		return nil
	}
	return img
}

// DataURI returns the photo as a data URI, when it's embedded, such as
// data:image/jpeg;base64,..., or else its URI
func (p *Photo) DataURI() string {
	if p == nil {
		return ""
	} else if len(p.Data) == 0 {
		return p.URI
	}
	mediaType := p.MediaType
	if mediaType == "" {
		mediaType = http.DetectContentType(p.Data)
	}
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(p.Data)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// parsePhoto returns the photo of a PHOTO property's value: a data URI, as
// vCard 4.0 has, base64, as vCard 2.1 and 3.0 have when the encoding is set,
// with the type, such as JPEG, or a link, or nil when it's none of them
func parsePhoto(value string, base64Encoded bool, imageType string) *Photo {
	value = strings.TrimSpace(value)
	if rest, found := strings.CutPrefix(value, "data:"); found {
		params, payload, found := strings.Cut(rest, ",")
		if !found || !strings.HasSuffix(strings.ToLower(params), ";base64") {
			return nil
		}
		value, imageType = payload, params[:len(params)-len(";base64")]
	} else if !base64Encoded {
		if value == "" {
			return nil
		}
		return &Photo{URI: value}
	}
	data, err := base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, value))
	if err != nil || len(data) == 0 {
		return nil
	}
	return &Photo{MediaType: mediaType(imageType, data), Data: data}
}

// mediaType returns the media type of a photo, from its type, which is a
// media type, such as image/jpeg, or a type, such as JPEG, or else detected
// from its data
func mediaType(imageType string, data []byte) string {
	imageType = strings.ToLower(strings.TrimSpace(imageType))
	switch {
	case strings.Contains(imageType, "/"):
		return imageType
	case imageType != "":
		return "image/" + imageType
	default:
		return http.DetectContentType(data)
	}
}
