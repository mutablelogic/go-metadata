package vcard_test

import (
	"strings"
	"testing"
	"time"

	// Packages
	ical "github.com/mutablelogic/go-metadata/pkg/ical"
	vcard "github.com/mutablelogic/go-metadata/pkg/vcard"
)

func Test_birthday_000(t *testing.T) {
	// A birthday without a year is in a leap year, which a calendar doesn't
	// show an age for, and the 29th of February is on the last day of
	// February each year, and without a revision, the stamp is the epoch,
	// so it's the same each time, as omodie writes it
	card := &vcard.Card{UID: "people/c3", FN: "Leap; Year, Person", Birthday: &vcard.Date{Month: 2, Day: 29}}
	c := card.BirthdayCalendar()
	if c == nil {
		t.Fatal("expected a calendar")
	}
	c.ProdID = "-//omodie//Google Contacts//EN"
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

func Test_birthday_001(t *testing.T) {
	// A birthday with a year is in its year, without X-APPLE-OMIT-YEAR, with
	// the contact's revision as its stamp, and their photo as its image, and
	// the name is the first email address without a formatted name
	rev := time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)
	card := &vcard.Card{
		UID:      "people/c4",
		Emails:   []vcard.Value{{Value: "ada@example.com"}},
		Birthday: &vcard.Date{Year: 1990, Month: 7, Day: 4},
		Photo:    &vcard.Photo{MediaType: "image/png", Data: []byte("not really a png")},
		Rev:      rev,
	}
	c := card.BirthdayCalendar()
	if c == nil || len(c.Events) != 1 {
		t.Fatalf("expected a calendar, got %v", c)
	}
	e := c.Events[0]
	if e.Start != ical.Date(1990, time.July, 4) || e.OmitYear != 0 || e.RRule != "FREQ=YEARLY" || !e.Stamp.Equal(rev) {
		t.Errorf("unexpected event %+v", e)
	}
	if e.Summary != "ada@example.com's birthday" || e.Image == nil || e.Image.Display != "BADGE" || e.Image.MediaType != "image/png" {
		t.Errorf("unexpected event %+v", e)
	}
	data, err := c.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DTSTART;VALUE=DATE:19900704\r\n", "DTSTAMP:20261018T093000Z\r\n", "IMAGE;VALUE=BINARY;ENCODING=BASE64;FMTTYPE=image/png;DISPLAY=BADGE:"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "X-APPLE-OMIT-YEAR") {
		t.Errorf("expected no X-APPLE-OMIT-YEAR in\n%s", data)
	}
}

func Test_birthday_002(t *testing.T) {
	// A contact without a birthday, or one without a month and a day, has
	// no calendar
	for _, card := range []*vcard.Card{
		{UID: "people/c5"},
		{UID: "people/c5", Birthday: &vcard.Date{Year: 1990}},
		{UID: "people/c5", Birthday: &vcard.Date{Year: 1990, Month: 3}},
	} {
		if c := card.BirthdayCalendar(); c != nil {
			t.Errorf("%+v: expected no calendar, got %+v", card.Birthday, c)
		}
	}
}

func Test_birthday_003(t *testing.T) {
	// The birthday of a contact of the test files, which has one, is a
	// calendar, which is decoded as it was encoded
	cards := testdata(t)
	card := cards["vcard_google2.vcf"]
	c := card.BirthdayCalendar()
	if c == nil {
		t.Fatal("expected a calendar")
	}
	data, err := c.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ical.Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	} else if len(got.Events) != 1 {
		t.Fatalf("expected one event, got %d", len(got.Events))
	}
	e := got.Events[0]
	if e.Start != ical.Date(vcard.OmitYear, time.November, 28) || e.OmitYear != vcard.OmitYear || e.UID != card.UID+"/birthday" || e.Image == nil || e.Image.Decode() == nil {
		t.Errorf("unexpected birthday %+v", e)
	}
}
