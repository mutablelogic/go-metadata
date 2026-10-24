package calendar

import (
	"strconv"
	"strings"
	"time"

	// Packages
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
)

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// span returns when a calendar starts, which is the earliest start of its
// events whose year is known, so a birthday whose year isn't known doesn't
// start it, and when it ends, which is the latest end of its events, or of
// the last occurrence of an event which repeats, and isn't known when an
// event repeats forever, such as a birthday. Dates are midnight in UTC, and
// the end is exclusive, as an event's is.
func span(events []*ical.Event) (start, end time.Time, hasStart, hasEnd bool) {
	forever := false
	for _, e := range events {
		if e.Start.IsZero() {
			continue
		}
		if e.OmitYear == 0 && (!hasStart || e.Start.Time.Before(start)) {
			start, hasStart = e.Start.Time, true
		}
		last, ok := lastEnd(e)
		if !ok {
			forever = true
			continue
		}
		if !hasEnd || last.After(end) {
			end, hasEnd = last, true
		}
	}
	if forever {
		end, hasEnd = time.Time{}, false
	}
	return start, end, hasStart, hasEnd
}

// lastEnd returns the end of an event, or of the last occurrence of an
// event which repeats: the end of its rule's UNTIL, and its length, or of
// its COUNT of occurrences, which is the start of the last of its periods,
// such as weeks, and its length, which is when it ends at the latest, as a
// rule can have more than one occurrence in a period. Returns false when
// it repeats forever.
func lastEnd(e *ical.Event) (time.Time, bool) {
	end := e.End.Time
	if end.Before(e.Start.Time) {
		end = e.Start.Time
	}
	if strings.TrimSpace(e.RRule) == "" {
		return end, true
	}
	length := end.Sub(e.Start.Time)

	rule := make(map[string]string)
	for _, part := range strings.Split(e.RRule, ";") {
		if key, value, found := strings.Cut(part, "="); found {
			rule[strings.ToUpper(strings.TrimSpace(key))] = strings.ToUpper(strings.TrimSpace(value))
		}
	}
	location := e.Start.Time.Location()
	if until := rule["UNTIL"]; until != "" {
		if t, ok := parseUntil(until, location); ok {
			if e.Start.Date {
				// The last day, which is all of it
				return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Add(length), true
			}
			return t.Add(length), true
		}
	}
	count, err := strconv.Atoi(rule["COUNT"])
	if err != nil || count < 1 {
		return time.Time{}, false
	}
	interval, err := strconv.Atoi(rule["INTERVAL"])
	if err != nil || interval < 1 {
		interval = 1
	}
	n := (count - 1) * interval
	last := e.Start.Time
	switch rule["FREQ"] {
	case "SECONDLY":
		last = last.Add(time.Duration(n) * time.Second)
	case "MINUTELY":
		last = last.Add(time.Duration(n) * time.Minute)
	case "HOURLY":
		last = last.Add(time.Duration(n) * time.Hour)
	case "DAILY":
		last = last.AddDate(0, 0, n)
	case "WEEKLY":
		last = last.AddDate(0, 0, 7*n)
	case "MONTHLY":
		last = last.AddDate(0, n, 0)
	case "YEARLY":
		last = last.AddDate(n, 0, 0)
	default:
		return time.Time{}, false
	}
	return last.Add(length), true
}
