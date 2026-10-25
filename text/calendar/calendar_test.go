package calendar_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
	calendar "github.com/mutablelogic/go-metadata/text/calendar"
)

const TEST_DIR = "../../testfiles"

// named is a reader with a name
type named struct {
	io.Reader
	name string
}

func (r named) Name() string { return r.name }

// entries returns the metadata of the iCalendar, by key, as their values,
// without its kind
func entries(t *testing.T, data, name string, opts ...metadata.Option) map[string]any {
	t.Helper()
	_, meta, err := metadata.Extract(context.Background(), named{strings.NewReader(data), name}, append(opts, metadata.WithContentType("text/calendar"))...)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]any, len(meta))
	for _, m := range meta {
		if key := m.NS() + ":" + m.Key(); key != "metadata:kind" && key != "dc:type" {
			result[key] = m.Any()
		}
	}
	return result
}

// events returns the events of the metadata
func events(t *testing.T, entries map[string]any) []calendar.Event {
	t.Helper()
	events, ok := entries["ical:event"].([]calendar.Event)
	if !ok {
		t.Fatalf("ical:event is %T", entries["ical:event"])
	}
	return events
}

// ics returns an iCalendar with the lines, which end with CRLF
func ics(lines ...string) string {
	return strings.Join(append(append([]string{"BEGIN:VCALENDAR", "VERSION:2.0"}, lines...), "END:VCALENDAR"), "\r\n") + "\r\n"
}

// equal reports an error when the value isn't the one which is wanted
func equal(t *testing.T, name string, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s: expected %#v, got %#v", name, want, got)
	}
}

func Test_calendar_000(t *testing.T) {
	// An iCalendar is detected as plain text, and refined by its extension
	for _, name := range []string{"c1.ics", "Birthday.ICS", "work.ical"} {
		contentType, _, err := metadata.ContentType(named{strings.NewReader(ics("BEGIN:VEVENT", "DTSTART:20261021T090000Z", "END:VEVENT")), name})
		if err != nil || contentType != "text/calendar" {
			t.Errorf("%s: expected text/calendar, got %q %v", name, contentType, err)
		}
	}
}

func Test_calendar_001(t *testing.T) {
	// An address book's birthday is an all-day event, a day long, which
	// repeats every year, in the Birthdays category, whose title is the
	// calendar's, and whose start is its date
	got := entries(t, ics(
		"PRODID:-//omodie//Google Contacts//EN",
		"BEGIN:VEVENT",
		"UID:people/c1/birthday",
		"DTSTAMP:20260901T100000Z",
		"DTSTART;VALUE=DATE:18151210",
		"RRULE:FREQ=YEARLY",
		"SUMMARY:Ada Lovelace's birthday",
		"CATEGORIES:Birthdays",
		"TRANSP:TRANSPARENT",
		"END:VEVENT",
	), "c1.ics")
	equal(t, "dc:title", "Ada Lovelace's birthday", got["dc:title"])
	equal(t, "dc:identifier", "people/c1/birthday", got["dc:identifier"])
	equal(t, "dc:date", "1815-12-10", got["dc:date"])
	equal(t, "dc:description", "Every year on 10 December, since 1815", got["dc:description"])
	equal(t, "ical:event", []calendar.Event{{
		UID:        "people/c1/birthday",
		Kind:       "event",
		Title:      "Ada Lovelace's birthday",
		Start:      "1815-12-10",
		End:        "1815-12-11",
		AllDay:     true,
		RRule:      "FREQ=YEARLY",
		Categories: []string{"Birthdays"},
	}}, events(t, got))

	// A birthday without a year is in 1604, and has no year, which isn't its
	// date, nor in its description
	got = entries(t, ics(
		"BEGIN:VEVENT",
		"UID:people/c3/birthday",
		"DTSTART;VALUE=DATE:16040229",
		"RRULE:FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=-1",
		"SUMMARY:Leap\\; Year\\, Person's birthday",
		"X-APPLE-OMIT-YEAR:1604",
		"END:VEVENT",
	), "c3.ics")
	e := events(t, got)
	equal(t, "dc:date", nil, got["dc:date"])
	equal(t, "dc:description", "Every year on 29 February", got["dc:description"])
	if len(e) != 1 || !e[0].NoYear || e[0].Start != "1604-02-29" || e[0].End != "1604-03-01" || e[0].Title != "Leap; Year, Person's birthday" {
		t.Errorf("unexpected birthday %+v", e)
	}
}

