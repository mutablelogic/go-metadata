package ical_test

import (
	"bytes"
	"image"
	"image/png"
	"reflect"
	"strings"
	"testing"
	"time"

	// Packages
	geo "github.com/mutablelogic/go-metadata/pkg/geo"
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
)

// calendar returns an iCalendar with the lines
func calendar(lines ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" + strings.Join(append(lines, "END:VCALENDAR"), "\r\n") + "\r\n"
}

// decode returns the calendar of the text
func decode(t *testing.T, data string) *ical.Calendar {
	t.Helper()
	c, err := ical.Decode(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// utc returns a time in UTC
func utc(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
}

func Test_encode_000(t *testing.T) {
	// A birthday without a year, as omodie's Google backend writes it: in a
	// leap year, which a calendar doesn't show an age for, with the 29th of
	// February on the last day of February each year, and the epoch as its
	// stamp, so it's the same each time
	c := &ical.Calendar{ProdID: "-//omodie//Google Contacts//EN", Events: []*ical.Event{{
		Kind:        ical.KindEvent,
		UID:         "people/c3/birthday",
		Start:       ical.Date(1604, time.February, 29),
		RRule:       "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=-1",
		Summary:     "Leap; Year, Person's birthday",
		Categories:  []string{"Birthdays"},
		Transparent: true,
		OmitYear:    1604,
	}}}
	data, err := c.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	want := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//omodie//Google Contacts//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:people/c3/birthday\r\n" +
		"DTSTAMP:19700101T000000Z\r\n" +
		"DTSTART;VALUE=DATE:16040229\r\n" +
		"RRULE:FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=-1\r\n" +
		"SUMMARY:Leap\\; Year\\, Person's birthday\r\n" +
		"CATEGORIES:Birthdays\r\n" +
		"TRANSP:TRANSPARENT\r\n" +
		"X-APPLE-OMIT-YEAR:1604\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	if string(data) != want {
		t.Errorf("expected\n%s\ngot\n%s", want, data)
	}
}

func Test_decode_000(t *testing.T) {
	// A calendar's name, description and zone are its own, and its events'
	// times are in their zones, with the zone's definition and the alarms
	// skipped
	c := decode(t, calendar(
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
		"BEGIN:VALARM",
		"ACTION:DISPLAY",
		"DESCRIPTION:Not the event's",
		"TRIGGER:-PT10M",
		"END:VALARM",
		"DTEND;TZID=Europe/Berlin:20261021T091500",
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE",
		"EXDATE;TZID=Europe/Berlin:20261026T090000,20261028T090000",
		"SUMMARY:Stand-up",
		"LOCATION:Room 4\\, second floor",
		"GEO:52.52;13.405",
		"DESCRIPTION:A long description which is folded across more than one line",
		" \\, as lines are\\nat most 75 octets.",
		"ORGANIZER;CN=\"King, Ada\":mailto:ada@example.com",
		"ATTENDEE;CN=\"Bob: the builder\";ROLE=REQ-PARTICIPANT:MAILTO:bob@example.com",
		"ATTENDEE;CN=Carol:mailto:carol@example.com",
		"CATEGORIES:Work,Daily",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"UID:standup@example.com",
		"RECURRENCE-ID;TZID=Europe/Berlin:20261104T090000",
		"DTSTART;TZID=Europe/Berlin:20261104T100000",
		"DURATION:PT1H30M",
		"SUMMARY:Stand-up",
		"STATUS:confirmed",
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
		"SUMMARY:From Outlook",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"UID:nostart@example.com",
		"SUMMARY:No start",
		"END:VEVENT",
	))
	if c.Name != "Work" || c.Description != "Meetings, mostly" || c.TZ != "Europe/Berlin" {
		t.Errorf("unexpected calendar %+v", c)
	}
	if len(c.Events) != 5 {
		t.Fatalf("expected five events, got %d", len(c.Events))
	}

	// Berlin is UTC+2 in October, and the alarm isn't the event's
	standup := c.Events[0]
	if standup.Kind != ical.KindEvent || standup.UID != "standup@example.com" || standup.Summary != "Stand-up" || standup.Location != "Room 4, second floor" {
		t.Errorf("unexpected event %+v", standup)
	}
	if !standup.Start.Time.Equal(utc(2026, 10, 21, 7, 0)) || standup.Start.TZID != "Europe/Berlin" || standup.Start.Floating || !standup.End.Time.Equal(utc(2026, 10, 21, 7, 15)) {
		t.Errorf("unexpected times %+v %+v", standup.Start, standup.End)
	}
	if len(standup.ExDates) != 2 || !standup.ExDates[0].Time.Equal(utc(2026, 10, 26, 8, 0)) || !standup.ExDates[1].Time.Equal(utc(2026, 10, 28, 8, 0)) {
		t.Errorf("unexpected exdates %+v", standup.ExDates)
	}
	if standup.Description != "A long description which is folded across more than one line, as lines are\nat most 75 octets." {
		t.Errorf("unexpected description %q", standup.Description)
	}
	if standup.Organizer != "ada@example.com" || !reflect.DeepEqual(standup.Attendees, []string{"bob@example.com", "carol@example.com"}) || !reflect.DeepEqual(standup.Categories, []string{"Work", "Daily"}) {
		t.Errorf("unexpected people %+v", standup)
	}
	if standup.RRule != "FREQ=WEEKLY;BYDAY=MO,WE" || standup.Geo == nil || !standup.Geo.Equal(geo.LatLong{Lat: 52.52, Long: 13.405}) {
		t.Errorf("unexpected rule or position %q %v", standup.RRule, standup.Geo)
	}

	// A changed occurrence replaces the start of the one it changes, and
	// its end is its start and duration, after Berlin's clocks went back
	changed := c.Events[1]
	if !changed.RecurrenceID.Time.Equal(utc(2026, 11, 4, 8, 0)) || !changed.Start.Time.Equal(utc(2026, 11, 4, 9, 0)) || !changed.End.Time.Equal(utc(2026, 11, 4, 10, 30)) || changed.Status != "CONFIRMED" {
		t.Errorf("unexpected changed occurrence %+v", changed)
	}

	// A time in UTC is in UTC, one without a zone is floating, as is one in
	// a zone which isn't known, which keeps its TZID, and an event without
	// an end ends when it starts
	call, lunch, outlook := c.Events[2], c.Events[3], c.Events[4]
	if call.Start.TZID != "" || call.Start.Floating || !call.End.Time.Equal(utc(2026, 10, 21, 0, 0)) {
		t.Errorf("unexpected call %+v", call)
	}
	if !lunch.Start.Floating || !lunch.Start.Time.Equal(utc(2026, 10, 22, 12, 0)) || lunch.End != lunch.Start {
		t.Errorf("unexpected lunch %+v", lunch)
	}
	if !outlook.Start.Floating || outlook.Start.TZID != "W. Europe Standard Time" || !outlook.Start.Time.Equal(utc(2026, 10, 23, 9, 0)) {
		t.Errorf("unexpected outlook event %+v", outlook.Start)
	}
}

func Test_decode_001(t *testing.T) {
	// A task without a start starts when it's due, and an all-day one ends
	// the next day, and zones which are offsets are used
	c := decode(t, calendar(
		"BEGIN:VTODO",
		"UID:task@example.com",
		"DUE;VALUE=DATE:20261031",
		"SUMMARY:File the tax return",
		"STATUS:NEEDS-ACTION",
		"END:VTODO",
		"BEGIN:VEVENT",
		"DTSTART;TZID=\"+02:00\":20260710T064500",
		"DTEND;TZID=\"+03:00\":20260710T104500",
		"SUMMARY:Flight",
		"END:VEVENT",
	))
	if len(c.Events) != 2 {
		t.Fatalf("expected two events, got %d", len(c.Events))
	}
	task, flight := c.Events[0], c.Events[1]
	if task.Kind != ical.KindTask || task.Due != ical.Date(2026, 10, 31) || task.Start != task.Due || task.End != ical.Date(2026, 11, 1) || task.Status != "NEEDS-ACTION" {
		t.Errorf("unexpected task %+v", task)
	}
	if !flight.Start.Time.Equal(utc(2026, 7, 10, 4, 45)) || !flight.End.Time.Equal(utc(2026, 7, 10, 7, 45)) || flight.Start.TZID != "+02:00" {
		t.Errorf("unexpected flight %+v %+v", flight.Start, flight.End)
	}

	// Text without a calendar, or an empty calendar, has no events
	for _, data := range []string{"", calendar(), "not a calendar"} {
		if c := decode(t, data); len(c.Events) != 0 || c.Name != "" {
			t.Errorf("%q: expected nothing, got %+v", data, c)
		}
	}
}

func Test_roundtrip_000(t *testing.T) {
	// A calendar which is encoded is decoded as it was, with its times in
	// UTC, in a zone, floating and as dates, and its image
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	ll := geo.LatLong{Lat: 52.52, Long: 13.405}
	c := &ical.Calendar{
		ProdID: ical.DefaultProdID,
		Name:   "Trips; and, more",
		TZ:     "Europe/Berlin",
		Image:  &ical.Image{MediaType: "image/png", Data: buf.Bytes(), Display: "BADGE"},
		Events: []*ical.Event{
			{
				Kind: ical.KindEvent, UID: "a@example.com", Summary: "Meeting", Description: "Line one\nLine two",
				Location: "Berlin", Geo: &ll, URL: "https://example.com/a;b",
				Start: ical.In(time.Date(2026, 10, 21, 9, 0, 0, 0, berlin), berlin), End: ical.In(time.Date(2026, 10, 21, 10, 0, 0, 0, berlin), berlin),
				RRule: "FREQ=WEEKLY", ExDates: []ical.Time{ical.In(time.Date(2026, 10, 28, 9, 0, 0, 0, berlin), berlin)},
				Status: "CONFIRMED", Organizer: "ada@example.com", Attendees: []string{"bob@example.com"}, Categories: []string{"Work"},
				Stamp: utc(2026, 10, 1, 8, 0), Created: utc(2026, 9, 1, 8, 0), Modified: utc(2026, 9, 2, 8, 0),
			},
			{Kind: ical.KindEvent, UID: "b@example.com", Start: ical.UTC(utc(2026, 10, 22, 12, 0)), End: ical.UTC(utc(2026, 10, 22, 13, 0)), Stamp: utc(2026, 10, 1, 8, 0)},
			{Kind: ical.KindEvent, UID: "c@example.com", Start: ical.Time{Time: utc(2026, 10, 23, 9, 0), Floating: true}, End: ical.Time{Time: utc(2026, 10, 23, 9, 0), Floating: true}, Stamp: utc(2026, 10, 1, 8, 0)},
			{Kind: ical.KindTask, UID: "d@example.com", Start: ical.Date(2026, 10, 30), Due: ical.Date(2026, 10, 31), End: ical.Date(2026, 10, 31), Stamp: utc(2026, 10, 1, 8, 0)},
		},
	}
	data, err := c.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, string(data))
	if got.Name != c.Name || got.TZ != c.TZ || got.Image == nil || !bytes.Equal(got.Image.Data, buf.Bytes()) || got.Image.Display != "BADGE" || got.Image.Decode() == nil {
		t.Errorf("unexpected calendar %+v", got)
	}
	if len(got.Events) != len(c.Events) {
		t.Fatalf("expected %d events, got %d", len(c.Events), len(got.Events))
	}
	for i, want := range c.Events {
		e := got.Events[i]
		if !e.Start.Time.Equal(want.Start.Time) || e.Start.TZID != want.Start.TZID || e.Start.Floating != want.Start.Floating || e.Start.Date != want.Start.Date {
			t.Errorf("%d: expected start %+v, got %+v", i, want.Start, e.Start)
		}
		if !e.End.Time.Equal(want.End.Time) || !e.Stamp.Equal(want.Stamp) || e.UID != want.UID || e.Kind != want.Kind {
			t.Errorf("%d: expected %+v, got %+v", i, want, e)
		}
	}
	a := got.Events[0]
	if a.Description != "Line one\nLine two" || a.URL != "https://example.com/a;b" || a.Geo == nil || !a.Geo.Equal(ll) || a.Organizer != "ada@example.com" || len(a.ExDates) != 1 || !a.Created.Equal(c.Events[0].Created) || !a.Modified.Equal(c.Events[0].Modified) {
		t.Errorf("unexpected event %+v", a)
	}
}
