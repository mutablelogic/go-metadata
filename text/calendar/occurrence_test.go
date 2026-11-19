package calendar_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	// Packages
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
	calendar "github.com/mutablelogic/go-metadata/text/calendar"
)

// starts returns the recurrence IDs, and starts, of the occurrences
func starts(occurrences []calendar.Occurrence) []string {
	var result []string
	for _, o := range occurrences {
		start := o.Start.Time.UTC().Format(time.RFC3339)
		if o.Start.Date {
			start = o.Start.Time.Format(time.DateOnly)
		}
		result = append(result, fmt.Sprintf("%s %s %s", o.UID, o.RecurrenceID, start))
	}
	return result
}

func Test_occurrences_000(t *testing.T) {
	// An event is of its own, with its end, a task starts when it's due,
	// without one, and one which is cancelled, a birthday without a year,
	// or an event without a start, has none
	got := calendar.Occurrences([]calendar.Event{
		{UID: "a", Kind: "event", Title: "Meeting", Start: "2026-11-20T08:00:00Z", End: "2026-11-20T09:00:00Z", TZ: "Europe/Berlin"},
		{UID: "t", Kind: "task", Title: "Pay", Due: "2026-11-21"},
		{UID: "c", Kind: "event", Start: "2026-11-22T08:00:00Z", Status: "CANCELLED"},
		{UID: "b", Kind: "event", Start: "1604-12-10", AllDay: true, NoYear: true, RRule: "FREQ=YEARLY"},
		{UID: "n", Kind: "event"},
	}, 0)
	want := []calendar.Occurrence{
		{UID: "a", Kind: "event", Title: "Meeting", Start: ical.UTC(time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)), End: ical.UTC(time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)), TZ: "Europe/Berlin"},
		{UID: "t", Kind: "task", Title: "Pay", Start: ical.Date(2026, 11, 21)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func Test_occurrences_001(t *testing.T) {
	// A weekly meeting, of four, at 9:00 in Berlin, which is 8:00 in UTC
	// until summer time ends, and 9:00 once it has, without the start it
	// excludes, and the one another event changes, which is of its own
	got := calendar.Occurrences([]calendar.Event{
		{UID: "w", Kind: "event", Title: "Standup", Start: "2026-10-12T07:00:00Z", End: "2026-10-12T07:15:00Z", TZ: "Europe/Berlin", RRule: "FREQ=WEEKLY;COUNT=4", ExDate: []string{"2026-10-19T07:00:00Z"}},
		{UID: "w", Kind: "event", Title: "Standup, later", Start: "2026-10-26T10:00:00Z", End: "2026-10-26T10:15:00Z", TZ: "Europe/Berlin", RecurrenceID: "2026-10-26T08:00:00Z"},
	}, 0)
	want := []string{
		"w 2026-10-12T07:00:00Z 2026-10-12T07:00:00Z",
		"w 2026-10-26T08:00:00Z 2026-10-26T10:00:00Z",
		"w 2026-11-02T08:00:00Z 2026-11-02T08:00:00Z",
	}
	if got := starts(got); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if end := got[2].End.Time; !end.Equal(time.Date(2026, 11, 2, 8, 15, 0, 0, time.UTC)) {
		t.Errorf("an occurrence is as long as the event: ends %v", end)
	}
}

func Test_occurrences_002(t *testing.T) {
	// An all-day event, yearly, until a date, is of dates, and one which
	// repeats forever is its first, and the most which are returned is the
	// max
	events := []calendar.Event{
		{UID: "y", Kind: "event", Start: "2026-12-24", End: "2026-12-25", AllDay: true, RRule: "FREQ=YEARLY;UNTIL=20281231"},
		{UID: "f", Kind: "event", Start: "2026-11-02T08:00:00Z", RRule: "FREQ=WEEKLY"},
	}
	want := []string{
		"f 2026-11-02T08:00:00Z 2026-11-02T08:00:00Z",
		"y 2026-12-24 2026-12-24",
		"y 2027-12-24 2027-12-24",
		"y 2028-12-24 2028-12-24",
	}
	if got := starts(calendar.Occurrences(events, 0)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := starts(calendar.Occurrences(events, 2)); !reflect.DeepEqual(got, want[:2]) {
		t.Errorf("of a max of 2, the earliest: got %q, want %q", got, want[:2])
	}

	// A rule which can't be read is its first
	if got := calendar.Occurrences([]calendar.Event{{UID: "x", Kind: "event", Start: "2026-11-02T08:00:00Z", RRule: "FREQ=SOMETIMES;COUNT=3"}}, 0); len(got) != 1 {
		t.Errorf("of a rule which can't be read: got %d", len(got))
	}
}

func Test_occurrences_003(t *testing.T) {
	// The first time an event which repeats forever occurs is excluded, of
	// its EXDATE, or changed, of another event, which is of its own, or
	// cancelled
	start := "2026-11-02T08:00:00Z"
	for name, test := range map[string]struct {
		events []calendar.Event
		want   []string
	}{
		"excluded": {[]calendar.Event{{UID: "f", Kind: "event", Start: start, RRule: "FREQ=WEEKLY", ExDate: []string{start}}}, nil},
		"changed": {[]calendar.Event{
			{UID: "f", Kind: "event", Start: start, RRule: "FREQ=WEEKLY"},
			{UID: "f", Kind: "event", Start: "2026-11-02T10:00:00Z", RecurrenceID: start},
		}, []string{"f " + start + " 2026-11-02T10:00:00Z"}},
		"cancelled": {[]calendar.Event{
			{UID: "f", Kind: "event", Start: start, RRule: "FREQ=WEEKLY"},
			{UID: "f", Kind: "event", Start: start, RecurrenceID: start, Status: "CANCELLED"},
		}, nil},
	} {
		if got := starts(calendar.Occurrences(test.events, 0)); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %q, want %q", name, got, test.want)
		}
	}

	// A task which has a start, and is due, is of when it's due
	got := calendar.Occurrences([]calendar.Event{{UID: "t", Kind: "task", Start: "2026-11-01", Due: "2026-11-21"}}, 0)
	if len(got) != 1 || !got[0].Start.Time.Equal(time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("a task is of when it's due: got %+v", got)
	}
}