func Test_calendar_002(t *testing.T) {
	// A calendar's name, description and zone are its own, and its events'
	// times are in UTC, converted with their zones, with the zone's
	// definition and the alarms skipped
	got := entries(t, ics(
		"X-WR-CALNAME:Work",
		"X-WR-CALDESC:Meetings\\, mostly",
		"X-WR-TIMEZONE:Europe/Berlin",
		"BEGIN:VTIMEZONE",
		"TZID:Europe/Berlin",
		"BEGIN:DAYLIGHT",
		"DTSTART:19700329T020000",
		"END:DAYLIGHT",
		"END:VTIMEZONE",
		"BEGIN:VEVENT",
		"UID:standup@example.com",
		"DTSTART;TZID=Europe/Berlin:20261021T090000",
		"DTEND;TZID=Europe/Berlin:20261021T091500",
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE",
		"EXDATE;TZID=Europe/Berlin:20261026T090000,20261028T090000",
		"SUMMARY:Stand-up",
		"LOCATION:Room 4\\, second floor",
		"DESCRIPTION:A long description which is folded across more than one line",
		" \\, as lines are\\nat most 75 octets.",
		"ORGANIZER;CN=\"King, Ada\":mailto:ada@example.com",
		"ATTENDEE;CN=\"Bob: the builder\";ROLE=REQ-PARTICIPANT:MAILTO:bob@example.com",
		"ATTENDEE;CN=Carol:mailto:carol@example.com",
		"CATEGORIES:Work,Daily",
		"BEGIN:VALARM",
		"ACTION:DISPLAY",
		"DESCRIPTION:Not the event's",
		"TRIGGER:-PT10M",
		"END:VALARM",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"UID:standup@example.com",
		"RECURRENCE-ID;TZID=Europe/Berlin:20261104T090000",
		"DTSTART;TZID=Europe/Berlin:20261104T100000",
		"DURATION:PT1H30M",
		"SUMMARY:Stand-up",
		"STATUS:CONFIRMED",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"UID:call@example.com",
		"DTSTART:20261020T230000Z",
		"DTEND:20261021T000000Z",
		"SUMMARY:Call New York",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"UID:floating@example.com",
		"DTSTART:20261022T120000",
		"SUMMARY:Lunch",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"UID:windows@example.com",
		"DTSTART;TZID=W. Europe Standard Time:20261023T090000",
		"DTEND;TZID=W. Europe Standard Time:20261023T100000",
		"SUMMARY:From Outlook",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"UID:nostart@example.com",
		"SUMMARY:No start",
		"END:VEVENT",
	), "work.ics")
	equal(t, "dc:title", "Work", got["dc:title"])
	equal(t, "dc:description", "Meetings, mostly\n5 events, from 20 October 2026 to 4 November 2026: Stand-up, Call New York, Lunch, From Outlook", got["dc:description"])
	equal(t, "ical:tz", "Europe/Berlin", got["ical:tz"])
	equal(t, "dc:creator", "ada@example.com", got["dc:creator"])
	equal(t, "dc:identifier", nil, got["dc:identifier"])
	equal(t, "dc:date", "2026-10-20T23:00:00Z", got["dc:date"])
	equal(t, "ical:location", "Room 4, second floor", got["ical:location"])

	e := events(t, got)
	if len(e) != 5 {
		t.Fatalf("expected five events, got %d", len(e))
	}

	// Berlin is UTC+2 in October, and the alarm isn't the event's
	standup := e[0]
	equal(t, "start", "2026-10-21T07:00:00Z", standup.Start)
	equal(t, "end", "2026-10-21T07:15:00Z", standup.End)
	equal(t, "tz", "Europe/Berlin", standup.TZ)
	equal(t, "all day", false, standup.AllDay)
	equal(t, "rrule", "FREQ=WEEKLY;BYDAY=MO,WE", standup.RRule)
	equal(t, "exdate", []string{"2026-10-26T08:00:00Z", "2026-10-28T08:00:00Z"}, standup.ExDate)
	equal(t, "description", "A long description which is folded across more than one line, as lines are\nat most 75 octets.", standup.Description)
	equal(t, "organizer", "ada@example.com", standup.Organizer)
	equal(t, "attendees", []string{"bob@example.com", "carol@example.com"}, standup.Attendees)
	equal(t, "categories", []string{"Work", "Daily"}, standup.Categories)

	// A changed occurrence replaces the start of the one it changes, and its
	// end is its start and duration, after Berlin's clocks went back
	changed := e[1]
	equal(t, "recurrence id", "2026-11-04T08:00:00Z", changed.RecurrenceID)
	equal(t, "start", "2026-11-04T09:00:00Z", changed.Start)
	equal(t, "end", "2026-11-04T10:30:00Z", changed.End)
	equal(t, "status", "CONFIRMED", changed.Status)

	// A time in UTC is in UTC, and one without a zone, or with a zone which
	// isn't known, is treated as UTC, without a zone, and an event without
	// an end ends when it starts
	equal(t, "tz", "UTC", e[2].TZ)
	equal(t, "end", "2026-10-21T00:00:00Z", e[2].End)
	equal(t, "start", "2026-10-22T12:00:00Z", e[3].Start)
	equal(t, "end", e[3].Start, e[3].End)
	equal(t, "tz", "", e[3].TZ)
	equal(t, "start", "2026-10-23T09:00:00Z", e[4].Start)
	equal(t, "tz", "", e[4].TZ)
}

