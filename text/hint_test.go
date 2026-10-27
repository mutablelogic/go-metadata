package text_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"slices"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	_ "github.com/mutablelogic/go-metadata/message"
	_ "github.com/mutablelogic/go-metadata/text"
)

// hintsOf returns the hints of the data of the type, when it's indexed,
// without namespaces
func hintsOf(t *testing.T, data, contentType string) []string {
	t.Helper()
	_, meta, err := metadata.Extract(context.Background(), bytes.NewReader([]byte(data)), metadata.WithContentType(contentType))
	if err != nil {
		t.Fatalf("%s: %v", contentType, err)
	}
	for _, m := range meta {
		if m.NS() == metadata.MetadataNamespace && m.Key() == metadata.HintKey {
			hints, ok := m.Any().([]string)
			if !ok {
				t.Fatalf("%s: expected the hints as a list, got %T", contentType, m.Any())
			}
			return hints
		}
	}
	return nil
}

func Test_hint_000(t *testing.T) {
	// Text, source code, data and documents which have text can be read as
	// Markdown, and those which don't, can't, and GeoJSON has places
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n"))
	for _, test := range []struct {
		name, contentType, data string
		want                    []string
	}{
		{"plain", "text/plain", "Hello", []string{"markdown"}},
		{"plain empty", "text/plain", " \n", nil},
		{"plain binary", "text/plain", "a\x00b", nil},
		{"other text", "text/x-log", "a line", []string{"markdown"}},
		{"source code", "text/x-go", "package main", []string{"markdown"}},
		{"geojson", "application/geo+json", `{"type":"Point","coordinates":[0,51.5]}`, []string{"geo", "markdown"}},
		{"markdown", "text/markdown", "# Notes", []string{"markdown"}},
		{"csv", "text/csv", "name,size\nada,1\n", []string{"markdown"}},
		{"csv empty", "text/csv", "", nil},
		{"html", "text/html", "<html><body><p>Text</p></body></html>", []string{"markdown"}},
		{"html empty", "text/html", "<html><head><title>Title</title></head><body></body></html>", nil},
		{"xml", "application/xml", "<a>text</a>", []string{"markdown"}},
		{"calendar", "text/calendar", "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:a\r\nDTSTART:20261026T100000Z\r\nSUMMARY:Meeting\r\nLOCATION:London\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", []string{"event", "geo", "markdown"}},
		{"calendar image", "text/calendar", "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:a\r\nDTSTART:20261026T100000Z\r\nIMAGE;VALUE=BINARY;ENCODING=BASE64;FMTTYPE=image/png:" + png + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", []string{"artwork", "event", "markdown"}},
		{"calendar empty", "text/calendar", "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", nil},
		{"contact", "text/vcard", "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Ada\r\nEND:VCARD\r\n", []string{"markdown"}},
		{"contact details", "text/vcard", "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Ada\r\nBDAY:--1210\r\nADR:;;12 St James's Square;London;;SW1Y 4JH;UK\r\nPHOTO:data:image/png;base64," + png + "\r\nEND:VCARD\r\n", []string{"artwork", "event", "geo", "markdown"}},
		{"email", "message/rfc822", "From: ada@example.com\r\nSubject: Notes\r\n\r\nHello\r\n", []string{"markdown"}},
		{"invitation", "message/rfc822", "From: ada@example.com\r\nSubject: Meeting\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\nContent-Type: text/plain\r\n\r\nCome\r\n--b\r\nContent-Type: text/calendar; method=REQUEST\r\nContent-Disposition: attachment; filename=invite.ics\r\n\r\nBEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n--b--\r\n", []string{"event", "markdown"}},
		{"mailbox", "application/mbox", "From ada@example.com Mon Feb 11 13:12:43 2002\nSubject: One\n\nHi\n", []string{"markdown"}},
		{"mailbox empty", "application/mbox", "", nil},
	} {
		if got := hintsOf(t, test.data, test.contentType); !slices.Equal(got, test.want) {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, got)
		}
	}
}

// tagsOf returns the tags of the data of the type, when it's indexed
func tagsOf(t *testing.T, data, contentType string) []string {
	t.Helper()
	_, meta, err := metadata.Extract(context.Background(), bytes.NewReader([]byte(data)), metadata.WithContentType(contentType))
	if err != nil {
		t.Fatalf("%s: %v", contentType, err)
	}
	for _, m := range meta {
		if m.NS() == metadata.MetadataNamespace && m.Key() == metadata.TagKey {
			tags, _ := m.Any().([]string)
			return tags
		}
	}
	return nil
}

func Test_tag_000(t *testing.T) {
	// A calendar of events is tagged event, and with what its events are,
	// such as a birthday, and an email with an invitation is tagged event,
	// as is a mailbox with one, but text isn't tagged
	for _, test := range []struct {
		name, contentType, data string
		want                    []string
	}{
		{"meeting", "text/calendar", "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:a\r\nDTSTART:20261026T100000Z\r\nDTEND:20261026T110000Z\r\nSUMMARY:Meeting\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", []string{"event"}},
		{"birthday", "text/calendar", "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:b\r\nDTSTART;VALUE=DATE:20261210\r\nDTEND;VALUE=DATE:20261211\r\nRRULE:FREQ=YEARLY\r\nCATEGORIES:Birthday\r\nSUMMARY:Ada\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", []string{"all-day", "birthday", "event", "recurring"}},
		{"empty calendar", "text/calendar", "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", nil},
		{"email", "message/rfc822", "From: ada@example.com\r\nSubject: Notes\r\n\r\nHello\r\n", nil},
		{"invitation", "message/rfc822", "From: ada@example.com\r\nSubject: Meeting\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\nContent-Type: text/plain\r\n\r\nCome\r\n--b\r\nContent-Type: text/calendar; method=REQUEST\r\nContent-Disposition: attachment; filename=invite.ics\r\n\r\nBEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n--b--\r\n", []string{"event"}},
		{"plain", "text/plain", "Hello", nil},
	} {
		if got := tagsOf(t, test.data, test.contentType); !slices.Equal(got, test.want) {
			t.Errorf("%s: expected %v, got %v", test.name, test.want, got)
		}
	}
}
