package calendar

import (
	"slices"
	"strings"
	"time"

	// Packages
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
	rrule "github.com/teambition/rrule-go"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Occurrence is a time an event, or a task, occurs: its UID, and, of an
// event which repeats, its recurrence ID, the start it was to have, which
// identify it, as an event can be in more than one calendar, its kind, event
// or task, title, and when it starts, and, unless it's an instant, or a
// task, ends, which is exclusive, as dates, of an all-day event, or times,
// in UTC, and the zone of its times, such as Europe/Berlin, or none, of a
// date, or a time which is floating
type Occurrence struct {
	UID          string
	RecurrenceID string // As a date, 2006-01-02, or an instant, in UTC, of RFC 3339, as an Event's
	Kind         string
	Title        string
	Start        ical.Time
	End          ical.Time // Zero when it has none
	TZ           string
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Occurrences returns the times the events occur, in order, the earliest
// max of them, or every one, when it's zero: an event, or a task, which
// starts when it's due, or else of its start, of its own, and each time an event which repeats occurs, of a
// rule which ends, of its UNTIL, or COUNT, without the starts it excludes,
// and those which another of the events, of the same UID, changes, of its
// recurrence ID, which is one of its own. An event which repeats forever,
// or whose rule can't be read, is its first time, as when it ends isn't
// known, and an event which is cancelled, or a yearly event whose year
// isn't known, such as a birthday, has none, as neither does an event
// without a start.
func Occurrences(events []Event, max int) []Occurrence {
	// The occurrences which events change, of their UIDs, and recurrence IDs
	changed := make(map[string]bool)
	for _, e := range events {
		if e.RecurrenceID != "" {
			changed[e.UID+"\x00"+e.RecurrenceID] = true
		}
	}
	var result []Occurrence
	for _, e := range events {
		start, end, ok := eventTimes(e)
		if !ok || e.UID == "" || e.NoYear || strings.EqualFold(e.Status, "CANCELLED") {
			continue
		}
		occurrence := Occurrence{UID: e.UID, RecurrenceID: e.RecurrenceID, Kind: e.Kind, Title: e.Title, Start: start, End: end, TZ: e.TZ}
		if e.RRule == "" || e.RecurrenceID != "" {
			result = append(result, occurrence)
			continue
		}

		// Each time an event which repeats occurs, of its rule, when it ends,
		// and its first, when it doesn't
		// of its first, when it doesn't, without those it excludes, and those
		// which are changed
		starts, bounded := expand(e, start)
		if !bounded {
			starts = []ical.Time{start}
		}
		length := end.Time.Sub(start.Time)
		for _, at := range starts {
			id := format(at)
			if changed[e.UID+"\x00"+id] || slices.Contains(e.ExDate, id) {
				continue
			}
			occurrence.RecurrenceID, occurrence.Start = id, at
			if !end.IsZero() {
				occurrence.End = ical.Time{Time: at.Time.Add(length), Date: at.Date}
			}
			result = append(result, occurrence)
		}
	}
	slices.SortStableFunc(result, func(a, b Occurrence) int { return a.Start.Time.Compare(b.Start.Time) })
	if max > 0 && len(result) > max {
		result = result[:max]
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// maxRepeats is the most times an event which repeats occurs, which are
// returned, of its rule, such as a daily one, which ends in many years
const maxRepeats = 1000

// eventTimes returns when an event starts, and ends, which is zero when it
// has no end, or of a task, which starts when it's due, or else of its
// start, or false when it has no start, or its times can't be read
func eventTimes(e Event) (ical.Time, ical.Time, bool) {
	value := e.Start
	if e.Kind == string(ical.KindTask) && e.Due != "" {
		value = e.Due
	}
	start, ok := parseWhen(value)
	if !ok {
		return ical.Time{}, ical.Time{}, false
	}
	var end ical.Time
	if e.Kind != string(ical.KindTask) {
		if t, ok := parseWhen(e.End); ok && t.Date == start.Date && !t.Time.Before(start.Time) {
			end = t
		}
	}
	return start, end, true
}

// parseWhen returns a date, 2006-01-02, or a time, as an instant, of RFC 3339,
// in UTC, as an Event's are, or false when it's empty, or can't be read
func parseWhen(value string) (ical.Time, bool) {
	switch {
	case value == "":
		return ical.Time{}, false
	case len(value) == len(time.DateOnly):
		t, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return ical.Time{}, false
		}
		return ical.Date(t.Year(), t.Month(), t.Day()), true
	default:
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return ical.Time{}, false
		}
		return ical.UTC(t), true
	}
}

// expand returns the starts of an event which repeats, of its rule, which
// is in its zone, as a weekly meeting's day is its zone's, up to maxRepeats,
// or false when it repeats forever, or its rule can't be read
func expand(e Event, start ical.Time) ([]ical.Time, bool) {
	opt, err := rrule.StrToROption(e.RRule)
	if err != nil || (opt.Until.IsZero() && opt.Count == 0) {
		return nil, false
	}
	location := time.UTC
	if l, ok := ical.Location(e.TZ); ok && !start.Date {
		location = l
	}
	opt.Dtstart = start.Time.In(location)
	r, err := rrule.NewRRule(*opt)
	if err != nil {
		return nil, false
	}
	next := r.Iterator()
	var result []ical.Time
	for range maxRepeats {
		t, ok := next()
		if !ok {
			break
		}
		if start.Date {
			result = append(result, ical.Date(t.Year(), t.Month(), t.Day()))
		} else {
			result = append(result, ical.UTC(t))
		}
	}
	return result, true
}