func Test_calendar_003(t *testing.T) {
	// A task without a start starts and ends when it's due
	e := events(t, entries(t, ics(
		"BEGIN:VTODO",
		"UID:task@example.com",
		"DUE;VALUE=DATE:20261031",
		"SUMMARY:File the tax return",
		"STATUS:NEEDS-ACTION",
		"END:VTODO",
	), "tasks.ics"))
	if len(e) != 1 {
		t.Fatalf("expected one task, got %d", len(e))
	}
	equal(t, "kind", "task", e[0].Kind)
	equal(t, "due", "2026-10-31", e[0].Due)
	equal(t, "start", "2026-10-31", e[0].Start)
	equal(t, "end", "2026-11-01", e[0].End)
	equal(t, "all day", true, e[0].AllDay)
	equal(t, "status", "NEEDS-ACTION", e[0].Status)
}

func Test_calendar_004(t *testing.T) {
	// Events with the same title, such as the flights of a trip, give the
	// calendar its title, and zones which are offsets are used
	got := entries(t, ics(
		"PRODID:-//Ben Fortuna//iCal4j 1.0//EN",
		"BEGIN:VEVENT",
		"DTSTART;TZID=\"+02:00\":20260710T064500",
		"DTEND;TZID=\"+03:00\":20260710T104500",
		"SUMMARY:Turkish Airlines flight details",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"DTSTART;TZID=\"+03:00\":20260713T160000",
		"DTEND;TZID=\"+02:00\":20260713T175000",
		"SUMMARY:Turkish Airlines flight details",
		"END:VEVENT",
	), "flights.ics")
	equal(t, "dc:title", "Turkish Airlines flight details", got["dc:title"])
	e := events(t, got)
	if len(e) != 2 {
		t.Fatalf("expected two events, got %d", len(e))
	}
	equal(t, "start", "2026-07-10T04:45:00Z", e[0].Start)
	equal(t, "end", "2026-07-10T07:45:00Z", e[0].End)
	equal(t, "tz", "+02:00", e[0].TZ)
	equal(t, "start", "2026-07-13T13:00:00Z", e[1].Start)
	equal(t, "end", "2026-07-13T15:50:00Z", e[1].End)
	equal(t, "dc:description", "2 events, from 10 July 2026 to 13 July 2026: Turkish Airlines flight details", got["dc:description"])

	// Events with different titles don't
	got = entries(t, ics(
		"BEGIN:VEVENT", "DTSTART:20260710T064500Z", "SUMMARY:Out", "END:VEVENT",
		"BEGIN:VEVENT", "DTSTART:20260713T160000Z", "SUMMARY:Back", "END:VEVENT",
	), "flights.ics")
	equal(t, "dc:title", "flights", got["dc:title"])
}

