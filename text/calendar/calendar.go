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
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
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
	URL          string   `json:"url,omitempty"`
	Status       string   `json:"status,omitempty"`
	Organizer    string   `json:"organizer,omitempty"`
	Attendees    []string `json:"attendees,omitempty"`
	Categories   []string `json:"categories,omitempty"`

	// markdown is true when the description is Markdown, as it was HTML,
	// rather than text, which isn't stored
	markdown bool
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

	// maxValues is the most values of the events of a calendar which are
	// each key, such as its attendees
	maxValues = 100
)

var (
	reContentType = regexp.MustCompile(`^text/calendar$`)

	// categoryTypes are the types of events by their categories: those of
	// the birthdays of contacts, and the types Google Calendar gives events,
	// which omodie writes as categories
	categoryTypes = map[string]string{
		"birthday":         "birthday",
		"birthdays":        "birthday",
		"out of office":    "out-of-office",
		"focus time":       "focus-time",
		"working location": "working-location",
		"from gmail":       "from-gmail",
	}

	// reHTML matches a tag of HTML which formats text, which a description
	// written by Google Calendar has, rather than text with a placeholder in
	// angle brackets, such as Press <Enter>
	reHTML = regexp.MustCompile(`(?i)</?(a|b|blockquote|br|code|div|em|font|h[1-6]|hr|i|img|li|ol|p|pre|s|span|strike|strong|sub|sup|table|tbody|td|th|thead|tr|u|ul)(\s[^<>]*)?/?>`)
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func init() {
	// The events of a calendar, and its kind, which is an event when it's
	// a calendar of one
	metadata.AddHandler(reContentType, "ical", func(_ context.Context, r io.Reader, _ *metadata.Opts) ([]metadata.Metadata, error) {
		c, err := ical.Decode(io.LimitReader(r, maxSize))
		if err != nil {
			return metadata.KindMetadata(dc.KindCalendar), err
		}
		kind := dc.KindCalendar
		if len(c.Events) == 1 && c.Events[0].Kind == ical.KindEvent {
			kind = dc.KindEvent
		}
		return append(calendarMetadata(c), metadata.KindMetadata(kind)...), nil
	}, Namespace, dc.Namespace, metadata.MetadataNamespace)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// calendarMetadata returns the metadata of a calendar: its events and tasks
// as ical:event, and of its events, to search for, their kinds, titles,
// statuses, organizers, attendees and locations, as ical:kind, ical:title,
// ical:status, ical:organizer, ical:attendee and ical:location, each a value
// when they all have the same one, or else the different values, its name as
// ical:calendar-name, its zone as ical:tz, and when it starts and ends, as
// ical:date-start and ical:date-end (see span), which aren't known for a
// birthday whose year isn't known, or which repeats forever. Its title is
// the title of its events when they all have the same one, such as the only
// event of an invitation, or the flights of a trip, and otherwise, the
// calendar's name, or the title from the file's name. Its
// description is the calendar's, and when its only event is, where, and its
// description, or how many events it has, between which dates, and their
// titles, so they're searched for. Its only event's UID is its identifier,
// the first event's organizer its creator, and the earliest start whose
// year is known its date, all of which are of all its events. At most
// maxEvents events are stored, and when there are more, ical:note says so.
// When there's only one, its description is the calendar's, rather than
// stored again with it.
// Reference: https://www.rfc-editor.org/rfc/rfc5545
func calendarMetadata(c *ical.Calendar) []metadata.Metadata {
	// All the events, which describe the calendar, of which the first are
	// stored, with their locations
	events := make([]Event, len(c.Events))
	for i, e := range c.Events {
		events[i] = toEvent(e)
	}
	stored := events[:min(len(events), maxEvents)]

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
	if title := sameTitle(events); title != "" {
		add(dc.Namespace, dc.Title, title)
	} else {
		add(dc.Namespace, dc.Title, strings.TrimSpace(c.Name))
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
			add(Namespace, "date-start", start.UTC())
		}
		if hasEnd {
			add(Namespace, "date-end", end.UTC())
		}
	}

	// The events which are stored, without the description of the only one,
	// which is the calendar's, and a note when they aren't all of them
	if len(stored) == 1 {
		stored = slices.Clone(stored)
		stored[0].Description, stored[0].markdown = "", false
	}
	if len(stored) > 0 {
		add(Namespace, "event", stored)
	}
	if len(stored) < len(events) {
		add(Namespace, "note", fmt.Sprintf("Only the first %d of the calendar's %d events are stored in ical:event.", len(stored), len(events)))
	}
	// The kinds, titles, statuses, organizers, attendees and locations of the
	// events, the calendar's name, and its zone
	values := func(key string, fn func(Event) []string) {
		if list := distinct(events, fn); len(list) == 1 {
			add(Namespace, key, list[0])
		} else if len(list) > 1 {
			result = append(result, meta.NewText(Namespace, key, list, strings.Join(list, "; ")))
		}
	}
	values("kind", func(e Event) []string { return []string{e.Kind} })
	values("title", func(e Event) []string { return []string{e.Title} })
	values("status", func(e Event) []string { return []string{e.Status} })
	values("organizer", func(e Event) []string { return []string{e.Organizer} })
	values("attendee", func(e Event) []string { return e.Attendees })
	values("location", func(e Event) []string { return []string{e.Location} })
	values("url", func(e Event) []string { return []string{e.URL} })
	values("type", func(e Event) []string { return e.types() })
	add(Namespace, "calendar-name", strings.TrimSpace(c.Name))
	add(Namespace, "tz", strings.TrimSpace(c.TZ))
	return result
}

// types returns what an event is, as well as its kind: a task, all day,
// over more than a day, repeating, an occurrence of one which repeats, a
// birthday, which is in the Birthdays category, or whose year isn't known,
// or the type Google Calendar gives it, which is its category, such as Out
// of office
func (e Event) types() []string {
	var result []string
	add := func(value string) {
		if !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	if e.Kind == string(ical.KindTask) {
		add("task")
	}
	if e.AllDay {
		add("all-day")
	}
	if e.multiDay() {
		add("multi-day")
	}
	if e.RRule != "" {
		add("recurring")
	}
	if e.RecurrenceID != "" {
		add("occurrence")
	}
	if e.NoYear {
		add("birthday")
	}
	for _, category := range e.Categories {
		if value, exists := categoryTypes[strings.ToLower(strings.TrimSpace(category))]; exists {
			add(value)
		}
	}
	return result
}

// multiDay returns true when an event ends a day or more after it starts,
// where an all-day event's end is the day after its last
func (e Event) multiDay() bool {
	if e.AllDay {
		start, errStart := time.Parse(time.DateOnly, e.Start)
		end, errEnd := time.Parse(time.DateOnly, e.End)
		return errStart == nil && errEnd == nil && end.Sub(start) > 24*time.Hour
	}
	start, errStart := time.Parse(time.RFC3339, e.Start)
	end, errEnd := time.Parse(time.RFC3339, e.End)
	return errStart == nil && errEnd == nil && end.Sub(start) >= 24*time.Hour
}

// distinct returns the different values of the events, in the order they
// first are, without those which are empty, and at most maxValues
func distinct(events []Event, fn func(Event) []string) []string {
	var result []string
	for _, event := range events {
		for _, value := range fn(event) {
			if value = strings.TrimSpace(value); value != "" && !slices.Contains(result, value) {
				result = append(result, value)
				if len(result) == maxValues {
					return result
				}
			}
		}
	}
	return result
}

// description returns an event's description as Markdown, when it's HTML,
// such as when Google Calendar wrote it, and otherwise as it is
func description(value string) string {
	if !reHTML.MatchString(value) {
		return value
	}
	markdown, err := htmltomarkdown.ConvertString(value)
	if err != nil {
		return value
	}
	return strings.TrimSpace(strings.ReplaceAll(markdown, "\u00a0", " "))
}

// toEvent returns an event, as it's stored, from an event of a calendar
func toEvent(e *ical.Event) Event {
	result := Event{
		UID:         e.UID,
		Kind:        string(e.Kind),
		Title:       e.Summary,
		Description: description(e.Description),
		markdown:    reHTML.MatchString(e.Description),
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
		URL:         e.URL,
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
