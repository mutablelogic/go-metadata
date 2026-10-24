package ical

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

// Image is the image of a calendar or an event, which is embedded, as its
// data and its media type, such as image/png, or linked, as a URI, with how
// it's displayed, such as BADGE or GRAPHIC
// Reference: https://www.rfc-editor.org/rfc/rfc7986#section-5.10
type Image struct {
	MediaType string `json:"media_type,omitempty"`
	Data      []byte `json:"data,omitempty"`
	URI       string `json:"uri,omitempty"`
	Display   string `json:"display,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Decode returns the image, when it's embedded, decoded, or nil when it's a
// link, or can't be decoded
func (i *Image) Decode() image.Image {
	if i == nil || len(i.Data) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(i.Data))
	if err != nil {
		return nil
	}
	return img
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// mediaType returns the media type of the image, or detects it from its data
func (i *Image) mediaType() string {
	if i.MediaType != "" {
		return i.MediaType
	}
	return http.DetectContentType(i.Data)
}

// parseImage returns the image of an IMAGE property, or an ATTACH property
// which is an image, which is base64, when it's binary, with its media type,
// or a data URI, or a link, or nil when it's none of them
func parseImage(value string, base64Encoded bool, mediaType, display string) *Image {
	value = strings.TrimSpace(value)
	if rest, found := strings.CutPrefix(value, "data:"); found {
		params, payload, found := strings.Cut(rest, ",")
		if !found || !strings.HasSuffix(strings.ToLower(params), ";base64") {
			return nil
		}
		value, base64Encoded, mediaType = payload, true, params[:len(params)-len(";base64")]
	} else if !base64Encoded {
		if value == "" {
			return nil
		}
		return &Image{URI: value, Display: display}
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
	img := &Image{MediaType: strings.ToLower(strings.TrimSpace(mediaType)), Data: data, Display: display}
	img.MediaType = img.mediaType()
	return img
}