func Test_calendar_005(t *testing.T) {
	// Without events, the title is the file's name, and there are no events
	// or date
	for _, data := range []string{"", ics(), "not a calendar"} {
		got := entries(t, data, "Holiday_plans.ics")
		equal(t, "dc:title", "Holiday plans", got["dc:title"])
		equal(t, "ical:event", nil, got["ical:event"])
		equal(t, "dc:date", nil, got["dc:date"])
	}
}

func Test_describe_000(t *testing.T) {
	// description returns the description of a calendar of the lines
	description := func(t *testing.T, lines ...string) any {
		t.Helper()
		return entries(t, ics(lines...), "event.ics")["dc:description"]
	}

	// A timed event is described in its zone, with where it is, and its own
	// description, and one which crosses midnight, or has no end, or is in
	// UTC
	equal(t, "timed", "21 October 2026, 09:00–09:15 CEST\nRoom 4\nThe weekly one", description(t,
		"BEGIN:VEVENT",
		"DTSTART;TZID=Europe/Berlin:20261021T090000",
		"DTEND;TZID=Europe/Berlin:20261021T091500",
		"LOCATION:Room 4",
		"DESCRIPTION:The weekly one",
		"END:VEVENT",
	))
	equal(t, "midnight", "21 October 2026, 23:00 to 22 October 2026, 01:00 UTC", description(t,
		"BEGIN:VEVENT", "DTSTART:20261021T230000Z", "DTEND:20261022T010000Z", "END:VEVENT",
	))
	equal(t, "no end", "21 October 2026, 23:00 UTC", description(t,
		"BEGIN:VEVENT", "DTSTART:20261021T230000Z", "END:VEVENT",
	))

	// An all-day event is its days, whose end is the day after its last
	equal(t, "a day", "21 October 2026", description(t,
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "END:VEVENT",
	))
	equal(t, "days", "21–23 October 2026", description(t,
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "DTEND;VALUE=DATE:20261024", "END:VEVENT",
	))
	equal(t, "months", "30 October 2026 to 2 November 2026", description(t,
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261030", "DTEND;VALUE=DATE:20261103", "END:VEVENT",
	))
}

func Test_describe_001(t *testing.T) {
	description := func(t *testing.T, lines ...string) any {
		t.Helper()
		return entries(t, ics(lines...), "event.ics")["dc:description"]
	}

	// A repeating event is how it repeats, its times, and from and until
	// when, or how many times
	equal(t, "weekly", "Every week on Monday and Wednesday, 09:00–09:15 CEST, from 21 October 2026", description(t,
		"BEGIN:VEVENT",
		"DTSTART;TZID=Europe/Berlin:20261021T090000",
		"DTEND;TZID=Europe/Berlin:20261021T091500",
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE",
		"END:VEVENT",
	))
	equal(t, "fortnightly", "Every 2 weeks on Wednesday, from 21 October 2026, until 16 December 2026", description(t,
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "RRULE:FREQ=WEEKLY;INTERVAL=2;UNTIL=20261216", "END:VEVENT",
	))
	equal(t, "monthly", "Every month on the 1st, at 08:00 UTC, from 1 November 2026, 12 times", description(t,
		"BEGIN:VEVENT", "DTSTART:20261101T080000Z", "RRULE:FREQ=MONTHLY;COUNT=12", "END:VEVENT",
	))
	equal(t, "daily", "Every day, from 21 October 2026", description(t,
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "RRULE:FREQ=DAILY", "END:VEVENT",
	))

	// A rule which isn't simple is described by its frequency, as is a
	// yearly rule with several months, or days
	equal(t, "not simple", "Repeats every month, from 13 October 2026", description(t,
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261013", "RRULE:FREQ=MONTHLY;BYDAY=2TU", "END:VEVENT",
	))
	equal(t, "yearly", "Repeats every year, from 1 January 2026", description(t,
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20260101", "RRULE:FREQ=YEARLY;BYMONTH=1,7;BYMONTHDAY=1", "END:VEVENT",
	))

	// A day's duration is a day in the event's zone, across the end of
	// daylight saving time, as 12:00 CEST to 12:00 CET
	e := events(t, entries(t, ics(
		"BEGIN:VEVENT", "DTSTART;TZID=Europe/Berlin:20261024T120000", "DURATION:P1D", "END:VEVENT",
	), "dst.ics"))
	equal(t, "start", "2026-10-24T10:00:00Z", e[0].Start)
	equal(t, "end", "2026-10-25T11:00:00Z", e[0].End)

	// Several events' dates are ordered as they're shown, in their zones:
	// 01:00 on the 22nd in Tokyo is before 23:00 on the 21st in Los Angeles,
	// but its date is later
	equal(t, "zones", "2 events, from 21 October 2026 to 22 October 2026", description(t,
		"BEGIN:VEVENT", "DTSTART;TZID=Asia/Tokyo:20261022T010000", "END:VEVENT",
		"BEGIN:VEVENT", "DTSTART;TZID=America/Los_Angeles:20261021T230000", "END:VEVENT",
	))
}

