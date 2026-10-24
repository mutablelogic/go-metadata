// Package calendar registers handlers of iCalendars, calendars of events and
// tasks, which return their events, which are searched for by when and where
// they are, a description of them in words, and their image as artwork
package calendar

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	dc "github.com/mutablelogic/go-metadata/dublincore"
	meta "github.com/mutablelogic/go-metadata/internal/meta"
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Event is an event, or a task, of an iCalendar, which is stored as
// ical:event. Its start and end are dates, as 2006-01-02, when it's all day,
// or instants in UTC, as RFC 3339, when it's timed, where the end is
// exclusive. A timed event in a zone is converted to UTC with its zone, and
// one without a zone, or in a zone which isn't known, is treated as UTC, and
// has no zone. A repeating event has its rule and the starts it excludes,
// as they're written, and an occurrence of it which is changed has the start
// it replaces as its recurrence ID. A task has when it's due.
type Event struct {
	UID          string   `json:"uid,omitempty"`
	Kind         string   `json:"kind"` // event, or task
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	Location     string   `json:"location,omitempty"`
	Start        string   `json:"start,omitempty"`
	End          string   `json:"end,omitempty"`
	Due          string   `json:"due,omitempty"` // when a task is due, as a date or an instant
	AllDay       bool     `json:"all_day,omitempty"`
	TZ           string   `json:"tz,omitempty"`      // the zone of a timed event, or UTC
	NoYear       bool     `json:"no_year,omitempty"` // a yearly event whose first year isn't known, such as a birthday
	RRule        string   `json:"rrule,omitempty"`
	ExDate       []string `json:"exdate,omitempty"`
	RecurrenceID string   `json:"recurrence_id,omitempty"`
	Status       string   `json:"status,omitempty"`
	Organizer    string   `json:"organizer,omitempty"`
	Attendees    []string `json:"attendees,omitempty"`
	Categories   []string `json:"categories,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Namespace is the namespace of the events of a calendar
	Namespace = "ical"

	// maxSize is the most of an iCalendar file which is read, as an export
	// of a calendar can be large
	maxSize = 64 << 20

	// maxEvents is the most events of a calendar which are stored
	maxEvents = 1000
)

var (
	reContentType = regexp.MustCompile(`^text/calendar$`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The events of a calendar, and its kind
	metadata.AddHandler(reContentType, "ical", func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		kind := metadata.KindMetadata(dc.KindCalendar)
		c, err := ical.Decode(io.LimitReader(r, maxSize))
		if err != nil {
			return kind, err
		}
		return append(calendarMetadata(c), kind...), nil
	}, Namespace, dc.Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// calendarMetadata returns the metadata of a calendar: its events and tasks
// as ical:event, the locations of its events, to search for, as
// ical:location, its zone as ical:tz, and when it starts and ends, as
// ical:start and ical:end (see span), which aren't known for a birthday
// whose year isn't known, or which repeats forever. Its title is the calendar's name,
// or the title of its events when they all have the same one, such as the
// flights of a trip, and otherwise, the title from the file's name. Its
// description is the calendar's, and when its only event is, where, and its
// description, or how many events it has, between which dates, and their
// titles, so they're searched for. Its only event's UID is its identifier,
// the first event's organizer its creator, and the earliest start whose
// year is known its date, all of which are of all its events. At most
// maxEvents events are stored, and when there are more, ical:note says so.
// Reference: https://www.rfc-editor.org/rfc/rfc5545
func calendarMetadata(c *ical.Calendar) []metadata.Metadata {
	// All the events, which describe the calendar, of which the first are
	// stored, with their locations
	events := make([]Event, len(c.Events))
	for i, e := range c.Events {
		events[i] = toEvent(e)
	}
	stored := events[:min(len(events), maxEvents)]
	var locations []string
	for _, event := range stored {
		if event.Location != "" && !slices.Contains(locations, event.Location) {
			locations = append(locations, event.Location)
		}
	}

	var result []metadata.Metadata
	add := func(ns, key string, value any) {
		if s, ok := value.(string); ok && s == "" {
			return
		}
		if entry := meta.New(ns, key, value); entry != nil {
			result = append(result, entry)
		}
	}

	// The title, description and identifier
	if name := strings.TrimSpace(c.Name); name != "" {
		add(dc.Namespace, dc.Title, name)
	} else {
		add(dc.Namespace, dc.Title, sameTitle(events))
	}
	add(dc.Namespace, dc.Description, describe(c.Description, events))
	if len(events) == 1 {
		add(dc.Namespace, dc.Identifier, events[0].UID)
	}
	if len(events) > 0 {
		add(dc.Namespace, dc.Creator, events[0].Organizer)
	}
	add(dc.Namespace, dc.Date, earliest(events))

	// When the calendar starts and ends, which aren't known for a birthday
	// whose year isn't known, or which repeats forever
	if start, end, hasStart, hasEnd := span(c.Events); hasStart || hasEnd {
		if hasStart {
			add(Namespace, "start", start.UTC())
		}
		if hasEnd {
			add(Namespace, "end", end.UTC())
		}
	}

	// The events which are stored, and a note when they aren't all of them,
	// and their locations, and the calendar's zone
	if len(stored) > 0 {
		add(Namespace, "event", stored)
	}
	if len(stored) < len(events) {
		add(Namespace, "note", fmt.Sprintf("Only the first %d of the calendar's %d events are stored in ical:event.", len(stored), len(events)))
	}
	if len(locations) > 0 {
		result = append(result, meta.NewText(Namespace, "location", locations, strings.Join(locations, "; ")))
	}
	add(Namespace, "tz", strings.TrimSpace(c.TZ))
	return result
}

// toEvent returns an event, as it's stored, from an event of a calendar
func toEvent(e *ical.Event) Event {
	result := Event{
		UID:         e.UID,
		Kind:        string(e.Kind),
		Title:       e.Summary,
		Description: e.Description,
		Location:    e.Location,
		Start:       format(e.Start),
		End:         format(e.End),
		Due:         format(e.Due),
		AllDay:      e.Start.Date,
		TZ:          zone(e.Start),
		NoYear:      e.OmitYear > 0,
		RRule:       e.RRule,
		Status:      e.Status,
		Organizer:   e.Organizer,
		Attendees:   e.Attendees,
		Categories:  e.Categories,
	}
	for _, exdate := range e.ExDates {
		result.ExDate = append(result.ExDate, format(exdate))
	}
	result.RecurrenceID = format(e.RecurrenceID)
	return result
}

// format returns a date as 2006-01-02, or a time as an instant in UTC, as
// RFC 3339, or an empty string when there's none
func format(t ical.Time) string {
	switch {
	case t.IsZero():
		return ""
	case t.Date:
		return t.Time.Format(time.DateOnly)
	default:
		return t.Time.UTC().Format(time.RFC3339)
	}
}

// zone returns the zone of a timed event: UTC, when it's in UTC, the name of
// its zone, such as Europe/Berlin or +02:00, or none when it's floating, or
// in a zone which isn't known, or all day
func zone(t ical.Time) string {
	switch {
	case t.Date, t.Floating:
		return ""
	case t.TZID == "":
		return "UTC"
	}
	if location, ok := ical.Location(t.TZID); ok {
		return location.String()
	}
	return ""
}

// sameTitle returns the title of the events when they all have the same one
func sameTitle(events []Event) string {
	if len(events) == 0 {
		return ""
	}
	for _, event := range events[1:] {
		if event.Title != events[0].Title {
			return ""
		}
	}
	return events[0].Title
}

// earliest returns the earliest start of the events, which are dates or
// instants, compared as dates and times, or "" when there are none. An event
// whose year isn't known, such as a birthday in 1604, isn't one.
func earliest(events []Event) string {
	var result string
	var at time.Time
	for _, event := range events {
		if event.NoYear {
			continue
		}
		t, err := time.Parse(time.RFC3339, event.Start)
		if err != nil {
			if t, err = time.Parse(time.DateOnly, event.Start); err != nil {
				continue
			}
		}
		if result == "" || t.Before(at) {
			result, at = event.Start, t
		}
	}
	return result
}

// nonEmpty returns the values which aren't empty
func nonEmpty(values []string) []string {
	var result []string
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
