package calendar

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	// Packages
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// maxTitles is the most of the titles of a calendar's events which are
// in its description
const maxTitles = 1000

// weekdays are the names of the days of a rule
var weekdays = map[string]string{
	"MO": "Monday", "TU": "Tuesday", "WE": "Wednesday", "TH": "Thursday",
	"FR": "Friday", "SA": "Saturday", "SU": "Sunday",
}

// units are the units of a rule's frequencies
var units = map[string]string{
	"DAILY": "day", "WEEKLY": "week", "MONTHLY": "month", "YEARLY": "year",
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// describe returns the description of a calendar: its own, and when
// its only event is, where, and its description, or how many events it has,
// between which dates, and their titles
func describe(description string, events []Event) string {
	var lines []string
	if description != "" {
		lines = append(lines, description)
	}
	switch len(events) {
	case 0:
	case 1:
		event := events[0]
		lines = append(lines, nonEmpty([]string{eventWhen(event), event.Location, event.Description})...)
	default:
		lines = append(lines, summarize(events))
	}
	return strings.Join(lines, "\n")
}

// summarize returns how many events there are, between which dates, and
// their titles, which are cut short when they're long
func summarize(events []Event) string {
	// The events are tasks when they're all tasks
	noun := "tasks"
	for _, event := range events {
		if event.Kind != "task" {
			noun = "events"
			break
		}
	}
	summary := fmt.Sprintf("%d %s", len(events), noun)

	// The dates of the first and last starts, of those whose year is known,
	// as they're shown, in their zones, compared as dates
	var first, last time.Time
	for _, event := range events {
		start, _, _, ok := times(event)
		if !ok || event.NoYear {
			continue
		}
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		if first.IsZero() || start.Before(first) {
			first = start
		}
		if last.IsZero() || start.After(last) {
			last = start
		}
	}
	switch {
	case first.IsZero():
	case formatDate(first) == formatDate(last):
		summary += ", on " + formatDate(first)
	default:
		summary += ", from " + formatDate(first) + " to " + formatDate(last)
	}

	// Their titles, once each, cut short
	var titles []string
	seen := make(map[string]bool)
	for _, event := range events {
		if event.Title != "" && !seen[event.Title] {
			seen[event.Title] = true
			titles = append(titles, event.Title)
		}
	}
	if list := strings.Join(titles, ", "); list != "" {
		if len(list) > maxTitles {
			list = strings.TrimRight(strings.ToValidUTF8(list[:maxTitles], ""), ", ") + "…"
		}
		summary += ": " + list
	}
	return summary
}

// eventWhen returns when an event is, in words, in its zone when it's known,
// or UTC when it's timed: its date or dates when it's all day, or its date
// and times, or how it repeats, or when a task is due, and whether it's
// cancelled
func eventWhen(event Event) string {
	start, end, location, ok := times(event)
	if !ok {
		return ""
	}
	var when string
	switch {
	case event.RRule != "":
		when = repeats(event, start, end, location)
	case event.Kind == "task":
		due := start
		if t, ok := parse(event.Due, location); ok {
			due = t
		}
		when = "Due " + formatDate(due)
		if !event.AllDay {
			when += ", " + due.Format("15:04 MST")
		}
	case event.AllDay:
		when = formatDays(start, end)
	default:
		when = formatTimes(start, end)
	}
	if strings.EqualFold(event.Status, "CANCELLED") {
		when = "Cancelled: " + when
	}
	return when
}

// repeats returns how a repeating event repeats, in words, for the
// simple rules: every day, week, month or year, or every number of them, on
// days of the week, until a date, or a number of times. Other rules are
// described by their frequency.
func repeats(event Event, start, end time.Time, location *time.Location) string {
	rule := make(map[string]string)
	for _, part := range strings.Split(event.RRule, ";") {
		if key, value, found := strings.Cut(part, "="); found {
			rule[strings.ToUpper(strings.TrimSpace(key))] = strings.ToUpper(strings.TrimSpace(value))
		}
	}
	unit, ok := units[rule["FREQ"]]
	if !ok {
		return "Repeats, from " + formatDate(start)
	}

	// Every unit, or every number of units
	interval, _ := strconv.Atoi(rule["INTERVAL"])
	every := "Every " + unit
	if interval > 1 {
		every = fmt.Sprintf("Every %d %ss", interval, unit)
	}

	// The days, which are the start's, or the weekdays of a weekly rule.
	// Other parts, apart from a yearly event's month and day, as a
	// birthday's, are described by the frequency alone.
	simple := true
	for key := range rule {
		switch key {
		case "FREQ", "INTERVAL", "UNTIL", "COUNT", "WKST":
		case "BYDAY":
			simple = simple && rule["FREQ"] == "WEEKLY"
		case "BYMONTH", "BYMONTHDAY":
			simple = simple && rule["FREQ"] == "YEARLY" && !strings.Contains(rule[key], ",")
		default:
			simple = false
		}
	}
	var days string
	switch {
	case !simple:
		every = "Repeats every " + unit
		if interval > 1 {
			every = fmt.Sprintf("Repeats every %d %ss", interval, unit)
		}
	case rule["FREQ"] == "WEEKLY" && rule["BYDAY"] != "":
		var names []string
		for _, day := range strings.Split(rule["BYDAY"], ",") {
			name, ok := weekdays[day]
			if !ok {
				return "Repeats every " + unit + ", from " + formatDate(start)
			}
			names = append(names, name)
		}
		days = " on " + joinAnd(names)
	case rule["FREQ"] == "WEEKLY":
		days = " on " + start.Weekday().String()
	case rule["FREQ"] == "MONTHLY":
		days = " on the " + ordinal(start.Day())
	case rule["FREQ"] == "YEARLY":
		days = " on " + start.Format("2 January")
	}
	when := every + days

	// The times of a timed event
	if !event.AllDay {
		if end.Equal(start) {
			when += ", at " + start.Format("15:04 MST")
		} else {
			when += ", " + start.Format("15:04") + "–" + end.In(location).Format("15:04 MST")
		}
	}

	// Since a birthday's year, when it's known, or from the first date, and
	// until a date, or a number of times
	switch {
	case event.NoYear:
	case rule["FREQ"] == "YEARLY" && simple:
		when += ", since " + start.Format("2006")
	default:
		when += ", from " + formatDate(start)
	}
	if until := rule["UNTIL"]; until != "" {
		if t, ok := parseUntil(until, location); ok {
			when += ", until " + formatDate(t)
		}
	} else if count, err := strconv.Atoi(rule["COUNT"]); err == nil && count > 0 {
		when += fmt.Sprintf(", %d times", count)
	}
	return when
}

// times returns an event's start and end, in its zone when it's known,
// or UTC, and the zone, or dates, in UTC, when it's all day
func times(event Event) (time.Time, time.Time, *time.Location, bool) {
	location := time.UTC
	if event.TZ != "" {
		if loaded, ok := ical.Location(event.TZ); ok {
			location = loaded
		}
	}
	start, ok := parse(event.Start, location)
	if !ok {
		return time.Time{}, time.Time{}, nil, false
	}
	end, ok := parse(event.End, location)
	if !ok || end.Before(start) {
		end = start
	}
	return start, end, location, true
}

// parse returns a date, or an instant, in the zone
func parse(value string, location *time.Location) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.In(location), true
	}
	if t, err := time.Parse(time.DateOnly, value); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// formatDays returns the days of an all-day event, whose end is the day