func Test_describe_002(t *testing.T) {
	description := func(t *testing.T, lines ...string) any {
		t.Helper()
		return entries(t, ics(lines...), "event.ics")["dc:description"]
	}

	// A task with a start is described by when it's due, which it keeps, a
	// task is when it's due, and a cancelled event says so
	got := entries(t, ics("BEGIN:VTODO", "DTSTART;VALUE=DATE:20261021", "DUE;VALUE=DATE:20261031", "END:VTODO"), "task.ics")
	equal(t, "task with a start", "Due 31 October 2026", got["dc:description"])
	e := events(t, got)
	equal(t, "start", "2026-10-21", e[0].Start)
	equal(t, "due", "2026-10-31", e[0].Due)
	equal(t, "task", "Due 31 October 2026", description(t, "BEGIN:VTODO", "DUE;VALUE=DATE:20261031", "END:VTODO"))
	equal(t, "cancelled", "Cancelled: 21 October 2026", description(t, "BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "STATUS:CANCELLED", "END:VEVENT"))

	// Several events are their number, the dates of their starts whose
	// years are known, and their titles, once each
	equal(t, "events", "3 events, on 21 October 2026: Lunch, Ada's birthday", description(t,
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "SUMMARY:Lunch", "END:VEVENT",
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "SUMMARY:Lunch", "END:VEVENT",
		"BEGIN:VEVENT", "DTSTART;VALUE=DATE:16041210", "SUMMARY:Ada's birthday", "X-APPLE-OMIT-YEAR:1604", "END:VEVENT",
	))
}

func Test_calendar_006(t *testing.T) {
	// The test files are calendars, with their events, or an event, when
	// they have one, which is a birthday, whose image is its artwork
	for name, want := range map[string]int{"ical_google.ics": 5, "ical_birthday1.ics": 1, "ical_birthday2.ics": 1} {
		f, err := os.Open(filepath.Join(TEST_DIR, name))
		if err != nil {
			t.Fatal(err)
		}
		_, meta, err := metadata.Extract(context.Background(), f, metadata.WithNamespace("dc", "ical", "metadata", metadata.ArtworkNamespace))
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := map[string]any{}
		for _, m := range meta {
			got[m.NS()+":"+m.Key()] = m.Any()
		}
		if e, ok := got["ical:event"].([]calendar.Event); !ok || len(e) != want {
			t.Errorf("%s: expected %d events, got %v", name, want, got["ical:event"])
		}
		if strings.HasPrefix(name, "ical_birthday") {
			equal(t, name+" kind", "event", got["metadata:kind"])
			equal(t, name+" type", "Event", got["dc:type"])
			if types, _ := got["ical:type"].([]string); !slices.Contains(types, "birthday") {
				t.Errorf("%s: expected a birthday, got %v", name, got["ical:type"])
			}
			if got["artwork:image"] == nil {
				t.Errorf("%s: expected the birthday's image", name)
			}
		} else {
			equal(t, name+" kind", "calendar", got["metadata:kind"])
		}
	}
}

func Test_span_000(t *testing.T) {
	// When a calendar starts and ends: a birthday whose year isn't known
	// has neither, and one whose year is known starts, but doesn't end, as
	// it repeats forever, as does any event which does
	span := func(t *testing.T, lines ...string) (any, any) {
		t.Helper()
		got := entries(t, ics(lines...), "span.ics")
		return got["ical:date-start"], got["ical:date-end"]
	}
	day := func(year int, month time.Month, d int) time.Time {
		return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
	}
	for _, test := range []struct {
		name       string
		lines      []string
		start, end any
	}{
		{"birthday without a year", []string{
			"BEGIN:VEVENT", "DTSTART;VALUE=DATE:16041210", "RRULE:FREQ=YEARLY", "X-APPLE-OMIT-YEAR:1604", "END:VEVENT",
		}, nil, nil},
		{"birthday", []string{
			"BEGIN:VEVENT", "DTSTART;VALUE=DATE:18151210", "RRULE:FREQ=YEARLY", "END:VEVENT",
		}, day(1815, 12, 10), nil},
		{"events", []string{
			"BEGIN:VEVENT", "DTSTART:20261021T090000Z", "DTEND:20261021T100000Z", "END:VEVENT",
			"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261030", "DTEND;VALUE=DATE:20261103", "END:VEVENT",
		}, time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC), day(2026, 11, 3)},
		{"an event which repeats forever", []string{
			"BEGIN:VEVENT", "DTSTART:20261021T090000Z", "DTEND:20261021T100000Z", "END:VEVENT",
			"BEGIN:VEVENT", "DTSTART;TZID=Europe/Berlin:20261021T090000", "DTEND;TZID=Europe/Berlin:20261021T091500", "RRULE:FREQ=WEEKLY;BYDAY=MO,WE", "END:VEVENT",
		}, time.Date(2026, 10, 21, 7, 0, 0, 0, time.UTC), nil},
		{"until a date", []string{
			"BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "RRULE:FREQ=WEEKLY;INTERVAL=2;UNTIL=20261216", "END:VEVENT",
		}, day(2026, 10, 21), day(2026, 12, 17)},
		{"until a time", []string{
			"BEGIN:VEVENT", "DTSTART:20261021T090000Z", "DTEND:20261021T093000Z", "RRULE:FREQ=DAILY;UNTIL=20261031T090000Z", "END:VEVENT",
		}, time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 31, 9, 30, 0, 0, time.UTC)},
		{"a number of times", []string{
			"BEGIN:VEVENT", "DTSTART:20261101T080000Z", "DTEND:20261101T090000Z", "RRULE:FREQ=MONTHLY;COUNT=12", "END:VEVENT",
		}, time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC), time.Date(2027, 10, 1, 9, 0, 0, 0, time.UTC)},
		{"no events", nil, nil, nil},
	} {
		start, end := span(t, test.lines...)
		equal(t, test.name+": start", test.start, start)
		equal(t, test.name+": end", test.end, end)
	}
}

