// Package iptc reads IPTC metadata, in the Information Interchange Model
// (IIM), which is a sequence of datasets, such as the title, keywords and
// caption of a photo, in a JPEG image or a Photoshop document
package iptc

import (
	"encoding/binary"
	"strings"
	"time"
	"unicode/utf8"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	charmap "golang.org/x/text/encoding/charmap"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Dataset is a dataset of IPTC metadata, which is a record, such as 2 for
// the application record, a number, such as 5 for the object name, and its
// data
type Dataset struct {
	Record uint8
	Number uint8
	Data   []byte
}

// field is a dataset of the application record, as metadata, with its key,
// and whether it can be repeated, such as keywords
type field struct {
	key      string
	repeated bool
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of IPTC metadata
	Namespace = "iptc"

	// tagMarker starts each dataset
	tagMarker = 0x1C

	// The records of the envelope, which has the character set, and of the
	// application, which has the metadata of the photo
	recordEnvelope    = 1
	recordApplication = 2

	// datasetCharset is the dataset of the envelope which is the character
	// set of the text, which is UTF-8 when it's utf8Charset
	datasetCharset = 90
	utf8Charset    = "\x1b%G"

	// The datasets of the date and time the photo was created
	datasetDateCreated = 55
	datasetTimeCreated = 60
)

// fields are the datasets of the application record which are metadata
var fields = map[uint8]field{
	5:   {"ObjectName", false},
	15:  {"Category", false},
	20:  {"SupplementalCategories", true},
	25:  {"Keywords", true},
	40:  {"SpecialInstructions", false},
	80:  {"Byline", true},
	85:  {"BylineTitle", true},
	90:  {"City", false},
	92:  {"Sublocation", false},
	95:  {"ProvinceState", false},
	100: {"CountryCode", false},
	101: {"CountryName", false},
	103: {"OriginalTransmissionReference", false},
	105: {"Headline", false},
	110: {"Credit", false},
	115: {"Source", false},
	116: {"CopyrightNotice", false},
	118: {"Contact", true},
	120: {"Caption", false},
	122: {"CaptionWriter", true},
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Parse returns the datasets of IPTC metadata, stopping at data which isn't
// a dataset, or a dataset which is cut short, so the datasets before it are
// returned
func Parse(data []byte) []Dataset {
	var result []Dataset
	for len(data) >= 5 && data[0] == tagMarker {
		record, number := data[1], data[2]
		length, header := int(binary.BigEndian.Uint16(data[3:])), 5

		// An extended dataset has the size of its length, up to four bytes
		if length&0x8000 != 0 {
			size := length & 0x7FFF
			if size > 4 || len(data) < header+size {
				break
			}
			length = 0
			for _, b := range data[header : header+size] {
				length = length<<8 | int(b)
			}
			header += size
		}
		if length < 0 || len(data) < header+length {
			break
		}
		result = append(result, Dataset{Record: record, Number: number, Data: data[header : header+length]})
		data = data[header+length:]
	}
	return result
}

// Metadata returns the datasets of the application record as metadata, in
// the iptc namespace, such as iptc:ObjectName and iptc:Keywords, which is a
// list when a dataset can be repeated, and the date and time the photo was
// created, as iptc:DateCreated. The text is UTF-8 when the envelope says
// so, or it's valid UTF-8, and otherwise ISO 8859-1.
func Metadata(datasets []Dataset) []metadata.Metadata {
	// The character set of the text
	isUTF8 := false
	for _, d := range datasets {
		if d.Record == recordEnvelope && d.Number == datasetCharset && string(d.Data) == utf8Charset {
			isUTF8 = true
		}
	}
	text := func(data []byte) string {
		if !isUTF8 && !utf8.Valid(data) {
			if decoded, err := charmap.ISO8859_1.NewDecoder().Bytes(data); err == nil {
				data = decoded
			}
		}
		return strings.TrimSpace(strings.TrimRight(string(data), "\x00"))
	}

	// The fields, in their order, with the values of those which are
	// repeated together
	var order []uint8
	values := make(map[uint8][]string)
	var date, clock string
	for _, d := range datasets {
		if d.Record != recordApplication {
			continue
		}
		switch d.Number {
		case datasetDateCreated:
			date = text(d.Data)
			continue
		case datasetTimeCreated:
			clock = text(d.Data)
			continue
		}
		f, exists := fields[d.Number]
		if !exists {
			continue
		}
		value := text(d.Data)
		if value == "" {
			continue
		}
		if _, exists := values[d.Number]; !exists {
			order = append(order, d.Number)
		} else if !f.repeated {
			continue
		}
		values[d.Number] = append(values[d.Number], value)
	}

	var result []metadata.Metadata
	for _, number := range order {
		f := fields[number]
		if f.repeated {
			result = append(result, meta.NewText(Namespace, f.key, values[number], strings.Join(values[number], "; ")))
		} else {
			result = append(result, meta.New(Namespace, f.key, values[number][0]))
		}
	}
	if t, ok := parseDateTime(date, clock); ok {
		result = append(result, meta.New(Namespace, "DateCreated", t))
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// parseDateTime returns the date and time of IPTC metadata, which is a date
// as CCYYMMDD, and a time as HHMMSS, with its UTC offset, as ±HHMM, or in
// UTC without one, or false when there's no date, or it isn't valid
func parseDateTime(date, clock string) (time.Time, bool) {
	if date == "" {
		return time.Time{}, false
	} else if clock == "" {
		t, err := time.Parse("20060102", date)
		return t, err == nil
	}
	for _, layout := range []string{"20060102150405-0700", "20060102150405"} {
		if t, err := time.Parse(layout, date+clock); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
