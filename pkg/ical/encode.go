package ical

import (
	"bytes"
	"encoding/base64"
	"io"
	"strconv"
	"strings"
	"time"

	// Packages
	contentline "github.com/mutablelogic/go-metadata/internal/contentline"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Encoder writes calendars as iCalendar 2.0
type Encoder struct {
	w io.Writer
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DefaultProdID is the product which writes a calendar without one
	DefaultProdID = "-//mutablelogic//go-metadata//EN"

	// The forms of a date, and of a time, in an iCalendar
	dateLayout = "20060102"
	timeLayout = "20060102T150405"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewEncoder returns an encoder of calendars to w
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Encode writes the calendar as iCalendar 2.0, with its name and
// description as Apple's and Google's X-WR-CALNAME and X-WR-CALDESC, which
// calendars read, and its events and tasks. An event's stamp, which it must
// have, is the Unix epoch when it isn't set, so a calendar is the same each
// time it's written. A time in a zone is written with its TZID, without the
// zone's definition, which calendars which know the zone don't need. Values
// which are empty aren't written.
func (e *Encoder) Encode(c *Calendar) error {
	w := contentline.NewWriter(e.w)
	w.Write("BEGIN", nil, "VCALENDAR")
	w.Write("VERSION", nil, "2.0")
	prodID := strings.TrimSpace(c.ProdID)
	if prodID == "" {
		prodID = DefaultProdID
	}
	w.Write("PRODID", nil, contentline.Escape(prodID))
	text(w, "X-WR-CALNAME", nil, c.Name)
	text(w, "X-WR-CALDESC", nil, c.Description)
	text(w, "X-WR-TIMEZONE", nil, c.TZ)
	writeImage(w, c.Image)
	for _, event := range c.Events {
		writeEvent(w, event)
	}
	return w.Write("END", nil, "VCALENDAR")
}

// MarshalText returns the calendar as iCalendar 2.0 (see Encoder.Encode)
func (c *Calendar) MarshalText() ([]byte, error) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// writeEvent writes an event, as a VEVENT, or a task, as a VTODO
func writeEvent(w *contentline.Writer, e *Event) {
	component := "VEVENT"
	if e.Kind == KindTask {
		component = "VTODO"
	}
	w.Write("BEGIN", nil, component)
	text(w, "UID", nil, e.UID)
	stamp := e.Stamp
	if stamp.IsZero() {
		stamp = time.Unix(0, 0)
	}
	w.Write("DTSTAMP", nil, stamp.UTC().Format(timeLayout+"Z"))
	writeTime(w, "DTSTART", e.Start)
	if e.Kind == KindTask {
		writeTime(w, "DUE", e.Due)
	} else {
		writeTime(w, "DTEND", e.End)
	}
	if rrule := strings.TrimSpace(e.RRule); rrule != "" {
		w.Write("RRULE", nil, rrule)
	}
	for _, exdate := range e.ExDates {
		writeTime(w, "EXDATE", exdate)
	}
	writeTime(w, "RECURRENCE-ID", e.RecurrenceID)
	text(w, "SUMMARY", nil, e.Summary)
	text(w, "DESCRIPTION", nil, e.Description)
	text(w, "LOCATION", nil, e.Location)
	if e.Geo != nil && e.Geo.Validate() == nil {
		w.Write("GEO", nil, strconv.FormatFloat(e.Geo.Lat, 'f', -1, 64)+";"+strconv.FormatFloat(e.Geo.Long, 'f', -1, 64))
	}
	if url := strings.TrimSpace(e.URL); url != "" {
		w.Write("URL", nil, strings.NewReplacer("\r", "%0D", "\n", "%0A").Replace(url))
	}
	text(w, "STATUS", nil, strings.ToUpper(e.Status))
	if organizer := strings.TrimSpace(e.Organizer); organizer != "" {
		w.Write("ORGANIZER", nil, "mailto:"+organizer)
	}
	for _, attendee := range e.Attendees {
		if attendee = strings.TrimSpace(attendee); attendee != "" {
			w.Write("ATTENDEE", nil, "mailto:"+attendee)
		}
	}
	if categories := nonEmpty(e.Categories); len(categories) > 0 {
		escaped := make([]string, len(categories))
		for i, category := range categories {
			escaped[i] = contentline.Escape(category)
		}
		w.Write("CATEGORIES", nil, strings.Join(escaped, ","))
	}
	if e.Transparent {
		w.Write("TRANSP", nil, "TRANSPARENT")
	}
	if !e.Created.IsZero() {
		w.Write("CREATED", nil, e.Created.UTC().Format(timeLayout+"Z"))
	}
	if !e.Modified.IsZero() {
		w.Write("LAST-MODIFIED", nil, e.Modified.UTC().Format(timeLayout+"Z"))
	}
	writeImage(w, e.Image)
	if e.OmitYear > 0 {
		w.Write("X-APPLE-OMIT-YEAR", nil, strconv.Itoa(e.OmitYear))
	}
	w.Write("END", nil, component)
}

// writeTime writes a date, with VALUE=DATE, or a time, in UTC, or in a zone,
// with its TZID, or floating, unless there's no time
func writeTime(w *contentline.Writer, name string, t Time) {
	switch {
	case t.IsZero():
	case t.Date:
		w.Write(name, []string{"VALUE=DATE"}, t.Time.Format(dateLayout))
	case t.Floating:
		w.Write(name, nil, t.Time.Format(timeLayout))
	case t.TZID != "":
		w.Write(name, []string{"TZID=" + t.TZID}, t.Time.Format(timeLayout))
	default:
		w.Write(name, nil, t.Time.UTC().Format(timeLayout+"Z"))
	}
}

// writeImage writes an image which is embedded, as base64, with its media
// type, or linked, with how it's displayed
func writeImage(w *contentline.Writer, i *Image) {
	switch {
	case i == nil:
	case len(i.Data) > 0:
		params := []string{"VALUE=BINARY", "ENCODING=BASE64", "FMTTYPE=" + i.mediaType()}
		if i.Display != "" {
			params = append(params, "DISPLAY="+strings.ToUpper(i.Display))
		}
		w.Write("IMAGE", params, base64.StdEncoding.EncodeToString(i.Data))
	case i.URI != "":
		params := []string{"VALUE=URI"}
		if i.Display != "" {
			params = append(params, "DISPLAY="+strings.ToUpper(i.Display))
		}
		w.Write("IMAGE", params, i.URI)
	}
}

// text writes a property, with its value escaped as text, unless it's empty
func text(w *contentline.Writer, name string, params []string, value string) {
	if value = strings.TrimSpace(value); value != "" {
		w.Write(name, params, contentline.Escape(value))
	}
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