// after its last
func formatDays(start, end time.Time) string {
	last := end.AddDate(0, 0, -1)
	switch {
	case !last.After(start):
		return formatDate(start)
	case last.Year() == start.Year() && last.Month() == start.Month():
		return fmt.Sprintf("%d–%s", start.Day(), formatDate(last))
	default:
		return formatDate(start) + " to " + formatDate(last)
	}
}

// formatTimes returns the date and times of a timed event, in its zone
func formatTimes(start, end time.Time) string {
	switch {
	case end.Equal(start):
		return formatDate(start) + ", " + start.Format("15:04 MST")
	case formatDate(end) == formatDate(start):
		return formatDate(start) + ", " + start.Format("15:04") + "–" + end.Format("15:04 MST")
	default:
		return formatDate(start) + ", " + start.Format("15:04") + " to " + formatDate(end) + ", " + end.Format("15:04 MST")
	}
}

// formatDate returns a date, such as 8 July 2026
func formatDate(t time.Time) string {
	return t.Format("2 January 2006")
}

// ordinal returns a number as an ordinal, such as 1st or 22nd
func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return strconv.Itoa(n) + suffix
}

// joinAnd returns the values joined with commas, and "and" before the last
func joinAnd(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	default:
		return strings.Join(values[:len(values)-1], ", ") + " and " + values[len(values)-1]
	}
}

// parseUntil returns the end of a rule, which is a date, or a time, in UTC
// when it ends with Z, shown in the zone
func parseUntil(value string, location *time.Location) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if t, err := time.Parse("20060102", value); err == nil {
		return t, true
	}
	if utc, found := strings.CutSuffix(value, "Z"); found {
		if t, err := time.Parse("20060102T150405", utc); err == nil {
			return t.In(location), true
		}
	}
	if t, err := time.ParseInLocation("20060102T150405", value, location); err == nil {
		return t, true
	}
	return time.Time{}, false
}
