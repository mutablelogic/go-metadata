package ical_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
)

// birthdays returns the calendars in the test files, which are anonymized
// birthdays of Google's contacts, as omodie writes them, and their data, by
// name
func birthdays(t *testing.T) (map[string]*ical.Calendar, map[string][]byte) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "testfiles", "ical_birthday*.ics"))
	if err != nil || len(files) == 0 {
		t.Fatalf("expected test files, got %v", err)
	}
	calendars, data := make(map[string]*ical.Calendar), make(map[string][]byte)
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var c ical.Calendar
		if err := c.UnmarshalText(b); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		calendars[filepath.Base(path)], data[filepath.Base(path)] = &c, b
	}
	return calendars, data
}

func Test_testdata_000(t *testing.T) {
	// A birthday is an all-day event, which repeats every year, and isn't
	// busy, with an image, and is the same when it's encoded again
	calendars, data := birthdays(t)
	for name, c := range calendars {
		if len(c.Events) != 1 {
			t.Fatalf("%s: expected one event, got %d", name, len(c.Events))
		}
		e := c.Events[0]
		if !e.Start.Date || e.End.Time.Sub(e.Start.Time) != 24*time.Hour || e.RRule != "FREQ=YEARLY" || !e.Transparent {
			t.Errorf("%s: expected an all-day event every year, got %+v", name, e)
		}
		if len(e.Categories) != 1 || e.Categories[0] != "Birthdays" || e.Image == nil || e.Image.Decode() == nil || e.Image.Display != "BADGE" {
			t.Errorf("%s: expected a birthday with an image, got %+v", name, e)
		}
		// The end, which is computed, isn't written, as it wasn't
		e.End = ical.Time{}
		if got, err := c.MarshalText(); err != nil {
			t.Fatal(err)
		} else if string(got) != string(data[name]) {
			t.Errorf("%s: expected\n%s\ngot\n%s", name, data[name], got)
		}
	}
}

func Test_testdata_001(t *testing.T) {
	// A birthday whose year isn't known is in 1604, which a calendar
	// doesn't show an age for, and one whose year is known is in its year
	calendars, _ := birthdays(t)
	noYear, withYear := calendars["ical_birthday1.ics"].Events[0], calendars["ical_birthday2.ics"].Events[0]
	if noYear.OmitYear != 1604 || noYear.Start.Time.Year() != 1604 || noYear.Summary != "Ada Johnson's birthday" {
		t.Errorf("unexpected birthday without a year %+v", noYear)
	}
	if withYear.OmitYear != 0 || withYear.Start.Time.Year() == 1604 {
		t.Errorf("unexpected birthday with a year %+v", withYear)
	}
}

func Test_testdata_002(t *testing.T) {
	// An anonymized export of Google Calendar: its name and zone, without
	// the zones' definitions or the alarms, a recurring event in a zone, and
	// the occurrence it changes, an all-day event, an event with an
	// organizer and attendees, in UTC, and an event whose position is a
	// place on a map
	b, err := os.ReadFile(filepath.Join("..", "..", "testfiles", "ical_google.ics"))
	if err != nil {
		t.Fatal(err)
	}
	var c ical.Calendar
	if err := c.UnmarshalText(b); err != nil {
		t.Fatal(err)
	}
	if c.Name != "Example Calendar" || c.TZ != "Europe/Berlin" || len(c.Events) != 5 {
		t.Fatalf("unexpected calendar %q %q with %d events", c.Name, c.TZ, len(c.Events))
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}

	// The recurring event, in Berlin, and the occurrence it changes, which
	// has its UID, and the start of the occurrence, on a day of its rule
	recurring, changed := c.Events[0], c.Events[1]
	if recurring.Start.TZID != "Europe/Berlin" || !recurring.Start.Time.Equal(time.Date(2021, 12, 20, 18, 30, 0, 0, berlin)) || !recurring.End.Time.Equal(time.Date(2021, 12, 20, 21, 0, 0, 0, berlin)) {
		t.Errorf("unexpected recurring event %+v %+v", recurring.Start, recurring.End)
	}
	if recurring.RRule != "FREQ=WEEKLY;WKST=MO;UNTIL=20220209T215959Z;BYDAY=MO,WE" || recurring.Summary != "Event 1" || recurring.Location != "1 Example Street, Exampleton" {
		t.Errorf("unexpected recurring event %+v", recurring)
	}
	if changed.UID != recurring.UID || !changed.RecurrenceID.Time.Equal(time.Date(2022, 1, 31, 18, 30, 0, 0, berlin)) || changed.RecurrenceID.Time.In(berlin).Weekday() != time.Monday || !changed.Start.Time.Equal(time.Date(2022, 2, 4, 18, 30, 0, 0, berlin)) {
		t.Errorf("unexpected changed occurrence %+v", changed)
	}

	// An all-day event, which isn't busy, whose alarm isn't its own
	allDay := c.Events[2]
	if allDay.Start != ical.Date(2022, 6, 16) || allDay.End != ical.Date(2022, 6, 17) || !allDay.Transparent || allDay.Description != "A description of the event,\nover two lines." {
		t.Errorf("unexpected all-day event %+v", allDay)
	}

	// An event in UTC, with an organizer, who's an attendee too, and
	// attendees, by their addresses
	meeting := c.Events[3]
	if meeting.Start.TZID != "" || meeting.Start.Floating || !meeting.Start.Time.Equal(time.Date(2018, 6, 25, 18, 0, 0, 0, time.UTC)) || meeting.Organizer != "person1@example.com" || len(meeting.Attendees) != 2 || meeting.Attendees[1] != "person2@example.com" {
		t.Errorf("unexpected meeting %+v", meeting)
	}

	// An event whose position is a place on a map, as there's no GEO
	place := c.Events[4]
	if place.Geo == nil || place.Geo.Lat != 51.5007 || place.Geo.Long != -0.1246 {
		t.Errorf("expected the position of the place, got %v", place.Geo)
	}
}