func Test_markdown_000(t *testing.T) {
	// A calendar as Markdown: its name and description, and its events, in
	// the order they start, with when they are, in words, where, who, and
	// their descriptions
	_, meta, err := metadata.Extract(context.Background(), strings.NewReader(ics(
		"X-WR-CALNAME:Work",
		"X-WR-CALDESC:Meetings",
		"BEGIN:VEVENT",
		"DTSTART;VALUE=DATE:20261030",
		"SUMMARY:Holiday",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"DTSTART;TZID=Europe/Berlin:20261021T090000",
		"DTEND;TZID=Europe/Berlin:20261021T091500",
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE",
		"SUMMARY:Stand-up",
		"LOCATION:Room 4",
		"ORGANIZER:mailto:ada@example.com",
		"ATTENDEE:mailto:bob@example.com",
		"ATTENDEE:mailto:carol@example.com",
		"DESCRIPTION:The *weekly* one\\nwith notes",
		"END:VEVENT",
		"BEGIN:VTODO",
		"DUE;VALUE=DATE:20261031",
		"END:VTODO",
	)), metadata.WithContentType("text/calendar"), metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Work\n\nMeetings\n\n" +
		"## Stand-up\n\n" +
		"- **When:** Every week on Monday and Wednesday, 09:00–09:15 CEST, from 21 October 2026\n" +
		"- **Where:** Room 4\n" +
		"- **Organizer:** ada@example.com\n" +
		"- **Attendees:** bob@example.com, carol@example.com\n\n" +
		"The \\*weekly\\* one  \nwith notes\n\n" +
		"## Holiday\n\n" +
		"- **When:** 30 October 2026\n\n" +
		"## Untitled task\n\n" +
		"- **When:** Due 31 October 2026\n\n"
	for _, m := range meta {
		if m.Key() == "body" && m.Value() != want {
			t.Errorf("expected\n%q\ngot\n%q", want, m.Value())
		}
	}
}

func Test_calendar_007(t *testing.T) {
	// A calendar of more events than are stored has a note which says so,
	// and is described by all of them, and one of fewer has no note
	lines := []string{}
	for i := range 1005 {
		lines = append(lines, "BEGIN:VEVENT", fmt.Sprintf("DTSTART;VALUE=DATE:2026%04d", 101+i%28), fmt.Sprintf("SUMMARY:Event %d", i), "END:VEVENT")
	}
	got := entries(t, ics(lines...), "many.ics")
	if e, ok := got["ical:event"].([]calendar.Event); !ok || len(e) != 1000 {
		t.Errorf("expected 1000 events, got %v", len(e))
	}
	equal(t, "ical:note", "Only the first 1000 of the calendar's 1005 events are stored in ical:event.", got["ical:note"])
	if description, _ := got["dc:description"].(string); !strings.HasPrefix(description, "1005 events, ") {
		t.Errorf("expected a description of all the events, got %.40q", description)
	}
	if got := entries(t, ics("BEGIN:VEVENT", "DTSTART;VALUE=DATE:20261021", "END:VEVENT"), "one.ics"); got["ical:note"] != nil {
		t.Errorf("expected no note, got %v", got["ical:note"])
	}
}

func Test_calendar_008(t *testing.T) {
	// An event of a calendar, written by Google Calendar, is titled by its
	// event, rather than its calendar, which is ical:calendar-name, its
	// description, which is HTML, is Markdown, which isn't stored with the
	// event again, and its kind, title, status, organizer, attendees and
	// location are each a value
	got := entries(t, ics(
		"PRODID:-//omodie//Google Calendar//EN",
		"X-WR-CALNAME:Work",
		"X-WR-TIMEZONE:Europe/Berlin",
		"BEGIN:VEVENT",
		"UID:6q5q@google.com",
		"DTSTART;TZID=Europe/Lisbon:20251006T134000",
		"DTEND;TZID=Europe/Lisbon:20251006T140000",
		"SUMMARY:[Fabric] Sprint Deadline",
		"DESCRIPTION:<p>👉 <i>This marks the </i><b>close</b> of the sprint.</p><br>Make sure:<ul><li>Tasks are done</li></ul>",
		"LOCATION:https://app.asana.com/0/portfolio/1",
		"STATUS:CONFIRMED",
		"ORGANIZER:mailto:rafael@example.com",
		"ATTENDEE:mailto:david@example.com",
		"ATTENDEE:mailto:rafael@example.com",
		"END:VEVENT",
	), "event.ics")
	equal(t, "dc:title", "[Fabric] Sprint Deadline", got["dc:title"])
	equal(t, "ical:calendar-name", "Work", got["ical:calendar-name"])
	equal(t, "ical:kind", "event", got["ical:kind"])
	equal(t, "ical:title", "[Fabric] Sprint Deadline", got["ical:title"])
	equal(t, "ical:status", "CONFIRMED", got["ical:status"])
	equal(t, "ical:organizer", "rafael@example.com", got["ical:organizer"])
	equal(t, "ical:attendee", []string{"david@example.com", "rafael@example.com"}, got["ical:attendee"])
	equal(t, "ical:location", "https://app.asana.com/0/portfolio/1", got["ical:location"])
	description, _ := got["dc:description"].(string)
	for _, text := range []string{"This marks the", "close", "of the sprint.", "Make sure:", "- Tasks are done"} {
		if !strings.Contains(description, text) {
			t.Errorf("dc:description: expected %q in %q", text, description)
		}
	}
	if strings.Contains(description, "<") {
		t.Errorf("dc:description has HTML: %q", description)
	}
	equal(t, "description", "", events(t, got)[0].Description)

	// The title of events which have different ones is the calendar's
	// name, and their values are their different ones, and a description
	// which is text, with its lines, is as it is
	got = entries(t, ics(
		"X-WR-CALNAME:Work",
		"BEGIN:VEVENT", "UID:a", "DTSTART:20261020T090000Z", "SUMMARY:Stand-up", "STATUS:CONFIRMED", "DESCRIPTION:First\\nSecond", "END:VEVENT",
		"BEGIN:VEVENT", "UID:b", "DTSTART:20261021T090000Z", "SUMMARY:Review", "STATUS:TENTATIVE", "END:VEVENT",
	), "work.ics")
	equal(t, "dc:title", "Work", got["dc:title"])
	equal(t, "ical:title", []string{"Stand-up", "Review"}, got["ical:title"])
	equal(t, "ical:status", []string{"CONFIRMED", "TENTATIVE"}, got["ical:status"])
	equal(t, "description", "First\nSecond", events(t, got)[0].Description)
}

func Test_calendar_009(t *testing.T) {
	// A description with a placeholder in angle brackets is text, which is
	// kept as it is, rather than HTML
	got := entries(t, ics(
		"BEGIN:VEVENT", "UID:a", "DTSTART:20261020T090000Z", "SUMMARY:Sign in",
		"DESCRIPTION:Press <Enter> to confirm\\, with <username> and <password>",
		"END:VEVENT",
	), "event.ics")
	description, _ := got["dc:description"].(string)
	if !strings.Contains(description, "Press <Enter> to confirm, with <username> and <password>") {
		t.Errorf("dc:description: expected the placeholders in %q", description)
	}
}

func Test_markdown_001(t *testing.T) {
	// An event's description which is HTML is Markdown, which isn't escaped
	// again, and one which is text is escaped
	_, meta, err := metadata.Extract(context.Background(), strings.NewReader(ics(
		"BEGIN:VEVENT",
		"DTSTART:20261021T090000Z",
		"SUMMARY:Sprint",
		`DESCRIPTION:<p>The <b>close</b> of the <a href="https://example.com/">sprint</a></p>`,
		"END:VEVENT",
	)), metadata.WithContentType("text/calendar"), metadata.WithNamespace(metadata.MarkdownNamespace), metadata.WithRange(":"))
	if err != nil {
		t.Fatal(err)
	}
	var body string
	for _, m := range meta {
		if m.Key() == "body" {
			body = m.Value()
		}
	}
	if !strings.Contains(body, "The **close** of the [sprint](https://example.com/)") {
		t.Errorf("expected the description as Markdown in %q", body)
	}
	if strings.Contains(body, `\*`) || strings.Contains(body, "<p>") {
		t.Errorf("expected the description not escaped, and without HTML, in %q", body)
	}
}

func Test_calendar_010(t *testing.T) {
	// An event's link is ical:url, and its types are what it is: all day,
	// as is a task due on a date, over more than a day, repeating, an
	// occurrence of one, a birthday, or the type Google Calendar gives it,
	// as its category, and a description
	// which is HTML has spaces, rather than non-breaking ones
	got := entries(t, ics(
		"BEGIN:VEVENT",
		"UID:a",
		"DTSTART;VALUE=DATE:20261026",
		"DTEND;VALUE=DATE:20261031",
		"SUMMARY:Holiday",
		"CATEGORIES:Out of office",
		"URL:https://www.google.com/calendar/event?eid=a",
		"DESCRIPTION:<p>Back&nbsp;on&nbsp;Monday</p>",
		"END:VEVENT",
	), "holiday.ics")
	equal(t, "ical:url", "https://www.google.com/calendar/event?eid=a", got["ical:url"])
	equal(t, "ical:type", []string{"all-day", "multi-day", "out-of-office"}, got["ical:type"])
	equal(t, "dc:description", "26–30 October 2026\nBack on Monday", got["dc:description"])

	got = entries(t, ics(
		"BEGIN:VEVENT", "UID:b", "DTSTART:20261026T090000Z", "DTEND:20261026T091500Z", "RRULE:FREQ=WEEKLY", "SUMMARY:Stand-up", "END:VEVENT",
		"BEGIN:VEVENT", "UID:b", "RECURRENCE-ID:20261102T090000Z", "DTSTART:20261102T100000Z", "DTEND:20261102T101500Z", "SUMMARY:Stand-up", "END:VEVENT",
		"BEGIN:VTODO", "UID:c", "DUE;VALUE=DATE:20261031", "SUMMARY:Report", "END:VTODO",
	), "work.ics")
	equal(t, "ical:type", []string{"recurring", "occurrence", "task", "all-day"}, got["ical:type"])
}
